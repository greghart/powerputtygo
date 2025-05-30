package sqlp_test

import (
	"context"
	"fmt"
	"log"

	"github.com/greghart/powerputtygo/sqlp"
)

func Example_reflect() {
	// assuming people database is already setup
	db, err := sqlp.Open("sqlite3", "./test.db")
	if err != nil {
		log.Panicf("testDB failed to open: %v", err)
	}

	err = db.RunInTx(context.Background(), func(ctx context.Context) error {
		// Select into slice
		// Will fail before query if Person is not setup correctly
		people := []person{}
		err := db.Select(ctx, &people, "SELECT * FROM people")
		if err != nil {
			return fmt.Errorf("select people failed: %w", err)
		}

		// Get into a struct
		p := person{}
		err = db.Get(ctx, &p, "SELECT * FROM people LIMIT 1")
		if err != nil {
			return fmt.Errorf("get person failed: %w", err)
		}

		// Or for row by row:
		rows, err := db.Query(ctx, "SELECT * FROM people")
		if err != nil {
			return fmt.Errorf("query people failed: %w", err)
		}
		defer rows.Close()
		scanner, err := sqlp.NewReflectScanner[person](rows)
		// If the struct is not setup correctly, this will fail
		// Eg. duplicate column names
		if err != nil {
			return fmt.Errorf("failed to create scanner: %w", err)
		}
		for rows.Next() {
			p, err := scanner.Scan()
			if err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}
			log.Printf("scanned person name: %s", p.FirstName)
		}

		return nil
	})
	if err != nil {
		log.Panicf("transaction not committed: %v", err)
	}
}
