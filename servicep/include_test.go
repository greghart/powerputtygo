package servicep

import "testing"

func TestIncludable(t *testing.T) {
	type expectation struct {
		q string
		r bool
	}
	tests := map[string]struct {
		includes     []string
		allows       []string
		expectations []expectation
	}{
		"single include": {
			includes: []string{"a"},
			expectations: []expectation{
				{"a", true},
				{"b", false},
			},
		},
		"nested include": {
			includes: []string{"a.b.c", "d"},
			expectations: []expectation{
				{"a", true},
				{"a.b", true},
				{"a.b.c", true},
				{"b.c", false},
				{"c", false},
				{"d", true},
				{"e", false},
				{"a.d", false},
			},
		},
		"only allowed": {
			includes: []string{"a.b.c", "d"},
			allows:   []string{"a.b"}, // a.b allows the a.b part of a.b.c, but not the full a.b.c
			expectations: []expectation{
				{"a", true},
				{"a.b", true},
				{"a.b.c", false},
				{"b.c", false},
				{"c", false},
				{"d", false},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			i := NewIncludeRequest()
			if len(test.allows) > 0 {
				i.SetSchema(NewIncludeSchema().Allow(test.allows...))
			}
			i.Include(test.includes...)

			for _, exp := range test.expectations {
				val := i.IsIncluded(exp.q)
				if val != exp.r {
					t.Errorf("Include(%q) got %v wanted %v", exp.q, val, exp.r)
				}
			}
		})
	}
}
