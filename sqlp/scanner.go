package sqlp

import (
	"database/sql"
	"fmt"
	"reflect"

	"github.com/greghart/powerputtygo/sqlp/internal/reflectp"
)

// ReflectScanner uses a generic type parameter to instantiate values instead of scanning into
// pre-existing destinations.
type ReflectScanner[E any] struct {
	dest *ReflectDestScanner
}

func NewReflectScanner[E any](rows *sql.Rows) *ReflectScanner[E] {
	return &ReflectScanner[E]{
		dest: NewReflectDestScanner(rows),
	}
}

// Scan will out E using reflection to map columns to fields.
func (rs *ReflectScanner[E]) Scan() (E, error) {
	var e E

	// Use the ReflectDestScanner to scan into a new value
	if err := rs.dest.Scan(&e); err != nil {
		return e, fmt.Errorf("failed to scan: %w", err)
	}

	return e, nil
}

// //////////////////////////////////////////////////////////////////////////////

// ReflectDestScanner is most similar to standard `sql` package, using reflection to dynamically
// map columns into struct field addresses.
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
	cols    []string
	targets []any // address targets for scanning
	mapper  Mapper[E]
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
		ms.targets = make([]any, len(ms.cols))
	}

	fallback := new(any)
	for i, c := range ms.cols {
		addr, ok := ms.mapper.Addr(&e, c)
		if !ok {
			// TODO: Options to make this an error
			// return e, fmt.Errorf("failed to get mapping for %v", c)
			ms.targets[i] = fallback
		} else {
			ms.targets[i] = addr
		}
	}

	return e, ms.Rows.Scan(ms.targets...)
}
