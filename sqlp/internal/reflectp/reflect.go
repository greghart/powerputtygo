package reflectp

import (
	"cmp"
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
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

	IsColumn bool // This fields' sub-fields will be considered columns of the parent table.

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
			IsColumn:   (opts.Contains("column") || (sf.Anonymous && !tagged)) && ft.Kind() == reflect.Struct,
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

////////////////////////////////////////////////////////////////////////////////

// traverse traverses the fields of the struct for given columns.
// Also triggers for intermediate fields (eg. triggers for Child field if requesting child_id).
// Calls cb with all fields matching a column, their full struct path, and whether it's a column
// (true) or an intermediate field (false).
func (f *Fields) traverse(cols []string, cb func(f *Field, path []int, b bool), _path ...[]int) error {
	path := []int{}
	if len(_path) > 0 {
		path = _path[0]
	}

	for i := range cols {
		field, ok := f.ByColumnName[cols[i]]
		if ok {
			cb(field, append(path[:], field.Index), true)
			continue
		}
		// Could be a sub field
		root, rest, _ := strings.Cut(cols[i], "_")
		field, ok = f.ByColumnName[root]
		if !ok || field.Fields() == nil {
			return fmt.Errorf("unknown column %s (on path %v)", cols[i], path)
		}
		path2 := append(path[:], field.Index)
		// Traverse nested first
		if err := field.Fields().traverse([]string{rest}, cb, path2); err != nil {
			return err
		}
		cb(field, path2, false)
	}
	return nil
}

// targeter is a function that will return a pointer to a field in the given value.
type targeter func(strct reflect.Value) (fieldPtr reflect.Value)

////////////////////////////////////////////////////////////////////////////////

// FieldsRows handles scanning rows into given struct field.
type FieldsRows struct {
	*sql.Rows
	fields  *Fields
	targets []any
	// Target the fields in our final struct
	targeters []targeter
	// Paths to sub struct fields that should be nil checked.
	// Nil check meaning to see if we ended up not scanning any data, we can nil out the 0 values
	// that were setup for scanning.
	zeroNilFields [][]int
}

func NewFieldsRows(f *Fields, rows *sql.Rows) (*FieldsRows, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	sr := &FieldsRows{
		Rows:      rows,
		fields:    f,
		targets:   make([]any, len(cols)),
		targeters: make([]targeter, len(cols)),
	}
	// Pre-calculate targeters and zero nil-checks
	subStructsByField := map[string][]int{}
	i := 0
	err = f.traverse(cols, func(field *Field, path []int, isColumn bool) {
		if !isColumn {
			subStructsByField[strings.Join(strings.Fields(fmt.Sprint(path)), ",")] = path
			return
		}
		if len(path) == 1 {
			sr.targeters[i] = func(v reflect.Value) reflect.Value {
				return reflect.Indirect(v).Field(path[0]).Addr()
			}
		} else {
			sr.targeters[i] = func(v reflect.Value) reflect.Value {
				for _, i := range path {
					v = reflect.Indirect(v).Field(i)
					// if this is a pointer and it's nil, allocate a new value and set it
					if v.Kind() == reflect.Ptr && v.IsNil() {
						alloc := reflect.New(deref(v.Type()))
						v.Set(alloc)
					}
					if v.Kind() == reflect.Map && v.IsNil() {
						v.Set(reflect.MakeMap(v.Type()))
					}
				}
				return v.Addr()
			}
		}
		i++
	})
	// Sort sub-structs by deepest path first
	// This ensures descendants are nil'd out first so ancestor can correctly nil out as well.
	for _, path := range subStructsByField {
		sr.zeroNilFields = append(sr.zeroNilFields, path)
	}
	slices.SortFunc(sr.zeroNilFields, func(a, b []int) int {
		return cmp.Compare(len(b), len(a))
	})

	return sr, err
}

func (sr *FieldsRows) Scan() (reflect.Value, error) {
	val := reflect.New(sr.fields.Type)

	for i := range sr.targeters {
		sr.targets[i] = sr.targeters[i](val).Interface()
	}

	if err := sr.Rows.Scan(sr.targets...); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to scan row: %w", err)
	}

	// Post process, remove any pointer structs that should be nil-d out
	for _, path := range sr.zeroNilFields {
		v := val
		for _, i := range path {
			v = reflect.Indirect(v).Field(i)
		}
		elem := v.Elem() // trust setup, will be pointers
		if elem.IsValid() && elem.IsZero() {
			v.Set(reflect.Zero(v.Type()))
		}
	}

	return val, nil
}

////////////////////////////////////////////////////////////////////////////////

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

var fieldsCache sync.Map // map[reflect.Type]Fields

func deref(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}
