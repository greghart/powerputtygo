package queryp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNamed(t *testing.T) {
	tests := map[string]struct {
		in            *NamedQuery
		expectedQuery string
		expectedArgs  []any
	}{
		"does not replace non-named placeholders": {
			Named("SELECT * FROM test WHERE id = :id"),
			"SELECT * FROM test WHERE id = :id",
			nil,
		},
		"replaces named placeholder": {
			Named("SELECT * FROM test WHERE id = :id").Param("id", 1),
			"SELECT * FROM test WHERE id = ?",
			[]any{1},
		},
		"replaces multiple named placeholders": {
			Named("SELECT * FROM test WHERE id = :id AND name = :name").Params(map[string]any{
				"id":   1,
				"name": "Alice",
			}),
			"SELECT * FROM test WHERE id = ? AND name = ?",
			[]any{1, "Alice"},
		},
		"replaces multiple named placeholders in any order": {
			Named("SELECT * FROM test WHERE id = :id AND name = :name").Params(map[string]any{
				"name": "Alice",
				"id":   1,
			}),
			"SELECT * FROM test WHERE id = ? AND name = ?",
			[]any{1, "Alice"},
		},
		"supports other placeholder styles": {
			Named("SELECT * FROM test WHERE id = :id").WithPlaceholderer(PostgresPlaceholderer).Param("id", 1),
			"SELECT * FROM test WHERE id = $1",
			[]any{1},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.in.String() != test.expectedQuery {
				t.Errorf("Named query %s did not match expected %s", test.in.String(), test.expectedQuery)
			}
			if !cmp.Equal(test.in.Args(), test.expectedArgs) {
				t.Errorf("Named args did not match expected: %v", cmp.Diff(test.expectedArgs, test.in.Args()))
			}
		})
	}
}
