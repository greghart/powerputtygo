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
		CreatedAt time.Time `sqlp:"created_at"`
		UpdatedAt time.Time `sqlp:"updated_at"`
	}
	type Person struct {
		// Column default from field name 'ID'
		ID int
		// Column specified with struct tag
		Name string `sqlp:"name"`
		// sub struct fields should come in `_` separated (configurable)
		// Eg. child1_name, child2_name
		Child1 *Person `sqlp:"child1"`
		Child2 *Person `sqlp:"child2"`
		Ignore *Person `sqlp:"-"` // will never be scanned
		// Embedded structs are assumed to be columns unless they are tagged otherwise.
		// Use `column` to signal that sub-struct columns are part of table.
		Timestamps        Timestamps `sql:"timestamps,column"`
		privateTimestamps            // Does still work since non-exported embedded struct has exported fields.
	}

	fields, _ := FieldsFactory(reflect.TypeOf(Person{}))
	expected := Fields{
		ByColumnName: map[string]*Field{
			"ID": {
				Column:     "ID",
				Index:      0,
				DirectType: reflect.TypeOf(0),
			},
			"name": {
				Column:     "name",
				Tag:        true,
				Index:      1,
				DirectType: reflect.TypeOf(""),
			},
			"child1": {
				Column:     "child1",
				Tag:        true,
				Index:      2,
				DirectType: reflect.TypeOf(Person{}),
			},
			"child2": {
				Column:     "child2",
				Tag:        true,
				Index:      3,
				DirectType: reflect.TypeOf(Person{}),
			},
			"Timestamps": {
				Column:     "Timestamps",
				Index:      6,
				DirectType: reflect.TypeOf(Timestamps{}),
			},
			"privateTimestamps": {
				Column:     "privateTimestamps",
				Tag:        true,
				Index:      7,
				DirectType: reflect.TypeOf(privateTimestamps{}),
				IsColumn:   true,
			},
		},
	}
	comparer := cmp.Comparer(func(x, y Field) bool {
		return (x.Column == y.Column &&
			cmp.Equal(x.Index, y.Index) &&
			x.DirectType.Kind() == y.DirectType.Kind() &&
			x.IsColumn == y.IsColumn)
	})
	if !cmp.Equal(fields.ByColumnName, expected.ByColumnName, comparer) {
		t.Errorf("TypeFields returned unexpected fields:\n%s", cmp.Diff(expected.ByColumnName, fields.ByColumnName, comparer))
	}
}
