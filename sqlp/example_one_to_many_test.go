package sqlp_test

import (
	"context"
	"log"

	"github.com/greghart/powerputtygo/mapperp"
	"github.com/greghart/powerputtygo/sqlp"
)

// Example_reflectOneToMany shows one option to handle one to many relationships. It would be up
// to the developer to handle aggregating the multiple rows of data into one resulting struct.
// You can also use mapperp to handle this more easily.
func Example_oneToMany() {
	// assuming people database is already setup
	db, err := sqlp.Open("sqlite3", ":memory:")
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
	rows, err := sqlp.Query[personRow](context.Background(), db, query)
	if err != nil {
		log.Panicf("query failed: %v", err)
	}
	defer rows.Close()

	people := []person{}
	pending := person{}
	for i := 0; rows.Next(); i++ {
		row, err := rows.ScanOut()
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

// Example_reflectOneToMany shows one option to handle one to many relationships. It would be up
// to the developer to handle aggregating the multiple rows of data into one resulting struct.
func Example_oneToMany_mapper() {
	// assuming people database is already setup
	db, err := sqlp.Open("sqlite3", ":memory:")
	if err != nil {
		log.Panicf("testDB failed to open: %v", err)
	}

	query := `
		SELECT 
			p.id, p.first_name, p.last_name,
			COALESCE(pet.id, 0) AS pet_id,
			COALESCE(pet.name, "") AS pet_name
		FROM people p
		LEFT JOIN pets pet ON pet.parent_id = p.id
		WHERE p.id = 1
	`
	// Use sqlp to scan rows easily
	type personRow struct { // A custom type for what we're querying specifically
		person
		pet pet `sqlp:"pet"`
	}

	rows, err := sqlp.Query[personRow](context.Background(), db, query)
	if err != nil {
		log.Panicf("query failed: %v", err)
	}
	defer rows.Close()

	// Use mapperp to map these rows to our domain models
	personMapper := mapperp.One( // First off, we want just one person
		func(row *personRow) *person { return &row.person },
		mapperp.InnerSlice( // With many pets
			func(p *person) *[]pet { return &p.Pets },
			func(e *pet) int64 { return e.ID },
			func(row *personRow) *pet { return &row.pet },
		),
	)
	var person person

	for i := 0; rows.Next(); i++ {
		row, err := rows.ScanOut()
		if err != nil {
			log.Panicf("failed to scan row: %v", err)
		}
		personMapper(&person, &row, i) // Map the row onto our person
	}
}
