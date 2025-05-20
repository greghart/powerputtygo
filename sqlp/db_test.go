package sqlp

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/greghart/powerputtygo/errcmp"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func TestSqlp_Exec(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()

	res, err := db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "John", "Doe")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	lastInsertId, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get last insert id: %v", err)
	}
	if lastInsertId == 0 {
		t.Fatalf("last insert id should be set properly")
	}
}

func TestSqlp_Query(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()
	db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "John", "Doe") // nolint:errcheck

	rows, err := db.Query(ctx, "SELECT id, first_name, last_name FROM people WHERE first_name = ?", "John")
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p person
		if err := rows.Scan(&p.ID, &p.FirstName, &p.LastName); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}

func TestSqlp_QueryRow(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()
	db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "John", "Doe") // nolint:errcheck

	row := db.QueryRow(ctx, "SELECT id, first_name, last_name FROM people WHERE first_name = ?", "John")
	var p person
	if err := row.Scan(&p.ID, &p.FirstName, &p.LastName); err != nil {
		t.Fatalf("failed to query: %v", row.Err())
	}
}

func TestSqlp_Select(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()

	id1, id2, id3 := grandchildrenSetup(ctx, db)
	// Another one to show off multiple rows
	res, _ := db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "Albert", "Einstein") // nolint:errcheck
	albertId, _ := res.LastInsertId()

	t.Run("multi table query with joins", func(t *testing.T) {
		people := []person{}
		err := db.Select(ctx, &people, selectGrandchildrenAndPets())
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
		people := []person{}
		err := db.Select(ctx, &people, "SELECT id, first_name, last_name FROM people")
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

	t.Run("to slice of people pointers", func(t *testing.T) {
		people := []*person{}
		err := db.Select(ctx, &people, "SELECT id, first_name, last_name FROM people")
		errcmp.MustMatch(t, err, "given ptr, expected struct")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestSqlp_RunInTx(t *testing.T) {
	db, ctx, cleanup := testPG(t)
	defer cleanup()

	t.Run("transacts operations as expected", func(t *testing.T) {
		id := 1
		nonTxCtx := ctx
		err := db.RunInTx(ctx, func(ctx context.Context) error {
			_, err := db.Exec(ctx, "INSERT INTO people (id, first_name, last_name) VALUES ($1, $2, $3)", id, "John", "Doe")
			errcmp.MustMatch(t, err, "")
			// person not found committed yet
			p := person{}
			err = db.Get(nonTxCtx, &p, "SELECT * FROM people WHERE id = $1", id)
			errcmp.MustMatch(t, err, "")
			if p.ID != 0 {
				t.Fatalf("got %v, expected no person", p)
			}
			// is found within transaction?
			err = db.Get(ctx, &p, "SELECT * FROM people WHERE id = $1", id)
			errcmp.MustMatch(t, err, "")
			if p.ID == 0 {
				t.Fatalf("found no person, expected person")
			}
			return nil
		})
		errcmp.MustMatch(t, err, "")

		// person committed now
		p := person{}
		err = db.Get(ctx, &p, "SELECT * FROM people WHERE id = $1", id)
		errcmp.MustMatch(t, err, "")
		if p.ID == 0 {
			t.Fatalf("found no person, expected person")
		}
	})

	t.Run("auto rolls back operations on panic", func(t *testing.T) {
		id := 2
		err := db.RunInTx(ctx, func(ctx context.Context) (err error) {
			_, err = db.Exec(ctx, "INSERT INTO people (id, first_name, last_name) VALUES ($1, $2, $3)", id, "John", "Doe")
			errcmp.MustMatch(t, err, "")
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("paniced")
				}
			}()
			panic("uhoh") // caught and rolled back
		})
		errcmp.MustMatch(t, err, "paniced")

		p := person{}
		err = db.Get(ctx, &p, "SELECT * FROM people WHERE id = $1", id)
		errcmp.MustMatch(t, err, "")
		if p.ID != 0 {
			t.Fatalf("got %v, expected no person", p)
		}
	})

	t.Run("auto rolls back operations on error", func(t *testing.T) {
		id := 3
		err := db.RunInTx(ctx, func(ctx context.Context) error {
			_, err := db.Exec(ctx, "INSERT INTO people (id, first_name, last_name) VALUES ($1, $2, $3)", id, "John", "Doe")
			errcmp.MustMatch(t, err, "")
			return fmt.Errorf("test error")
		})
		errcmp.MustMatch(t, err, "test error")

		p := person{}
		err = db.Get(ctx, &p, "SELECT * FROM people WHERE id = $1", id)
		errcmp.MustMatch(t, err, "")
		if p.ID != 0 {
			t.Fatalf("got %v, expected no person", p)
		}
	})
}

func TestSqlp_Get(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()

	id1, id2, id3 := grandchildrenSetup(ctx, db)
	t.Run("multi table query joins", func(t *testing.T) {
		p := person{}
		err := db.Get(ctx, &p, selectGrandchildrenAndPets("p.id = ?"), id1)
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
		p := person{}
		err := db.Get(ctx, &p, "SELECT id, first_name, last_name FROM people")
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		expected := person{ID: id1, FirstName: "John", LastName: "Doe"}
		if !cmp.Equal(p, expected, personComparer) {
			t.Errorf("gotten person unexpected:\n%v", cmp.Diff(expected, p, personComparer))
		}
	})

	t.Run("to person pointer", func(t *testing.T) {
		p := &person{}
		err := db.Get(ctx, &p, "SELECT id, first_name, last_name FROM people")
		errcmp.MustMatch(t, err, "given ptr, expected struct")
	})
}

////////////////////////////////////////////////////////////////////////////////

func grandchildrenSetup(ctx context.Context, db *DB) (int64, int64, int64) {
	res, _ := db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "John", "Doe")
	id, _ := res.LastInsertId()
	res2, _ := db.Exec(ctx, "INSERT INTO people (first_name, last_name, parent_id) VALUES (?, ?, ?)", "Lil Johnnie", "Doe", id)
	id2, _ := res2.LastInsertId()
	res3, _ := db.Exec(ctx, "INSERT INTO people (first_name, last_name, parent_id) VALUES (?, ?, ?)", "Lil Lil Johnnie", "Doe", id2) // nolint:errcheck
	id3, _ := res3.LastInsertId()
	db.Exec(ctx, "INSERT INTO pets (name, type, parent_id) VALUES (?, ?, ?)", "Eevee", "Dog", id2) // nolint:errcheck
	return id, id2, id3
}

// Not using query helpers since those shouldn't be tested as part of these tests
func selectGrandchildrenAndPets(_wheres ...string) string {
	wheres := []string{"1=1"}
	if len(_wheres) > 0 {
		wheres = _wheres
	}
	return fmt.Sprintf(
		`
		SELECT 
			p.id, p.first_name, p.last_name,
			COALESCE(child.id, 0) AS child_id,
			COALESCE(child.first_name, "") AS child_first_name,
			COALESCE(child.last_name, "") AS child_last_name,
			COALESCE(grandchild.id, 0) AS child_child_id,
			COALESCE(grandchild.first_name, "") AS child_child_first_name,
			COALESCE(grandchild.last_name, "") AS child_child_last_name,
			COALESCE(pet.id, 0) AS pet_id,
			COALESCE(pet.name, "") AS pet_name,
			COALESCE(pet.type, "") AS pet_type,
			COALESCE(child_pet.id, 0) AS child_pet_id,
			COALESCE(child_pet.name, "") AS child_pet_name,
			COALESCE(child_pet.type, "") AS child_pet_type,
			COALESCE(grandchild_pet.id, 0) AS child_child_pet_id,
			COALESCE(grandchild_pet.name, "") AS child_child_pet_name,
			COALESCE(grandchild_pet.type, "") AS child_child_pet_type
		FROM people p
		LEFT JOIN people child ON p.id = child.parent_id
		LEFT JOIN people grandchild ON child.id = grandchild.parent_id
		LEFT JOIN pets pet ON p.id = pet.parent_id
		LEFT JOIN pets child_pet ON child.id = child_pet.parent_id
		LEFT JOIN pets grandchild_pet ON grandchild.id = grandchild_pet.parent_id
		WHERE p.parent_id IS NULL AND (%s)
		ORDER BY p.id ASC
		`,
		strings.Join(wheres, " AND "))
}

////////////////////////////////////////////////////////////////////////////////

// Verify some tests with Postgres as well
// More to keep these as nice examples
// Note sqlite has difference auto increment syntax, so pg tests should manually set ids
func testPG(t *testing.T) (*DB, context.Context, func()) {
	t.Helper()

	db, err := Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=sqlp_test sslmode=disable")
	if err != nil {
		t.Fatalf("testPG failed to open: %v", err)
	}
	return testDBSetup(t, db)
}

// testDB returns a test database and a cleanup function.
func testDB(t *testing.T) (*DB, context.Context, func()) {
	t.Helper()

	db, err := Open("sqlite3", "./test.db")
	if err != nil {
		t.Fatalf("testDB failed to open: %v", err)
	}
	return testDBSetup(t, db)
}

func testDBSetup(t *testing.T, db *DB) (*DB, context.Context, func()) {
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
			first_name TEXT,
			last_name TEXT,
			parent_id INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
		;
		CREATE TABLE IF NOT EXISTS pets (
			id INTEGER PRIMARY KEY,
			name TEXT,
			type TEXT,
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
	ID          int64   `sqlp:"id"`
	NumChildren int     `sqlp:"num_children"`
	FirstName   string  `sqlp:"first_name"`
	LastName    string  `sqlp:"last_name"`
	Child       *person `sqlp:"child"`
	Pet         *pet    `sqlp:"pet"`
	timestamps
}

type pet struct {
	ID   int    `sqlp:"id"`
	Name string `sqlp:"name"`
	Type string `sqlp:"type"`
}

type timestamps struct {
	CreatedAt time.Time `sqlp:"created_at"`
	UpdatedAt time.Time `sqlp:"updated_at"`
}

func isWithinDuration(t1 time.Time, t2 time.Time, d time.Duration) bool {
	return time.Duration(math.Abs(float64(t1.Sub(t2)))) <= d
}

func _ptrComparer[T any](x, y *T, cmp func(a, b T) bool) bool {
	if x == nil && y == nil {
		return true
	}
	if x != nil && y != nil {
		return cmp(*x, *y)
	}
	return false
}

func _petComparer(x, y pet) bool {
	return (x.ID == y.ID &&
		x.Name == y.Name &&
		x.Type == y.Type)
}

func _personComparer(x, y person) bool {
	return (x.ID == y.ID &&
		x.FirstName == y.FirstName &&
		x.LastName == y.LastName &&
		isWithinDuration(x.CreatedAt, y.CreatedAt, 5*time.Second) &&
		isWithinDuration(x.UpdatedAt, y.UpdatedAt, 5*time.Second) &&
		_ptrComparer(x.Child, y.Child, _personComparer) &&
		_ptrComparer(x.Pet, y.Pet, _petComparer))
}

var personComparer = cmp.Comparer(_personComparer)
