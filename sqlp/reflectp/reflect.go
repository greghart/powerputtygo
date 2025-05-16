package reflectp

import (
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
)

// Field represents a Field in a struct.
// Adapted from json package reflection.
// Key difference is json recursively encodes/decodes, we just need a flattened column centric view.
type Field struct {
	Column string
	Fields *StructFields // Fields of the struct, if this is a struct.

	Tag   bool
	Index int
	Type  reflect.Type

	Promote bool // This fields' sub-fields will be promoted to the parent struct.
}

// StructFields represents the fields of a struct.
type StructFields struct {
	ByColumnName map[string]*Field
}

type recContext struct {
	root         string
	byColumnName map[string]*Field
	visited      map[reflect.Type]struct{}
	i            []int
}

// TypeFields returns the reflected fields of a struct, pre-processed for easier row scanning.
// TypeFields must be ran on a struct type.
// Note, this process has to defer some amount of work, since for potentially recursive structs,
// we can't know how deep to go until there is data to check against.
func TypeFields(t reflect.Type) (*StructFields, error) {
	byColumnName := make(map[string]*Field, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
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

		// index := make([]int, len(ctx.i)+1)
		// copy(index, ctx.i)
		// index[len(ctx.i)] = i

		field := Field{
			Column:  column,
			Tag:     tagged,
			Index:   i,
			Type:    ft,
			Promote: (opts.Contains("promote") || (sf.Anonymous && !tagged)) && ft.Kind() == reflect.Struct,
		}

		if _, ok := byColumnName[column]; ok {
			// Collision == error for now
			return nil, fmt.Errorf("duplicate column name %s", column)
		}
		byColumnName[column] = &field
	}

	return &StructFields{ByColumnName: byColumnName}, nil
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

var timeType = reflect.TypeOf(time.Time{})
