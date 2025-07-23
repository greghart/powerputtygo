package sqlp

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/greghart/powerputtygo/queryp"
	"github.com/greghart/powerputtygo/sqlp/internal/reflectp"
)

// DB extends the stdlib sql.DB type to add additional behavior.
type DB struct {
	*sql.DB
	logger  *slog.Logger
	metrics DBMetrics
}

type DBOptions struct {
	Logger  *slog.Logger
	Metrics DBMetrics
}

// NewDB builds a new sqlp.DB for when you already have an existing sql.DB.
func NewDB(db *sql.DB, _options ...DBOptions) *DB {
	opts := DBOptions{}
	if len(_options) == 1 {
		opts = _options[0]
	}
	logger := slog.Default()
	if opts.Logger != nil {
		logger = _options[0].Logger.WithGroup("sqlp")
	}
	return &DB{
		DB:      db,
		logger:  logger,
		metrics: opts.Metrics,
	}
}

func Open(driverName, dataSourceName string, _options ...DBOptions) (*DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	return NewDB(db, _options...), nil
}

////////////////////////////////////////////////////////////////////////////////
// Standardized APIs

// Exec runs ExecContext.
func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	db.logger.Debug("Exec", "query", slog.StringValue(query), "args", &args)
	db.metrics.IncQuery()
	defer db.metrics.ObserveDuration()()

	return db.queryer(ctx).ExecContext(ctx, query, args...)
}

// Query runs QueryContext.
func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	db.logger.Debug("Query", "query", slog.StringValue(query), "args", &args)
	db.metrics.IncQuery()
	defer db.metrics.ObserveDuration()()

	return db.queryer(ctx).QueryContext(ctx, query, args...)
}

// QueryRow runs QueryRowContext.
func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	db.logger.Debug("QueryRow", "query", slog.StringValue(query), "args", &args)
	db.metrics.IncQuery()
	defer db.metrics.ObserveDuration()()

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
		columns = append(columns, fmt.Sprintf("\"%s\"", col))
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
	if len(columns) == 0 {
		query = fmt.Sprintf("INSERT INTO %s DEFAULT VALUES", table)
	}
	result, err := db.Exec(ctx, query, args.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert: %w", err)
	}
	return result, nil
}

// InsertBulk is a convenience function to insert multiple entities into the database.
// Uses sqlite placeholderer by default, and is more just here for reference --
// re-implementing yourself is recommended for performance and flexibility.
// TODO: Figure out how to inject a custom placeholderer into this API.
// Should probably just refactor to use a struct of options here?
func InsertBulk[E any](ctx context.Context, db *DB, table string, entities []E) (sql.Result, error) {
	if len(entities) == 0 {
		return nil, nil
	}

	val := reflect.ValueOf(entities[0])
	structFields, err := reflectp.FieldsFactory(val.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to reflect fields for %T: %w", entities[0], err)
	}

	// build columns to insert
	fieldsByColumn := structFields.Writable()
	columns := make([]string, 0, len(fieldsByColumn))
	fields := make([]*reflectp.Field, 0, len(fieldsByColumn))
	for col, field := range fieldsByColumn {
		columns = append(columns, fmt.Sprintf("\"%s\"", col))
		fields = append(fields, field)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("cannot bulk insert default values into %s", table)
	}

	// build multiple values tuples out of entities
	values := strings.Builder{}
	args := queryp.NewArgs(len(fieldsByColumn) * len(entities))
	for i := range entities {
		if i != 0 {
			values.WriteString(",\n")
		}
		values.WriteRune('(')
		val := reflect.ValueOf(entities[i])
		firstColumn := true
		for _, field := range fields {
			colValue, err := val.FieldByIndexErr(field.Index)
			if err != nil {
				return nil, fmt.Errorf("failed to get field %s from entity %T: %w", field.Column, entities[0], err)
			}
			if !firstColumn {
				values.WriteRune(',')
			}
			firstColumn = false
			values.WriteString(args.Add(colValue.Interface()))
		}
		values.WriteRune(')')
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s",
		table,
		strings.Join(columns, ", "),
		values.String(),
	)
	if len(columns) == 0 {
		query = fmt.Sprintf("INSERT INTO %s DEFAULT VALUES", table)
	}
	result, err := db.Exec(ctx, query, args.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk insert: %w", err)
	}
	return result, nil
}

func Update[E any](ctx context.Context, db *DB, table string, e *E, ider Identifier[*E]) (sql.Result, error) {
	if e == nil {
		return nil, fmt.Errorf("cannot update nil entity")
	}
	val := reflect.ValueOf(*e)
	fields, err := reflectp.FieldsFactory(val.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to reflect fields for %T: %w", e, err)
	}

	sets := make([]string, 0, len(fields.ByColumnName))
	args := queryp.NewArgs()
	for col, field := range fields.Writable() {
		if col == ider.IDColumn() {
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
		ider.IDColumn(),
		args.Add(ider.ID(e)),
	)
	result, err := db.Exec(ctx, query, args.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert: %w", err)
	}
	return result, nil
}

// Identifier helps us identify entities for templated updates and inserts
type Identifier[E any] interface {
	// GetID returns the unique identifier for the entity.
	ID(e E) any
	// GetIDColumn returns the name of the column that contains the ID.
	IDColumn() string
}

type PKIdentifier[E any] struct {
	column string
	getID  func(e E) any
}

func NewPKIdentifier[E any](column string, getID func(e E) any) *PKIdentifier[E] {
	return &PKIdentifier[E]{column, getID}
}

func (pk *PKIdentifier[E]) ID(e E) any {
	return pk.getID(e)
}

func (pk *PKIdentifier[E]) IDColumn() string {
	return pk.column
}

////////////////////////////////////////////////////////////////////////////////

type DBMetrics struct {
	QueryCounter MetricCounter
	// QueryDurationObserver is used to observe the duration of queries (db time)
	QueryDurationObserver MetricObserver
}

// MetricCounter is used for query counting.
// Copied from prometheus to avoid dependency here.
type MetricCounter interface {
	Inc()
}

// MetricObserver is used for query duration observation.
// Copied from prometheus to avoid dependency here.
type MetricObserver interface {
	Observe(float64)
}

func (m *DBMetrics) IncQuery() {
	if m == nil || m.QueryCounter == nil {
		return
	}
	m.QueryCounter.Inc()
}

func (m *DBMetrics) ObserveDuration() func() {
	if m == nil || m.QueryDurationObserver == nil {
		return func() {}
	}
	start := time.Now()
	return func() {
		m.QueryDurationObserver.Observe(time.Since(start).Seconds())
	}
}
