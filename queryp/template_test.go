package queryp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTemplate(t *testing.T) {
	tests := map[string]struct {
		t            *TemplateBuilder
		expectedQ    string
		expectedArgs []any
	}{
		"does not replace non-named placeholders": {
			Must(NewTemplate("SELECT * FROM test WHERE id = :id")).Build(),
			"SELECT * FROM test WHERE id = :id",
			nil,
		},
		"replaces named placeholder": {
			Must(NewTemplate("SELECT * FROM test WHERE id = :id")).
				Param("id", 1),
			"SELECT * FROM test WHERE id = ?",
			[]any{1},
		},
		"replaces multiple named placeholders": {
			Must(NewTemplate("SELECT * FROM test WHERE id = :id AND name = :name")).
				Params(map[string]any{
					"id":   1,
					"name": "Alice",
				}),
			"SELECT * FROM test WHERE id = ? AND name = ?",
			[]any{1, "Alice"},
		},
		"replaces multiple named placeholders in any order": {
			Must(NewTemplate("SELECT * FROM test WHERE id = :id AND name = :name")).
				Params(map[string]any{
					"name": "Alice",
					"id":   1,
				}),
			"SELECT * FROM test WHERE id = ? AND name = ?",
			[]any{1, "Alice"},
		},
		"supports other placeholder styles": {
			Must(NewTemplate("SELECT * FROM test WHERE id = :id")).
				Placeholderer(PostgresPlaceholderer).
				Param("id", 1),
			"SELECT * FROM test WHERE id = $1",
			[]any{1},
		},
		"supports Param function": {
			Must(NewTemplate(`SELECT * FROM test WHERE id = {{.Param "id"}}`)).
				Param("id", 1),
			"SELECT * FROM test WHERE id = ?",
			[]any{1},
		},
		"supports Includes function": {
			Must(NewTemplate(`{{if .Includes "pet"}}COALESCE(pet.id, 0) AS pet_id, COALESCE(pet.name, "") AS pet_name{{end}}`)).
				Include("pet"),
			`COALESCE(pet.id, 0) AS pet_id, COALESCE(pet.name, "") AS pet_name`,
			nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			q, args, err := test.t.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if q != test.expectedQ {
				t.Errorf("expected query %q, got %q", test.expectedQ, q)
			}
			if !cmp.Equal(args, test.expectedArgs) {
				t.Errorf("unexpected args: %s", cmp.Diff(test.expectedArgs, args))
			}
		})
	}
}
