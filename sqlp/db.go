package sqlp

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/greghart/powerputtygo/queryp"
	"github.com/greghart/powerputtygo/sqlp/internal/reflectp"
)

// DB extends the stdlib sql.DB type to add additional behavior.
type DB struct {
	*sql.DB
}

// NewDB builds a new sqlp.DB for when you already have an existing sql.DB.
func NewDB(db *sql.DB) *DB {
	return &DB{db}
}

func Open(driverName, dataSourceName string) (*DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	return NewDB(db), nil
}

////////////////////////////////////////////////////////////////////////////////
// Standardized APIs

// Exec runs ExecContext.
func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.queryer(ctx).ExecContext(ctx, query, args...)
}

// Query runs QueryContext.
func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.queryer(ctx).QueryContext(ctx, query, args...)
}

// QueryRow runs QueryRowContext.
func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.queryer(ctx).QueryRowContext(ctx, query, args...)
}

////////////////////////////////////////////////////////////////////////////////
// Transactional APIs

type Queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type contextKeyType string

const (
	ctxKey = contextKeyType("sqlp")
)

// RunInTx runs the callback fxn in a transaction.
// If context already has a transaction, it will use that one.
// You can return an error from the callback to trigger the transaction to rollback.
func (db *DB) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	tx := db.txContext(ctx)
	// Setup new tx as needed.
	if tx == nil {
		_tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		tx = _tx
		defer func() {
			err := tx.Rollback()
			if err != nil && err != sql.ErrTxDone {
				// Rolled back due to error, but errored on rollback.
				fmt.Printf("failed to rollback transaction: %v\n", err)
			}
		}()
		ctx = context.WithValue(ctx, ctxKey, tx)
	}

	if err := fn(ctx); err != nil {
		return err
	}

	return tx.Commit()
}

// queryer returns the proper queryer for context, whether a Tx or normal DB.
func (db *DB) queryer(ctx context.Context) Queryer {
	if tx := db.txContext(ctx); tx != nil {
		return tx
	}
	return db.DB
}

// txContext returns contexts current transaction if any.
func (db *DB) txContext(ctx context.Context) *sql.Tx {
	if tx := ctx.Value(ctxKey); tx != nil {
		return tx.(*sql.Tx)
	}
	return nil
}

// Query is a convenience function to get a *sqlp.Rows for scanning out E.
func Query[E any](ctx context.Context, db *DB, query string, args ...any) (*Rows[E], error) {
	return WrapRows[E](db.Query(ctx, query, args...))
}

// Get is a convenience function to quickly get an entity out of a query using reflection.
func Get[E any](ctx context.Context, db *DB, query string, args ...any) (*E, error) {
	rows, err := Query[E](ctx, db, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, rows.Err()
	}
	e, err := rows.ScanOut()
	if err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}
	return &e, rows.Err()
}

// Select is a convenience function to quickly get a slice of entities out of a query using reflection.
func Select[E any](ctx context.Context, db *DB, query string, args ...any) ([]E, error) {
	rows, err := Query[E](ctx, db, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer rows.Close()

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

// Insert is a convenience function to insert an entity into the database.
// Uses sqlite placeholderer by default, and is more just here for reference --
// re-implementing yourself is recommended for performance and flexibility.
func Insert[E any](ctx context.Context, db *DB, table string, e E) (sql.Result, error) {
	val := reflect.ValueOf(e)
	fields, err := reflectp.FieldsFactory(val.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to reflect fields for %T: %w", e, err)
	}

	columns := make([]string, 0, len(fields.ByColumnName))
	placeholders := make([]string, 0, len(fields.ByColumnName))
	args := queryp.NewArgs()
	for col, field := range fields.Writable() {
		columns = append(columns, col)
		colValue, err := val.FieldByIndexErr(field.Index)
		if err != nil {
			return nil, fmt.Errorf("failed to get field %s from %T: %w", col, e, err)
		}
		placeholders = append(placeholders, args.Add(colValue.Interface()))
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)
	result, err := db.Exec(ctx, query, args.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert: %w", err)
	}
	return result, nil
}

func Update[E Identifiable](ctx context.Context, db *DB, table string, e E) (sql.Result, error) {
	val := reflect.ValueOf(e)
	fields, err := reflectp.FieldsFactory(val.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to reflect fields for %T: %w", e, err)
	}

	sets := make([]string, 0, len(fields.ByColumnName))
	args := queryp.NewArgs()
	for col, field := range fields.Writable() {
		if col == e.IDColumn() {
			continue // never write id column
		}
		colValue, err := val.FieldByIndexErr(field.Index)
		if err != nil {
			return nil, fmt.Errorf("failed to get field %s from %T: %w", col, e, err)
		}
		placeholder := args.Add(colValue.Interface())
		sets = append(sets, fmt.Sprintf("%s = %s", col, placeholder))
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = %s",
		table,
		strings.Join(sets, ", "),
		e.IDColumn(),
		args.Add(e.ID()),
	)
	result, err := db.Exec(ctx, query, args.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert: %w", err)
	}
	return result, nil
}

// Identifiable helps us constrain generics that can be identified with some sort of primary key.
type Identifiable interface {
	ID() any          // ID returns the unique identifier for the entity.
	IDColumn() string // IDColumn returns the name of the column that contains the ID.
}
