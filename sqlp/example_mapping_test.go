package sqlp_test

import (
	"context"
	"log"

	"github.com/greghart/powerputtygo/sqlp"
)

func Example_mapping() {
	// assuming people database is already setup
	db, err := sqlp.Open("sqlite3", "./test.db")
	if err != nil {
		log.Panicf("testDB failed to open: %v", err)
	}

	petMapper := sqlp.Mapper[pet]{
		"id":   func(p *pet) any { return &p.ID },
		"name": func(p *pet) any { return &p.Name },
		"type": func(p *pet) any { return &p.Type },
	}
	personMapper := sqlp.Mapper[person]{
		"id":         func(p *person) any { return &p.ID },
		"first_name": func(p *person) any { return &p.FirstName },
		"last_name":  func(p *person) any { return &p.LastName },
	}
	personMapper = sqlp.MergeMappers(personMapper, petMapper, "pet", func(p *person) *pet {
		if p.Pet == nil {
			p.Pet = &pet{}
		}
		return p.Pet
	})
	// Support children
	personMapper = sqlp.MergeMappers(personMapper, personMapper, "child", func(p *person) *person {
		if p.Child == nil {
			p.Child = &person{}
		}
		return p.Child
	})
	// Support grandchildren
	personMapper = sqlp.MergeMappers(personMapper, personMapper, "child", func(p *person) *person {
		if p.Child == nil {
			p.Child = &person{}
		}
		return p.Child
	})

	// We use COALESCE as a strategy to handle joins where there will be partial data
	query := `
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
	`
	rows, err := db.Query(context.Background(), query)
	if err != nil {
		log.Panicf("failed to query people: %v", err)
	}

	scanner := sqlp.NewMappingScanner(rows, personMapper)
	for rows.Next() {
		p, err := scanner.Scan() // p is a person!
		if err != nil {
			log.Panicf("failed to scan row: %v", err)
		}
		log.Printf("Person: %+v", p)
		// Note that p.Pet will be a zero object if there is no pet
	}
}
