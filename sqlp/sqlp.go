package sqlp

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

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
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
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

////////////////////////////////////////////////////////////////////////////////
// Reflective APIs

// Select runs a query and scans the results into dest, using reflection to scan.
func (db *DB) Select(ctx context.Context, dest any, query string, args ...any) error {
	// Validate destination types, we want a pointer to a slice of structs (or pointers to structs).
	destType := reflect.TypeOf(dest)
	if destType.Kind() != reflect.Pointer {
		return fmt.Errorf("select given %T, wanted a pointer", dest)
	}
	sliceType := destType.Elem()
	if sliceType.Kind() != reflect.Slice {
		return fmt.Errorf("select given %T, wanted a slice", dest)
	}
	elemType := sliceType.Elem()
	if elemType.Kind() == reflect.Pointer {
		return fmt.Errorf("given slice of pointers, wanted a slice of structs")
	}
	rv, err := reflectp.StructFieldsFactory(elemType)
	if err != nil {
		return fmt.Errorf("failed to reflect fields for %T: %w", elemType, err)
	}
	destV := reflect.ValueOf(dest).Elem()

	// Run the query
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Prepare row scanning
	scan, err := rv.Scanner(rows)
	if err != nil {
		return fmt.Errorf("failed to get scanner: %w", err)
	}

	for rows.Next() {
		val, err := scan()
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		destV.Set(reflect.Append(destV, val.Elem()))
	}

	return rows.Err()
}
