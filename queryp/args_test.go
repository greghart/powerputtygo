package queryp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestArgs(t *testing.T) {
	args := NewArgs()
	name := args.Add("Alice")
	age := args.Add(30)
	if name != "?" {
		t.Errorf("name given placeholder %s, wanted '?'", name)
	}
	if age != "?" {
		t.Errorf("age given placeholder %s, wanted '?'", name)
	}
	expected := []any{"Alice", 30}
	if !cmp.Equal(args.Args(), expected) {
		t.Errorf("given args did not match expected\n%v", cmp.Diff(expected, args.Args()))
	}
}

func TestArgs_pg(t *testing.T) {
	args := NewArgs().WithPlaceholderer(PostgresPlaceholderer)
	name := args.Add("Alice")
	age := args.Add(30)
	if name != "$1" {
		t.Errorf("name given placeholder %s, wanted '$1'", name)
	}
	if age != "$2" {
		t.Errorf("age given placeholder %s, wanted '$2'", name)
	}
}
