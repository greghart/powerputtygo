package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/greghart/powerputtygo/errcmp"
	"github.com/greghart/powerputtygo/mapperp"
	"github.com/greghart/powerputtygo/queryp"
	"github.com/greghart/powerputtygo/sqlp"
	_ "github.com/mattn/go-sqlite3"
)

// TestIntegration_DB tests powerputty db packages working in harmony.
func TestIntegration_DB(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()

	parent1 := peopleSetup(t, ctx, db)
	parent2 := peopleSetup(t, ctx, db)

	queryTemplate := queryp.Must(queryp.NewTemplate(
		`
		SELECT 
			p.id, p.name, p.created_at, p.updated_at
			{{- if .Include "child" "grandchild"}},
				COALESCE(child.id, 0) AS child_id,
				COALESCE(child.name, "") AS child_name
			{{- end}}
			{{- if .Include "grandchild"}},
				COALESCE(grandchild.id, 0) AS child_child_id,
				COALESCE(grandchild.name, "") AS child_child_name
			{{- end}}
			{{- if .Include "pet"}},
				COALESCE(pet.id, 0) AS pet_id,
				COALESCE(pet.name, "") AS pet_name,
				pet.type AS pet_type
				{{- if .Include "child"}},
					COALESCE(child_pet.id, 0) AS child_pet_id,
					COALESCE(child_pet.name, "") AS child_pet_name,
					child_pet.type AS child_pet_type
				{{- end}}
				{{- if .Include "grandchild"}},
					COALESCE(grandchild_pet.id, 0) AS grandchild_pet_id,
					COALESCE(grandchild_pet.name, "") AS grandchild_pet_name,
					grandchild_pet.type AS grandchild_pet_type
				{{- end}}
			{{- end}}
		FROM people p
		{{if .Include "child" "grandchild" -}}
			LEFT JOIN people child ON p.id = child.parent_id
		{{- end}}
		{{if .Include "grandchild" -}}
			LEFT JOIN people grandchild ON child.id = grandchild.parent_id
		{{- end}}
		{{if .Include "pet" -}}
			LEFT JOIN pets pet ON p.id = pet.parent_id
			{{if .Include "child" -}}
				LEFT JOIN pets child_pet ON child.id = child_pet.parent_id
			{{- end}}
			{{if .Include "grandchild" -}}
				LEFT JOIN pets grandchild_pet ON grandchild.id = grandchild_pet.parent_id
			{{- end}}
		{{- end}}
		WHERE p.parent_id IS NULL
		{{- if .Param "id"}}
			AND p.id = :id
		{{- end}}
		ORDER BY p.id ASC
		`,
	))
	type row struct {
		Pet           pet `sqlp:"pet"`
		ChildPet      pet `sqlp:"child_pet"`
		GrandchildPet pet `sqlp:"grandchild_pet"`
		person
	}

	// Reusable mappers since we're nesting the same entities multiple times.
	mapPets := func(getPet func(r *row) *pet) mapperp.Mapper[row, person] {
		return mapperp.InnerSlice(
			func(e *person) *[]pet { return &e.Pets },
			func(e *pet) int64 { return e.ID },
			getPet,
		)
	}
	getChild := func(p *person) *person {
		if p.Child == nil {
			p.Child = &person{}
		}
		return p.Child
	}
	mapper := mapperp.Slice[row, person](
		func(e *person) int64 { return e.ID },
		func(r *row) *person { return &r.person },
		mapperp.Take( // person
			mapPets(func(r *row) *pet { return &r.Pet }),
			mapperp.Inner( // person.child
				getChild,
				mapperp.One(func(r *row) *person { return r.Child }),
				mapPets(func(r *row) *pet { return &r.ChildPet }),
				mapperp.Inner( // person.child.child
					func(e *person) *person { return e.Child },
					mapperp.One(func(r *row) *person { return r.Child.Child }),
					mapPets(func(r *row) *pet { return &r.GrandchildPet }),
				),
			),
		),
	)

	q, args, err := queryTemplate.
		Include("pet", "child", "grandchild").
		Execute()
	if err != nil {
		t.Fatalf("failed to apply query template: %v", err)
	}
	t.Logf("Query:\n%s", q)

	rows, err := sqlp.Query[row](ctx, db, q, args...)
	errcmp.MustMatch(t, err, "")
	defer rows.Close()

	var out []person
	for i := 0; rows.Next(); i++ {
		row, err := rows.ScanOut()
		if err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		mapper(&out, &row, i)
	}

	js, _ := json.MarshalIndent(out, "", "  ")
	t.Logf("out:\n%s", string(js))

	opts := cmp.Options{
		cmpopts.EquateEmpty(),
		// For integration test, we can ignore update timestamps since those are expected to change
		cmp.FilterPath(
			func(p cmp.Path) bool {
				return strings.Contains(p.String(), "UpdatedAt") || strings.Contains(p.String(), "CreatedAt")
			},
			cmp.Ignore(),
		),
	}

	expected := []person{parent1, parent2}
	if !cmp.Equal(out, expected, opts) {
		t.Errorf("unexpected output:\n%s", cmp.Diff(out, expected, opts))
	}
}

////////////////////////////////////////////////////////////////////////////////

func testDB(t testing.TB) (*sqlp.DB, context.Context, func()) {
	t.Helper()

	db, err := sqlp.Open("sqlite3", "./test.db")
	if err != nil {
		t.Fatalf("testDB failed to open: %v", err)
	}
	return testDBSetup(t, db)
}

func testDBSetup(t testing.TB, db *sqlp.DB) (*sqlp.DB, context.Context, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("testDB failed to ping: %v", err)
	}

	// Setup a test table for the tests.
	_, err := db.Exec(ctx, "DROP TABLE IF EXISTS people; DROP TABLE IF EXISTS pets")
	if err != nil {
		t.Fatalf("testDB failed to drop table: %v", err)
	}
	_, err = db.Exec(
		ctx,
		`
		CREATE TABLE IF NOT EXISTS people (
			id INTEGER PRIMARY KEY,
			name TEXT,
			parent_id INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
		;
		CREATE TABLE IF NOT EXISTS pets (
			id INTEGER PRIMARY KEY,
			name TEXT,
			type TEXT NULLABLE,
			parent_id INTEGER
		)`)
	if err != nil {
		t.Fatalf("testDB failed to create table: %v", err)
	}
	return db, ctx, func() {
		db.Close()
		cancel()
	}
}

type person struct {
	ID         int64      `sqlp:"id"`
	Name       string     `sqlp:"name"`
	Child      *person    `sqlp:"child"` // For has one
	Pets       []pet      // For has many
	Timestamps timestamps `sqlp:",promote"`
}

type pet struct {
	ID   int64   `sqlp:"id"`
	Name string  `sqlp:"name"`
	Type *string `sqlp:"type"`
}

type timestamps struct {
	CreatedAt time.Time `sqlp:"created_at"`
	UpdatedAt time.Time `sqlp:"updated_at"`
}

func peopleSetup(t testing.TB, ctx context.Context, db *sqlp.DB) person {
	res, _ := db.Exec(ctx, "INSERT INTO people (name) VALUES (?)", "John Doe")
	id, _ := res.LastInsertId()
	res2, _ := db.Exec(ctx, "INSERT INTO people (name, parent_id) VALUES (?, ?)", "Lil Johnnie", id)
	id2, _ := res2.LastInsertId()
	res3, _ := db.Exec(ctx, "INSERT INTO people (name, parent_id) VALUES (?, ?)", "Lil Lil Johnnie", id2)
	id3, _ := res3.LastInsertId()
	pet1, _ := db.Exec(ctx, "INSERT INTO pets (name, type, parent_id) VALUES (?, ?, ?)", "Eevee", "Dog", id) // nolint:errcheck
	pet1ID, _ := pet1.LastInsertId()
	pet2, _ := db.Exec(ctx, "INSERT INTO pets (name, parent_id) VALUES (?, ?)", "Unknown Pet", id) // nolint:errcheck
	pet2ID, _ := pet2.LastInsertId()
	// Grandchildren pets
	pet3, _ := db.Exec(ctx, "INSERT INTO pets (name, type, parent_id) VALUES (?, ?, ?)", "Eevee", "Dog", id3) // nolint:errcheck
	pet3ID, _ := pet3.LastInsertId()
	pet4, _ := db.Exec(ctx, "INSERT INTO pets (name, parent_id) VALUES (?, ?)", "Unknown Pet", id3) // nolint:errcheck
	pet4ID, _ := pet4.LastInsertId()
	return person{
		ID: id, Name: "John Doe",
		Child: &person{
			ID: id2, Name: "Lil Johnnie",
			Child: &person{
				ID: id3, Name: "Lil Lil Johnnie",
				Pets: []pet{
					{ID: pet3ID, Name: "Eevee", Type: stringPtr("Dog")},
					{ID: pet4ID, Name: "Unknown Pet"},
				},
			},
		},
		Pets: []pet{
			{ID: pet1ID, Name: "Eevee", Type: stringPtr("Dog")},
			{ID: pet2ID, Name: "Unknown Pet"},
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
