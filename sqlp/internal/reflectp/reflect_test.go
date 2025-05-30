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
		// Use `column` to signal that sub-struct columns are direct parts of table.
		Timestamps        Timestamps `sql:"timestamps,promote"`
		privateTimestamps            // Does still work since non-exported embedded struct has exported fields.
	}

	fields, _ := FieldsFactory(reflect.TypeOf(Person{}))
	expected := Fields{
		ByColumnName: map[string]*Field{
			"ID": {
				Column:     "ID",
				Index:      []int{0},
				DirectType: reflect.TypeOf(0),
			},
			"name": {
				Column:     "name",
				Tag:        true,
				Index:      []int{1},
				DirectType: reflect.TypeOf(""),
			},
			"child1": {
				Column:     "child1",
				Tag:        true,
				Index:      []int{2},
				DirectType: reflect.TypeOf(Person{}),
			},
			"child2": {
				Column:     "child2",
				Tag:        true,
				Index:      []int{3},
				DirectType: reflect.TypeOf(Person{}),
			},
			"Timestamps": {
				Column:     "Timestamps",
				Index:      []int{5},
				DirectType: reflect.TypeOf(Timestamps{}),
			},
			"created_at": {
				Column:     "created_at",
				Tag:        true,
				Index:      []int{6, 0},
				DirectType: reflect.TypeOf(time.Time{}),
			},
			"updated_at": {
				Column:     "updated_at",
				Tag:        true,
				Index:      []int{6, 1},
				DirectType: reflect.TypeOf(time.Time{}),
			},
		},
	}
	comparer := cmp.Comparer(func(x, y Field) bool {
		return (x.Column == y.Column &&
			cmp.Equal(x.Index, y.Index) &&
			x.DirectType.Kind() == y.DirectType.Kind())

	})
	if !cmp.Equal(fields.ByColumnName, expected.ByColumnName, comparer) {
		t.Errorf("TypeFields returned unexpected fields:\n%s", cmp.Diff(expected.ByColumnName, fields.ByColumnName, comparer))
	}
}
