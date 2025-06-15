package sqlp

import (
	"log"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReflectDestScanner(t *testing.T) {
	db, ctx, cleanup := testDB(t)
	defer cleanup()

	grandparent := grandchildrenSetup(ctx, db)
	albert := albertSetup(ctx, db)

	// destination scanning however we want
	rows, err := db.Query(ctx, selectGrandchildrenAndPets())
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()

	scanner := NewReflectDestScanner(rows)

	var people []person
	for rows.Next() {
		var p person
		if err := scanner.Scan(&p); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		people = append(people, p)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	expected := []person{grandparent, albert}
	if !cmp.Equal(people, expected, personComparer) {
		t.Errorf("selected people unexpected:\n%v", cmp.Diff(expected, people, personComparer))
	}
}

func TestMappingScanner(t *testing.T) {
	pm := personMapper(t)

	db, ctx, cleanup := testDB(t)
	defer cleanup()

	grandparent := grandchildrenSetup(ctx, db)
	albert := albertSetup(ctx, db)

	// destination scanning however we want
	rows, err := db.Query(ctx, selectGrandchildrenAndPets())
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()

	scanner := NewMappingScanner(rows, pm)

	var people []person
	for rows.Next() {
		p, err := scanner.Scan()
		if err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		people = append(people, p)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// TODO: Mapper currently does not nil out zero values that are setup in mappers
	// This is less of an issue, because users specifically do the "touch"ing themselves, so they
	// should be aware, but maybe there is something we can improve on.
	grandparent.Pet = &pet{}
	grandparent.Child.Child.Pet = &pet{}
	albert.Pet = &pet{}
	albert.Child = &person{
		Pet:   &pet{},
		Child: &person{Pet: &pet{}},
	}
	expected := []person{grandparent, albert}
	if !cmp.Equal(people, expected, personComparer) {
		t.Errorf("selected people unexpected:\n%v", cmp.Diff(expected, people, personComparer))
	}
}
