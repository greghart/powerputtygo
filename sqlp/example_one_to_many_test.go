package sqlp_test

import (
	"context"
	"log"

	"github.com/greghart/powerputtygo/sqlp"
)

// Example_reflectOneToMany shows one option to handle one to many relationships. It would be up
// to the developer to handle aggregating the multiple rows of data into one resulting struct.
func Example_reflectOneToMany() {
	// assuming people database is already setup
	db, err := sqlp.Open("sqlite3", "./test.db")
	if err != nil {
		log.Panicf("testDB failed to open: %v", err)
	}

	// A custom type for what we're querying specifically
	type personRow struct {
		person
		child person `sqlp:"child"`
	}
	query := `
		SELECT p.id, p.first_name, p.last_name, 
			COALESCE(child.id, 0) AS child_id,
			COALESCE(child.first_name, "") AS child_first_name,
			COALESCE(child.last_name, "") AS child_last_name,
		FROM people p
		LEFT JOIN people child ON child.parent_id = p.id
		ORDER BY p.id /* Know we can get all children for a person sequentially */
	`
	rows, err := db.Query(context.Background(), query)
	if err != nil {
		log.Panicf("query failed: %v", err)
	}
	defer rows.Close()

	scanner, err := sqlp.NewReflectScanner[personRow](rows)
	if err != nil {
		log.Panicf("failed to reflect person scanner: %v", err)
	}

	people := []person{}
	pending := person{}
	for i := 0; rows.Next(); i++ {
		row, err := scanner.Scan()
		if err != nil {
			log.Panicf("failed to scan row: %v", err)
		}
		// New person being scanned
		if row.ID != pending.ID {
			// If we have a pending person, add them to the list
			if pending.ID != 0 {
				people = append(people, pending)
			}
			pending = row.person
		}
		// Add joined children
		if row.child.ID != 0 {
			pending.Children = append(pending.Children, row.child)
		}
	}
	log.Printf("scanned %d people", len(people))
}
