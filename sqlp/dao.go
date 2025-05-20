package sqlp

import (
	"context"
	"fmt"
	"reflect"

	"github.com/greghart/powerputtygo/sqlp/internal/reflectp"
)

// DAO provides a data access layer for a specific entity
type DAO[E any] struct {
	*DB
	entity E
	table  string
	t      reflect.Type
}

func NewDAO[E any](db *DB, table string) *DAO[E] {
	var entity E
	return &DAO[E]{DB: db, entity: entity, table: table, t: reflect.TypeOf(entity)}
}

// Runs reflection process to ensure entity is setup correctly
func (dao *DAO[E]) Validate() error {
	_, err := reflectp.FieldsFactory(dao.t)
	return err
}

// Find retrieves an entity by its ID, assuming `id` is the primary key.
// Note, this is setup for reference as much as usage. Such methods are trivial to write yourself,
// rather than unnecessarily complicate struct tags to tag pks and other fields.
func (dao *DAO[E]) Find(ctx context.Context, id int) (E, error) {
	return dao.Get(
		ctx,
		"SELECT * FROM "+dao.table+" WHERE id = ?",
		id,
	)
}

func (dao *DAO[E]) Get(ctx context.Context, q string, args ...any) (E, error) {
	var entity E
	entities, err := dao.Select(ctx, q, args...)
	if len(entities) > 0 {
		entity = entities[0]
	}
	return entity, err
}

func (dao *DAO[E]) Select(ctx context.Context, q string, args ...any) ([]E, error) {
	var entities []E
	// Re-implement DB#Select, avoiding using reflection for filling our results
	fields, err := reflectp.FieldsFactory(dao.t) // should be cached with Validate
	if err != nil {
		return nil, fmt.Errorf("failed to reflect fields for %T: %w", dao.t, err)
	}

	rows, err := dao.DB.Query(ctx, q, args...)
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
