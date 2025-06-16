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
		people, err := sqlp.Select[person](ctx, db, "SELECT * FROM people")
		if err != nil {
			return fmt.Errorf("select people failed: %w", err)
		}
		log.Printf("found %d people\n", len(people))

		// Get into a struct
		p, err := sqlp.Get[person](ctx, db, "SELECT * FROM people LIMIT 1")
		if err != nil {
			return fmt.Errorf("get person failed: %w", err)
		}
		log.Printf("found person %v\n", p.ID)

		// Or for row by row:
		rows, err := db.Query(ctx, "SELECT * FROM people")
		if err != nil {
			return fmt.Errorf("query people failed: %w", err)
		}
		defer rows.Close()
		scanner := sqlp.NewReflectScanner[person](rows)

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
