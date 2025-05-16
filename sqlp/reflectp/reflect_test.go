package reflectp

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestTypeFields(t *testing.T) {
	type Timestamps struct {
		CreatedAt time.Time `sqlp:"created_at"`
		UpdatedAt time.Time `sqlp:"updated_at"`
	}
	type privateTimestamps struct {
		CreatedAt time.Time `sqlp:"private_created_at"`
		UpdatedAt time.Time `sqlp:"private_updated_at"`
	}
	type Person struct {
		// Column default from field name 'ID'
		ID int
		// Column specified with struct tag
		Name string `sqlp:"name"`
		// Embedded structs should come in `_` separated (configurable)
		// Eg. child1_name, child2_name
		Child1     *Person `sqlp:"child1"`
		Child2     *Person `sqlp:"child2"`
		Ignore     *Person `sqlp:"-"` // will never be scanned
		unexported *Person // Not reflectable
		// Anonymous structs are assumed to be promoted unless they are tagged
		// Use `promote` to force an embed to promote fields to top level
		Timestamps
		privateTimestamps `sqlp:"private,promote"` // Does still work since non-exported embedded struct has exported fields
	}

	fields, _ := TypeFields(reflect.TypeOf(Person{}))
	expected := StructFields{
		ByColumnName: map[string]*Field{
			"ID": {
				Column: "ID",
				Index:  0,
				Type:   reflect.TypeOf(0),
			},
			"name": {
				Column: "name",
				Tag:    true,
				Index:  1,
				Type:   reflect.TypeOf(""),
			},
			"child1": {
				Column: "child1",
				Tag:    true,
				Index:  2,
				Type:   reflect.TypeOf(Person{}),
			},
			"child2": {
				Column: "child2",
				Tag:    true,
				Index:  3,
				Type:   reflect.TypeOf(Person{}),
			},
			"Timestamps": {
				Column:  "Timestamps",
				Tag:     true,
				Index:   6,
				Type:    reflect.TypeOf(Timestamps{}),
				Promote: true,
			},
			"private": {
				Column:  "private",
				Tag:     true,
				Index:   7,
				Type:    reflect.TypeOf(privateTimestamps{}),
				Promote: true,
			},
		},
	}
	comparer := cmp.Comparer(func(x, y Field) bool {
		return (x.Column == y.Column &&
			cmp.Equal(x.Index, y.Index) &&
			x.Type.Kind() == y.Type.Kind() &&
			x.Promote == y.Promote)
	})
	if !cmp.Equal(fields.ByColumnName, expected.ByColumnName, comparer) {
		t.Errorf("TypeFields returned unexpected fields:\n%s", cmp.Diff(fields.ByColumnName, expected.ByColumnName, comparer))
	}
}
