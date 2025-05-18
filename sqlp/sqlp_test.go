package sqlp

import (
	"context"
	"log"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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
	db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "John", "Doe")

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
		t.Logf("person: %v", p)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}

func TestSqlp_QueryRow(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()
	db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "John", "Doe")

	row := db.QueryRow(ctx, "SELECT id, first_name, last_name FROM people WHERE first_name = ?", "John")
	var p person
	if err := row.Scan(&p.ID, &p.FirstName, &p.LastName); err != nil {
		t.Fatalf("failed to query: %v", row.Err())
	}
}

func TestSqlp_Select(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()

	res, _ := db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "John", "Doe")
	id, _ := res.LastInsertId()
	db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "Albert", "Einstein")
	db.Exec(ctx, "INSERT INTO people (first_name, last_name, parent_id) VALUES (?, ?, ?)", "Lil Johnnie", "Doe", id)

	query := `
		SELECT 
			p.id, p.first_name, p.last_name,
			COALESCE(c.id, 0) AS child_id,
			COALESCE(c.first_name, "") AS child_first_name,
			COALESCE(c.last_name, "") AS child_last_name
		FROM people p
		LEFT JOIN people c ON p.id = c.parent_id
		WHERE p.parent_id IS NULL
		ORDER BY p.id ASC
	`
	t.Run("multi table query with embeds and joins", func(t *testing.T) {
		people := []person{}
		err := db.Select(ctx, &people, query)
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}
		expected := []person{
			{
				ID: 1, FirstName: "John", LastName: "Doe",
				Child: &person{ID: 3, FirstName: "Lil Johnnie", LastName: "Doe"},
			},
			{
				ID: 2, FirstName: "Albert", LastName: "Einstein",
			},
		}
		if !cmp.Equal(people, expected, personComparer) {
			t.Errorf("selected people unexpected:\n%v", cmp.Diff(people, expected, personComparer))
		}
	})

	t.Run("simple one table query", func(t *testing.T) {
		people := []person{}
		err := db.Select(ctx, &people, "SELECT id, first_name, last_name FROM people")
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}
		expected := []person{
			{ID: 1, FirstName: "John", LastName: "Doe"},
			{ID: 2, FirstName: "Albert", LastName: "Einstein"},
			{ID: 3, FirstName: "Lil Johnnie", LastName: "Doe"},
		}
		if !cmp.Equal(people, expected, personComparer) {
			t.Errorf("selected people unexpected:\n%v", cmp.Diff(people, expected, personComparer))
		}
	})

	t.Run("to slice of people pointers", func(t *testing.T) {
		people := []*person{}
		err := db.Select(ctx, &people, query)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

////////////////////////////////////////////////////////////////////////////////

// testDB returns a test database and a cleanup function.
func testDB(t *testing.T) (*DB, context.Context, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	db, err := Open("sqlite3", "./test.db")
	if err != nil {
		t.Fatalf("testDB failed to open: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("testDB failed to ping: %v", err)
	}
	// Setup a test table for the tests.
	_, err = db.Exec(ctx, "DROP TABLE IF EXISTS people")
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
	ID        int     `sqlp:"id"`
	FirstName string  `sqlp:"first_name"`
	LastName  string  `sqlp:"last_name"`
	Child     *person `sqlp:"child"`
	timestamps
}

type timestamps struct {
	CreatedAt time.Time `sqlp:"created_at"`
	UpdatedAt time.Time `sqlp:"updated_at"`
}

func isWithinDuration(t1 time.Time, t2 time.Time, d time.Duration) bool {
	return time.Duration(math.Abs(float64(t1.Sub(t2)))) <= d
}

func _personComparer(x, y person) bool {
	if !(x.ID == y.ID &&
		x.FirstName == y.FirstName &&
		x.LastName == y.LastName &&
		isWithinDuration(x.CreatedAt, y.CreatedAt, 5*time.Second) &&
		isWithinDuration(x.UpdatedAt, y.UpdatedAt, 5*time.Second)) {
		return false
	}
	if x.Child == nil && y.Child == nil {
		return true
	}
	if x.Child != nil && y.Child != nil {
		return _personComparer(*x.Child, *y.Child)
	}
	return false
}

var personComparer = cmp.Comparer(_personComparer)
