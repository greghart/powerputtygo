package sqlp

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewMapper(t *testing.T) {
	personMapper := personMapper(t)
	p := person{
		ID:        1,
		FirstName: "Joe",
		Child: &person{
			ID:        2,
			FirstName: "Bob",
		},
	}
	tests := []struct {
		col   string
		ptr   any
		value any
	}{
		{"id", &p.ID, int64(1)},
		{"first_name", &p.FirstName, "Joe"},
		{"child_id", &p.Child.ID, int64(2)},
		{"child_first_name", &p.Child.FirstName, "Bob"},
		{"pet_id", nil, int64(0)}, // Pet is not set, so no addr can be expected
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("col %v", test.col), func(t *testing.T) {
			addr, ok := personMapper.Addr(&p, test.col)
			if !ok {
				t.Fatalf("wanted to find %v", test.col)
			}
			if test.ptr != nil && addr != test.ptr {
				t.Errorf("addr: got %v, wanted %v", addr, test.ptr)
			}
			if !cmp.Equal(test.value, reflect.ValueOf(addr).Elem().Interface()) {
				t.Errorf("value: got %v, wanted %v", reflect.ValueOf(addr).Elem().Interface(), test.value)
			}
		})
	}
}

// //////////////////////////////////////////////////////////////////////////////

func personMapper(t *testing.T) Mapper[person] {
	t.Helper()
	petMapper := Mapper[pet]{
		"id":   func(p *pet) any { return &p.ID },
		"name": func(p *pet) any { return &p.Name },
		"type": func(p *pet) any { return &p.Type },
	}
	personMapper := Mapper[person]{
		"id":         func(p *person) any { return &p.ID },
		"first_name": func(p *person) any { return &p.FirstName },
		"last_name":  func(p *person) any { return &p.LastName },
	}
	personMapper = MergeMappers(personMapper, petMapper, "pet", func(p *person) *pet {
		if p.Pet == nil {
			p.Pet = &pet{}
		}
		return p.Pet
	})
	// Support children
	personMapper = MergeMappers(personMapper, personMapper, "child", func(p *person) *person {
		if p.Child == nil {
			p.Child = &person{}
		}
		return p.Child
	})
	// Support grandchildren
	personMapper = MergeMappers(personMapper, personMapper, "child", func(p *person) *person {
		if p.Child == nil {
			p.Child = &person{}
		}
		return p.Child
	})
	return personMapper
}
