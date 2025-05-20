package sqlp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDAO_Validate(t *testing.T) {
	db, _, cleanup := testDB(t)
	defer cleanup()

	type validater interface {
		Validate() error
	}
	tests := map[string]struct {
		dao      func() validater
		expected string
	}{
		"no fields -> nil": {
			dao: func() validater {
				return NewDAO[struct{}](db, "test_table")
			},
		},
		"good fields -> nil": {
			dao: func() validater {
				type goodFields struct {
					ID   int    `sqlp:"id"`
					Name string `sqlp:"name"`
				}
				return NewDAO[goodFields](db, "test_table")
			},
		},
		"person -> nil": {
			dao: func() validater {
				return NewDAO[person](db, "test_table")
			},
		},
		"bad fields -> err": {
			dao: func() validater {
				type badFields struct {
					ID   int    `sqlp:"id"`
					Name string `sqlp:"id"` // duplicate tag
				}
				return NewDAO[badFields](db, "test_table")
			},
			expected: "duplicate column name id",
		},
		"bad but deep fields -> err": {
			dao: func() validater {
				type badFields struct {
					ID   int    `sqlp:"id"`
					Name string `sqlp:"id"` // duplicate tag
				}
				type parent struct {
					Bad badFields `sqlp:"bad"`
				}
				return NewDAO[parent](db, "test_table")
			},
			expected: "failed to process sub struct Bad: duplicate column name id",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			dao := test.dao()
			err := dao.Validate()
			if test.expected == "" && err != nil {
				t.Errorf("expected no error, got %v", err)
			} else if test.expected != "" && (err == nil || err.Error() != test.expected) {
				t.Errorf("expected error %v, got %v", test.expected, err)
			}
		})
	}
}

func TestDAO_Get(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()

	dao := NewDAO[person](db, "people")

	id1, id2, id3 := grandchildrenSetup(ctx, db)

	t.Run("multi table query joins", func(t *testing.T) {
		p, err := dao.Get(ctx, selectGrandchildrenAndPets("p.id = ?"), id1)
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		expected := person{
			ID: id1, FirstName: "John", LastName: "Doe",
			Child: &person{
				ID: id2, FirstName: "Lil Johnnie", LastName: "Doe",
				Child: &person{
					ID: id3, FirstName: "Lil Lil Johnnie", LastName: "Doe",
				},
				Pet: &pet{ID: 1, Name: "Eevee", Type: "Dog"},
			},
		}
		if !cmp.Equal(p, expected, personComparer) {
			t.Errorf("selected people unexpected:\n%v", cmp.Diff(expected, p, personComparer))
		}
	})

	t.Run("simple one table query", func(t *testing.T) {
		p, err := dao.Get(ctx, "SELECT id, first_name, last_name FROM people")
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		expected := person{ID: id1, FirstName: "John", LastName: "Doe"}
		if !cmp.Equal(p, expected, personComparer) {
			t.Errorf("gotten person unexpected:\n%v", cmp.Diff(expected, p, personComparer))
		}
	})
}

func TestDAO_Select(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()

	dao := NewDAO[person](db, "people")

	id1, id2, id3 := grandchildrenSetup(ctx, db)
	// Another one to show off multiple rows
	res, _ := db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "Albert", "Einstein") // nolint:errcheck
	albertId, _ := res.LastInsertId()

	t.Run("multi table query with joins", func(t *testing.T) {
		people, err := dao.Select(ctx, selectGrandchildrenAndPets())
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}
		expected := []person{
			{
				ID: id1, FirstName: "John", LastName: "Doe",
				Child: &person{
					ID: id2, FirstName: "Lil Johnnie", LastName: "Doe",
					Child: &person{
						ID: id3, FirstName: "Lil Lil Johnnie", LastName: "Doe",
					},
					Pet: &pet{ID: 1, Name: "Eevee", Type: "Dog"},
				},
			},
			{
				ID: albertId, FirstName: "Albert", LastName: "Einstein",
			},
		}
		if !cmp.Equal(people, expected, personComparer) {
			t.Errorf("selected people unexpected:\n%v", cmp.Diff(expected, people, personComparer))
		}
	})

	t.Run("simple one table query", func(t *testing.T) {
		people, err := dao.Select(ctx, "SELECT id, first_name, last_name FROM people")
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}
		expected := []person{
			{ID: id1, FirstName: "John", LastName: "Doe"},
			{ID: id2, FirstName: "Lil Johnnie", LastName: "Doe"},
			{ID: id3, FirstName: "Lil Lil Johnnie", LastName: "Doe"},
			{ID: albertId, FirstName: "Albert", LastName: "Einstein"},
		}
		if !cmp.Equal(people, expected, personComparer) {
			t.Errorf("selected people unexpected:\n%v", cmp.Diff(expected, people, personComparer))
		}
	})
}
