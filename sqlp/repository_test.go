package sqlp

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/greghart/powerputtygo/errcmp"
)

func TestRepository_Validate(t *testing.T) {
	db, _ := testDB(t)

	type validater interface {
		Validate() error
	}
	tests := map[string]struct {
		repository func() validater
		expected   string
	}{
		"no fields -> nil": {
			repository: func() validater {
				return NewRepository[struct{}](db, "test_table")
			},
		},
		"good fields -> nil": {
			repository: func() validater {
				type goodFields struct {
					ID   int    `sqlp:"id"`
					Name string `sqlp:"name"`
				}
				return NewRepository[goodFields](db, "test_table")
			},
		},
		"person -> nil": {
			repository: func() validater {
				return NewRepository[person](db, "test_table")
			},
		},
		"bad fields -> err": {
			repository: func() validater {
				type badFields struct {
					ID   int    `sqlp:"id"`
					Name string `sqlp:"id"` // duplicate tag
				}
				return NewRepository[badFields](db, "test_table")
			},
			expected: "duplicate column name id",
		},
		"bad but deep fields -> err": {
			repository: func() validater {
				type badFields struct {
					ID   int    `sqlp:"id"`
					Name string `sqlp:"id"` // duplicate tag
				}
				type parent struct {
					Bad badFields `sqlp:"bad"`
				}
				return NewRepository[parent](db, "test_table")
			},
			expected: "failed to process sub struct Bad: duplicate column name id",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			repository := test.repository()
			err := repository.Validate()
			if test.expected == "" && err != nil {
				t.Errorf("expected no error, got %v", err)
			} else if test.expected != "" && (err == nil || err.Error() != test.expected) {
				t.Errorf("expected error %v, got %v", test.expected, err)
			}
		})
	}
}

func TestRepository_Insert(t *testing.T) {
	db, ctx := testDB(t)

	repository := NewRepository[person](db, "people")

	t.Run("inserts the column like fields as expected", func(t *testing.T) {
		p := person{
			ID:        -1,
			FirstName: "Joe",
			LastName:  "Schmoe",
		}
		res, err := repository.Insert(ctx, p)
		errcmp.MustMatch(t, err, "", "insert failed")
		id, err := res.LastInsertId()
		errcmp.MustMatch(t, err, "", "no insert id")

		fetched, err := repository.Find(ctx, id)
		errcmp.MustMatch(t, err, "", fmt.Sprintf("couldn't fetch id %d", id))
		p.ID = fetched.ID
		if !cmp.Equal(*fetched, p, personComparer) {
			t.Errorf("updated entity mis-match (-got +want):\n%s", cmp.Diff(*fetched, p, personComparer))
		}
	})
}

func TestRepository_Update(t *testing.T) {
	db, ctx := testDB(t)

	grandparent := grandchildrenSetup(ctx, db)

	t.Run("non-identifiable repo", func(t *testing.T) {
		repository := NewRepository[person](db, "people")

		p := grandparent
		p.FirstName = "UPDATED"
		_, err := repository.Update(ctx, &p)
		errcmp.MustMatch(t, err, "does not have an Identifier implementation")
	})

	t.Run("identifiable entity", func(t *testing.T) {
		repository := NewRepository[personID](db, "people").WithIdentifier(func(e *personID) any { return e.ID })

		p := personID{grandparent}
		p.FirstName = "UPDATED"
		_, err := repository.Update(ctx, &p)
		errcmp.MustMatch(t, err, "", "could not update")

		updated, err := repository.Get(ctx, "SELECT * FROM people WHERE id = ?", grandparent.ID)
		errcmp.MustMatch(t, err, "", "failed to get updated grandparent")

		if updated.FirstName != p.FirstName {
			t.Errorf("updated first name %s, wanted %s", updated.FirstName, p.FirstName)
		}
	})
}

func TestRepository_Get(t *testing.T) {
	db, ctx := testDB(t)

	repository := NewRepository[person](db, "people")

	grandparent := grandchildrenSetup(ctx, db)

	t.Run("multi table query joins", func(t *testing.T) {
		p, err := repository.Get(ctx, selectGrandchildrenAndPets("p.id = ?"), grandparent.ID)
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		expected := grandparent
		if !cmp.Equal(*p, expected, personComparer) {
			t.Errorf("selected people unexpected:\n%v", cmp.Diff(expected, *p, personComparer))
		}
	})

	t.Run("simple one table query", func(t *testing.T) {
		p, err := repository.Get(ctx, "SELECT id, first_name, last_name FROM people")
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		expected := person{ID: grandparent.ID, FirstName: "John", LastName: "Doe"}
		if !cmp.Equal(*p, expected, personComparer) {
			t.Errorf("gotten person unexpected:\n%v", cmp.Diff(expected, *p, personComparer))
		}
	})
}

func TestRepository_Select(t *testing.T) {
	db, ctx := testDB(t)

	repository := NewRepository[person](db, "people")

	grandparent := grandchildrenSetup(ctx, db)
	albert := albertSetup(ctx, db)

	t.Run("multi table query with joins", func(t *testing.T) {
		people, err := repository.Select(ctx, selectGrandchildrenAndPets())
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}
		expected := []person{
			grandparent,
			albert,
		}
		if !cmp.Equal(people, expected, personComparer) {
			t.Errorf("selected people unexpected:\n%v", cmp.Diff(expected, people, personComparer))
		}
	})

	t.Run("simple one table query", func(t *testing.T) {
		people, err := repository.Select(ctx, "SELECT id, first_name, last_name FROM people")
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}
		expected := []person{
			{ID: grandparent.ID, FirstName: "John", LastName: "Doe"},
			{ID: grandparent.Child.ID, FirstName: "Lil Johnnie", LastName: "Doe"},
			{ID: grandparent.Child.Child.ID, FirstName: "Lil Lil Johnnie", LastName: "Doe"},
			albert,
		}
		if !cmp.Equal(people, expected, personComparer) {
			t.Errorf("selected people unexpected:\n%v", cmp.Diff(expected, people, personComparer))
		}
	})
}
