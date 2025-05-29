package sqlp

import (
	"context"
	"fmt"
	"reflect"

	"github.com/greghart/powerputtygo/sqlp/internal/reflectp"
)

// Repository provides a data access layer for a specific entity
type Repository[E any] struct {
	*DB
	entity E
	table  string
	t      reflect.Type
}

func NewRepository[E any](db *DB, table string) *Repository[E] {
	var entity E
	return &Repository[E]{DB: db, entity: entity, table: table, t: reflect.TypeOf(entity)}
}

// Runs reflection process to ensure entity is setup correctly
func (r *Repository[E]) Validate() error {
	_, err := reflectp.FieldsFactory(r.t)
	return err
}

// Find retrieves an entity by its ID, assuming `id` is the primary key.
// Note, this is setup for reference as much as usage. Such methods are trivial to write yourself,
// rather than unnecessarily complicate struct tags to tag pks and other fields.
func (r *Repository[E]) Find(ctx context.Context, id int) (E, error) {
	return r.Get(
		ctx,
		"SELECT * FROM "+r.table+" WHERE id = ?",
		id,
	)
}

func (r *Repository[E]) Get(ctx context.Context, q string, args ...any) (E, error) {
	var entity E
	entities, err := r.Select(ctx, q, args...)
	if len(entities) > 0 {
		entity = entities[0]
	}
	return entity, err
}

func (r *Repository[E]) Select(ctx context.Context, q string, args ...any) ([]E, error) {
	var entities []E
	// Re-implement DB#Select, to avoid using reflection for filling our results
	fields, err := reflectp.FieldsFactory(r.t) // should be cached with Validate
	if err != nil {
		return nil, fmt.Errorf("failed to reflect fields for %T: %w", r.t, err)
	}

	rows, err := r.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Prepare row scanning
	fRows, err := fields.Rows(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get fields rows: %w", err)
	}

	for rows.Next() {
		val, err := fRows.Scan()
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		entities = append(entities, val.Elem().Interface().(E))
	}

	return entities, rows.Err()
}
