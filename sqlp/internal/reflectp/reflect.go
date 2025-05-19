package reflectp

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Field represents a Field in a struct.
// Adapted from json package reflection.
// Key difference is json recursively encodes/decodes, we're handling flat tabular data.
type Field struct {
	Column string

	Tag        bool
	Index      int
	DirectType reflect.Type // Direct type of field, equal to Type unless pointer
	Type       reflect.Type

	IsColumn bool // This fields' sub-fields will be promoted to the parent struct.

	// Cached sub fields
	fields *Fields // Fields of the struct, if this is a struct.
}

// Get the sub fields of this field when it's a struct itself.
func (f *Field) Fields() *Fields {
	if f.fields != nil {
		return f.fields
	}
	if f.DirectType.Kind() == reflect.Struct {
		fields, _ := FieldsFactory(f.DirectType) // nolint:errcheck we pre-touched all structs
		f.fields = fields
		return fields
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////

// Fields represents the fields of a struct.
type Fields struct {
	ByColumnName map[string]*Field
	Type         reflect.Type
}

// Internally, all types are stored in a cache to avoid repeated work.
func FieldsFactory(t reflect.Type) (*Fields, error) {
	if f, ok := fieldsCache.Load(t); ok {
		return f.(*Fields), nil
	}
	f, err := newFields(t)
	if err != nil {
		return nil, err
	}
	fCache, _ := fieldsCache.LoadOrStore(t, f)
	return fCache.(*Fields), nil
}

// newFields returns the reflected fields of a struct, pre-processed for easier row scanning.
// newFields must be ran on a struct type.
// Note, this process has to defer some amount of work, since for potentially recursive structs,
// we can't know how deep to go until there is data to check against.
func newFields(t reflect.Type, _visited ...map[reflect.Type]bool) (*Fields, error) {
	visited := map[reflect.Type]bool{}
	if len(visited) > 0 {
		visited = _visited[0]
	}
	visited[t] = true
	byColumnName := make(map[string]*Field, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		// Ignore cases
		if sf.Anonymous {
			t := sf.Type
			if t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			if !sf.IsExported() && t.Kind() != reflect.Struct {
				// Ignore embedded fields of unexported non-struct types.
				continue
			}
			// Do not ignore embedded fields of unexported struct types
			// since they may have exported fields.
		} else if !sf.IsExported() {
			// Ignore unexported non-embedded fields.
			continue
		}

		// Process
		tag := sf.Tag.Get("sqlp")
		if tag == "-" {
			continue
		}
		column, opts := parseTag(tag)
		if !isValidTag(column) {
			column = ""
		}

		ft := sf.Type
		if ft.Name() == "" && ft.Kind() == reflect.Pointer {
			// Follow pointer.
			ft = ft.Elem()
		}

		tagged := column != ""
		if column == "" {
			column = sf.Name
		}

		field := Field{
			Column:     column,
			Tag:        tagged,
			Index:      i,
			DirectType: ft,
			Type:       sf.Type,
			IsColumn:   (opts.Contains("promote") || (sf.Anonymous && !tagged)) && ft.Kind() == reflect.Struct,
		}
		if _, ok := visited[ft]; ft.Kind() == reflect.Struct && !ok {
			// Recursively touch structs to error early.
			_, err := newFields(ft, visited)
			if err != nil {
				return nil, fmt.Errorf("failed to process sub struct %s: %w", sf.Name, err)
			}
		}

		if _, ok := byColumnName[column]; ok {
			// Collision == error for now
			return nil, fmt.Errorf("duplicate column name %s", column)
		}
		byColumnName[column] = &field
	}

	return &Fields{Type: t, ByColumnName: byColumnName}, nil
}

func (f *Fields) Rows(rows *sql.Rows) (*FieldsRows, error) {
	return NewFieldsRows(f, rows)
}

// targeter is a function that will return a pointer to a field in the given value.
type targeter func(strct reflect.Value) (fieldPtr reflect.Value)

type fieldTargeter struct {
	field    *Field
	targeter targeter
}

// TODO: Can we add a field traversal method, that traverses all fields that are touched
// by the given columns?
// Then it's easy-ish to do both targeters and nilOutZeros in one go.

// traverse traverses the fields of the struct for given columns.
// Calls cb with all fields matching a column, as well as any intermediate embedded structs.
// The boolean indicates whether the field is a column directly or not.
func (f *Fields) traverse(cols []string, cb func(f *Field, b bool)) error {
	// for i := range cols {
	// 	field, ok := f.ByColumnName[cols[i]]
	// 	if ok {
	// 		cb(field, true)
	// 	}
	// 	// Could be a sub field
	// 	root, rest, _ := strings.Cut(cols[i], "_")
	// 	field, ok = f.ByColumnName[root]
	// 	if !ok || field.Fields() == nil {
	// 		return fmt.Errorf("unknown column %s", cols[i])
	// 	}
	// 	// subT, err := field.Fields().fieldTargeter(rest)
	// 	// if err != nil {
	// 	// 	return err
	// 	// }
	// 	// return &fieldTargeter{
	// 	// 	field: subT.field,
	// 	// 	targeter: func(v reflect.Value) reflect.Value {
	// 	// 		v = direct(v)
	// 	// 		// Touch the field to ensure it is initialized
	// 	// 		if v.Field(field.Index).IsZero() {
	// 	// 			v.Field(field.Index).Set(reflect.New(field.DirectType))
	// 	// 		}
	// 	// 		return subT.targeter(v.Field(field.Index))
	// 	// 	},
	// 	// }, nil
	// }
	return nil
}

// fieldTargeter returns field for given column, and a targeter to it
func (f *Fields) fieldTargeter(col string) (*fieldTargeter, error) {
	field, ok := f.ByColumnName[col]
	if ok {
		return &fieldTargeter{
			field: field,
			targeter: func(v reflect.Value) reflect.Value {
				return direct(v).Field(field.Index).Addr()
			},
		}, nil
	}

	// Could be a sub field
	root, rest, _ := strings.Cut(col, "_")
	field, ok = f.ByColumnName[root]
	if !ok || field.Fields() == nil {
		return nil, fmt.Errorf("unknown column %s", col)
	}
	subT, err := field.Fields().fieldTargeter(rest)
	if err != nil {
		return nil, err
	}
	return &fieldTargeter{
		field: subT.field,
		targeter: func(v reflect.Value) reflect.Value {
			v = direct(v)
			// Touch the field to ensure it is initialized
			if v.Field(field.Index).IsZero() {
				v.Field(field.Index).Set(reflect.New(field.DirectType))
			}
			return subT.targeter(v.Field(field.Index))
		},
	}, nil
}

////////////////////////////////////////////////////////////////////////////////

// FieldsRows handles scanning rows into given struct field.
type FieldsRows struct {
	*sql.Rows
	fields    *Fields
	targets   []any
	targeters []fieldTargeter
	// Embedded pointer struct fields that are touched by these columns
	// We should nil out zero values of these fields
	zeroNilEmbeds []*Field
}

func NewFieldsRows(f *Fields, rows *sql.Rows) (*FieldsRows, error) {
	sr := &FieldsRows{
		Rows:   rows,
		fields: f,
	}
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	sr.targets = make([]any, len(cols))             // where fields will scan into, re-used across rows
	sr.targeters = make([]fieldTargeter, len(cols)) // targeters for each column, pre-calculated
	for i, c := range cols {
		t, err := f.fieldTargeter(c)
		if err != nil {
			return nil, fmt.Errorf("failed to get targeter for column %s: %w", c, err)
		}
		sr.targeters[i] = *t
	}

	sr.zeroNilEmbeds = []*Field{}
	byRoot := make(map[string]bool)
	for i := range cols {
		root, _, _ := strings.Cut(cols[i], "_")
		if _, ok := byRoot[root]; ok {
			continue
		}
		byRoot[root] = true

		field, ok := sr.fields.ByColumnName[root]

		if ok && field.Fields() != nil && field.Type.Kind() == reflect.Pointer {
			sr.zeroNilEmbeds = append(sr.zeroNilEmbeds, field)
		}
	}

	return sr, nil
}

func (sr *FieldsRows) Scan() (reflect.Value, error) {
	val := reflect.New(sr.fields.Type)

	for i := range sr.targeters {
		sr.targets[i] = sr.targeters[i].targeter(val).Interface()
	}

	if err := sr.Rows.Scan(sr.targets...); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to scan row: %w", err)
	}

	// Post process, remove any embedded pointer structs that should be nil-d out
	// TODO: Needs to be recursive!
	for _, field := range sr.zeroNilEmbeds {
		elem := direct(val).Field(field.Index).Elem()
		if elem.IsValid() && elem.IsZero() {
			direct(val).Field(field.Index).Set(reflect.Zero(field.Type))
		}
	}

	return val, nil
}

////////////////////////////////////////////////////////////////////////////////

func direct(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Pointer {
		return v.Elem()
	}
	return v
}

// tagOptions is the string following a comma in a struct field's "sqlp"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

func parseTag(tag string) (string, tagOptions) {
	tag, opt, _ := strings.Cut(tag, ",")
	return tag, tagOptions(opt)
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var name string
		name, s, _ = strings.Cut(s, ",")
		if name == optionName {
			return true
		}
	}
	return false
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
			// Backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name.
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}

var timeType = reflect.TypeOf(time.Time{})

var fieldsCache sync.Map // map[reflect.Type]Fields

type isZeroer interface {
	IsZero() bool
}

var isZeroerType = reflect.TypeFor[isZeroer]()
