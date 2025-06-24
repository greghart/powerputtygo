package servicep

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

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
					t.Errorf("Include(%q) got %v expecteded %v", exp.q, val, exp.r)
				}
			}
		})
	}
}

func TestIncludeRequest_All(t *testing.T) {
	req := NewIncludeRequest().Include("a.b.c", "d")
	var got []string
	for s := range req.All() {
		got = append(got, s)
	}
	expected := []string{"a.b.c", "d"}
	if !cmp.Equal(got, expected, ignoreSort) {
		t.Errorf("All() returned %v, expected %v", got, expected)
	}
}

func TestIncludeRequest_Filter(t *testing.T) {
	req := NewIncludeRequest().Include("a.b.c", "d", "foo.bar")
	var got []string
	f := func(s string) bool { return len(s) > 3 }
	for s := range req.Filter(f) {
		got = append(got, s)
	}
	expected := []string{"a.b.c", "foo.bar"}
	if !cmp.Equal(got, expected, ignoreSort) {
		t.Errorf("Filter() = %v, expected %v", got, expected)
	}
}

func TestIncludeRequest_Subcludes(t *testing.T) {
	req := NewIncludeRequest().Include("a.b.c", "a.b.d.e", "a.x", "b.c")
	var got []string
	for s := range req.Subcludes("a.b") {
		got = append(got, s)
	}
	expected := []string{"c", "d.e"}
	if !cmp.Equal(got, expected, ignoreSort) {
		t.Errorf("got %v, expected %v", got, expected)
	}
}

////////////////////////////////////////////////////////////////////////////////

var ignoreSort = cmpopts.SortSlices(func(a, b string) bool { return a < b })
