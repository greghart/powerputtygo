package sqlp_test

import (
	"context"
	"log"
	"time"

	"github.com/greghart/powerputtygo/sqlp"
)

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
	ID   int64  `sqlp:"id"`
	Name string `sqlp:"name"`
	Type string `sqlp:"type"`
}

type timestamps struct {
	CreatedAt time.Time `sqlp:"created_at"`
	UpdatedAt time.Time `sqlp:"updated_at"`
}

// Example shows the setup used by almost all examples in this documentation.
func Example() {
	db, err := sqlp.Open("sqlite3", "./test.db")
	if err != nil {
		log.Panicf("testDB failed to open: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Panicf("testDB failed to ping: %v", err)
	}

	// Setup a test table for the tests.
	_, err = db.Exec(ctx, "DROP TABLE IF EXISTS people; DROP TABLE IF EXISTS pets")
	if err != nil {
		log.Panicf("test DB failed to drop table: %v", err)
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
		log.Panicf("testDB failed to create table: %v", err)
	}
	db.Close()
}
