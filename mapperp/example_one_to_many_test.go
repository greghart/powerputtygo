package mapperp_test

import (
	"context"
	"log"

	"github.com/greghart/powerputtygo/mapperp"
	"github.com/greghart/powerputtygo/sqlp"
)

// Example_reflectOneToMany shows one option to handle one to many relationships. It would be up
// to the developer to handle aggregating the multiple rows of data into one resulting struct.
func Example_mapOneToMany() {
	// assuming people database is already setup
	db, err := sqlp.Open("sqlite3", "./test.db")
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
	rows, err := db.Query(context.Background(), query)
	if err != nil {
		log.Panicf("query failed: %v", err)
	}
	defer rows.Close()

	// Use sqlp to scan rows easily
	type personRow struct { // A custom type for what we're querying specifically
		person
		pet pet `sqlp:"pet"`
	}
	// Use sqlp to scan data into our flat row based struct
	scanner, err := sqlp.NewReflectScanner[personRow](rows)
	if err != nil {
		log.Panicf("failed to reflect person scanner: %v", err)
	}

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
		row, err := scanner.Scan()
		if err != nil {
			log.Panicf("failed to scan row: %v", err)
		}
		personMapper(&person, &row, i) // Map the row onto our person
	}
	log.Printf("scanned person: %+v", person)
}
