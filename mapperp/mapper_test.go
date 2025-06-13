package mapperp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMapper_One(t *testing.T) {
	tests := map[string]struct {
		rows     []row
		expected person
	}{
		"single person -> person": {
			rows: []row{
				{person: person{ID: 1, Name: "Alice"}, pet: pet{}},
			},
			expected: person{ID: 1, Name: "Alice"},
		},
		"two people -> first person": {
			rows: []row{
				{person: person{ID: 1, Name: "Alice"}},
				{person: person{ID: 2, Name: "Bob"}},
			},
			expected: person{ID: 1, Name: "Alice"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rowMapper := One(func(row *row) *person {
				return &row.person
			})

			var result person
			for i, r := range test.rows {
				rowMapper(&result, &r, i)
			}

			if !cmp.Equal(result, test.expected) {
				t.Errorf("mapped person unexpected:\n%v", cmp.Diff(test.expected, result))
			}
		})
	}
}

func TestMapper_Slice(t *testing.T) {
	tests := map[string]struct {
		rows     []row
		expected []person
	}{
		"single person -> single person": {
			rows: []row{
				{person: person{ID: 1, Name: "Alice"}, pet: pet{}},
			},
			expected: []person{
				{ID: 1, Name: "Alice"},
			},
		},
		"two people -> two people": {
			rows: []row{
				{person: person{ID: 1, Name: "Alice"}},
				{person: person{ID: 2, Name: "Bob"}},
			},
			expected: []person{
				{ID: 1, Name: "Alice"},
				{ID: 2, Name: "Bob"},
			},
		},
		"two people, three rows -> two people": {
			rows: []row{
				{person: person{ID: 1, Name: "Alice"}},
				{person: person{ID: 2, Name: "Bob"}, pet: pet{ID: 1, Name: "Kitty"}},
				// we're not mapping pets here, so this row is ignored
				{person: person{ID: 2, Name: "Bob"}, pet: pet{ID: 2, Name: "Doggy"}},
			},
			expected: []person{
				{ID: 1, Name: "Alice"},
				{ID: 2, Name: "Bob"},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rowMapper := Slice(
				func(e *person) int64 {
					return e.ID
				},
				func(row *row) *person {
					return &row.person
				},
			)

			var result []person
			for i, r := range test.rows {
				rowMapper(&result, &r, i)
			}

			if !cmp.Equal(result, test.expected) {
				t.Errorf("mapped people unexpected:\n%v", cmp.Diff(test.expected, result))
			}
		})
	}
}

func TestMapper_All(t *testing.T) {
	tests := map[string]struct {
		rows     []row
		expected []person
	}{
		"single person with pets": {
			rows: []row{
				{person: person{ID: 1, Name: "Alice"}, pet: pet{ID: 1, Name: "Kitty"}},
				{person: person{ID: 1, Name: "Alice"}, pet: pet{ID: 2, Name: "Doggy"}},
			},
			expected: []person{
				{ID: 1, Name: "Alice", Pets: []pet{
					{ID: 1, Name: "Kitty"},
					{ID: 2, Name: "Doggy"},
				}},
			},
		},
		"two people with pets": {
			rows: []row{
				{person: person{ID: 1, Name: "Alice"}, pet: pet{ID: 1, Name: "Kitty"}},
				{person: person{ID: 1, Name: "Alice"}, pet: pet{ID: 2, Name: "Doggy"}},
				{person: person{ID: 2, Name: "Bob"}, pet: pet{ID: 3, Name: "Weasely"}},
				{person: person{ID: 2, Name: "Bob"}, pet: pet{ID: 4, Name: "Fishy"}},
			},
			expected: []person{
				person{ID: 1, Name: "Alice", Pets: []pet{
					{ID: 1, Name: "Kitty"},
					{ID: 2, Name: "Doggy"},
				}},
				person{ID: 2, Name: "Bob", Pets: []pet{
					{ID: 3, Name: "Weasely"},
					{ID: 4, Name: "Fishy"},
				}},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rowMapper := Slice(
				func(e *person) int64 { return e.ID },
				func(row *row) *person { return &row.person },
				Last(
					// Pets
					InnerSlice(
						func(e *person) *[]pet {
							return &e.Pets
						},
						func(e *pet) int64 {
							return e.ID
						},
						func(row *row) *pet {
							return &row.pet
						},
					),
				),
			)

			var result []person
			for i, r := range test.rows {
				rowMapper(&result, &r, i)
			}

			if !cmp.Equal(result, test.expected) {
				t.Errorf("mapped person unexpected:\n%v", cmp.Diff(test.expected, result))
			}
		})
	}
}

////////////////////////////////////////////////////////////////////////////////

// a result from a database join of person and pet
type row struct {
	person
	pet
	parent person
}

// our domain models
type person struct {
	ID     int64
	Name   string
	Pets   []pet
	Parent *person
}

type pet struct {
	ID   int64
	Name string
}
