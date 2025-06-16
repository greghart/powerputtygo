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
	mapper Mapper[E]
}

func NewRepository[E any](db *DB, table string) *Repository[E] {
	var entity E
	return &Repository[E]{
		DB:     db,
		entity: entity,
		table:  table,
		t:      reflect.TypeOf(entity),
		mapper: nil,
	}
}

// Runs reflection process to ensure entity is setup correctly
func (r *Repository[E]) Validate() error {
	if r.mapper == nil {
		_, err := reflectp.FieldsFactory(r.t)
		return err
	}
	return nil
}

// SetMapper sets a custom column mapper which will be used for all queries instead of reflection.
func (r *Repository[E]) SetMapper(mapper Mapper[E]) {
	r.mapper = mapper
}

// Find retrieves an entity by its ID, assuming `id` is the primary key.
// Note, this is setup for reference as much as usage. Such methods are trivial to write yourself,
// rather than unnecessarily complicate struct tags to tag pks and other fields.
func (r *Repository[E]) Find(ctx context.Context, id int) (*E, error) {
	return r.Get(
		ctx,
		"SELECT * FROM "+r.table+" WHERE id = ?",
		id,
	)
}

// Get functions very similarly to `sqlp.Get`, but obeys uses custom mapper, if any.
func (r *Repository[E]) Get(ctx context.Context, q string, args ...any) (*E, error) {
	var entity *E
	entities, err := r.Select(ctx, q, args...)
	if len(entities) > 0 {
		e := entities[0] // copy out of array
		entity = &e
	}
	return entity, err
}

// Select functions very similarly to `sqlp.Select`, but obeys uses custom mapper, if any.
func (r *Repository[E]) Select(ctx context.Context, q string, args ...any) ([]E, error) {
	rows, err := Query[E](ctx, r.DB, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer rows.Close()
	rows.SetMapper(r.mapper)

	var results []E
	for rows.Next() {
		e, err := rows.ScanOut()
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, e)
	}

	return results, rows.Err()
}
