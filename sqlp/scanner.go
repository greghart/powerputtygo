package sqlp

import (
	"database/sql"
	"fmt"
	"reflect"

	"github.com/greghart/powerputtygo/sqlp/internal/reflectp"
)

// ReflectScanner uses a generic type parameter to return values instead of scanning into destinations
type ReflectScanner[E any] struct {
	*ReflectDestScanner
}

func NewReflectScanner[E any](rows *sql.Rows) (*ReflectScanner[E], error) {
	// Type parameter lets us check validity immediately
	var e E
	destFields, err := reflectp.FieldsFactory(reflect.TypeOf(e))
	if err != nil {
		return nil, fmt.Errorf("failed to reflect fields for %T: %w", reflect.TypeOf(e), err)
	}
	fRows, err := destFields.Rows(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get fields rows: %w", err)
	}
	return &ReflectScanner[E]{
		ReflectDestScanner: &ReflectDestScanner{
			Rows:  rows,
			fRows: fRows,
		},
	}, nil
}

// Scan will scan into the given destination using reflection to map columns to fields.
// Note, if called multiple times with different destinations, will just panic.
func (rs *ReflectScanner[E]) Scan() (E, error) {
	var e E
	err := rs.ReflectDestScanner.Scan(&e)
	return e, err
}

// //////////////////////////////////////////////////////////////////////////////

// ReflectDestScanner is similar to ReflectScanner, but scans into a destination rather than
// initializing new datums itself. Useful for considerate memory management and a more conventional
// `Scan` API
type ReflectDestScanner struct {
	*sql.Rows
	fRows *reflectp.FieldsRows
}

func NewReflectDestScanner(rows *sql.Rows) *ReflectDestScanner {
	return &ReflectDestScanner{
		Rows: rows,
	}
}

// Scan will scan into the given destination using reflection to map columns to fields.
// Note, if called multiple times with different destinations, will just panic.
func (rs *ReflectDestScanner) Scan(dest any) error {
	destV := reflect.ValueOf(dest)
	if rs.fRows == nil {
		destType := destV.Type()
		if destType.Kind() != reflect.Pointer {
			return fmt.Errorf("reflect dest scanner given %T, wanted a pointer", dest)
		}
		elemType := destType.Elem()
		destFields, err := reflectp.FieldsFactory(elemType)
		if err != nil {
			return fmt.Errorf("failed to reflect fields for %T: %w", elemType, err)
		}
		fRows, err := destFields.Rows(rs.Rows)
		if err != nil {
			return fmt.Errorf("failed to get fields rows: %w", err)
		}
		rs.fRows = fRows
	}

	_, err := rs.fRows.Scan(destV)
	return err
}

////////////////////////////////////////////////////////////////////////////////

type MappingScanner[E any] struct {
	*sql.Rows
	cols   []string
	mapper Mapper[E]
}

func NewMappingScanner[E any](rows *sql.Rows, mapper Mapper[E]) *MappingScanner[E] {
	return &MappingScanner[E]{
		Rows:   rows,
		mapper: mapper,
	}
}

func (ms *MappingScanner[E]) Scan() (E, error) {
	var e E

	if ms.cols == nil {
		cols, err := ms.Columns()
		if err != nil {
			return e, fmt.Errorf("failed to get columns: %w", err)
		}
		ms.cols = cols
	}

	targets := make([]any, len(ms.cols))
	for i, c := range ms.cols {
		addr, ok := ms.mapper.Addr(&e, c)
		if !ok {
			return e, fmt.Errorf("failed to get mapping for %v", c)
		}
		targets[i] = addr
	}

	return e, ms.Rows.Scan(targets...)
}
