package sqlp

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	_ "github.com/mattn/go-sqlite3"
)

type person struct {
	ID        int    `sqlp:"id"`
	FirstName string `sqlp:"first_name"`
	LastName  string `sqlp:"last_name"`
}

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

	db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "John", "Doe")
	db.Exec(ctx, "INSERT INTO people (first_name, last_name) VALUES (?, ?)", "Albert", "Einstein")

	t.Run("to slice of people", func(t *testing.T) {
		people := []person{}
		err := db.Select(ctx, &people, "SELECT id, first_name, last_name FROM people ORDER BY id ASC")
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}
		expected := []person{
			{ID: 1, FirstName: "John", LastName: "Doe"},
			{ID: 2, FirstName: "Albert", LastName: "Einstein"},
		}
		if !cmp.Equal(people, expected) {
			t.Fatalf("selected people unexpected:\n%v", cmp.Diff(people, expected))
		}
	})

	t.Run("to slice of people pointers", func(t *testing.T) {
		people := []*person{}
		err := db.Select(ctx, &people, "SELECT id, first_name, last_name FROM people ORDER BY id ASC")
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
	_, err = db.Exec(ctx, "CREATE TABLE IF NOT EXISTS people (id INTEGER PRIMARY KEY, first_name TEXT, last_name TEXT)")
	if err != nil {
		t.Fatalf("testDB failed to create table: %v", err)
	}
	return db, ctx, func() {
		db.Close()
		cancel()
	}
}
