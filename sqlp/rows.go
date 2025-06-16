package sqlp

import (
	"database/sql"
)

// OutScanner is like a rows.Scan, but scans out values instead of into destinations.
type OutScanner[E any] interface {
	Scan() (E, error)
}

// Rows is a thin wrapper around sql.Rows to allow for easy access to sqlp's extensions.
type Rows[E any] struct {
	*sql.Rows
	scanner OutScanner[E]
}

func NewRows[E any](rows *sql.Rows) *Rows[E] {
	return &Rows[E]{
		Rows:    rows,
		scanner: NewReflectScanner[E](rows),
	}
}

func WrapRows[E any](rows *sql.Rows, e error) (*Rows[E], error) {
	if e != nil {
		return nil, e
	}
	return NewRows[E](rows), nil
}

func (r *Rows[E]) SetMapper(m Mapper[E]) {
	if m != nil {
		r.scanner = NewMappingScanner(r.Rows, m)
	}
}

// ScanOut scans out a row into our generic type E.
func (r *Rows[E]) ScanOut() (E, error) {
	return r.scanner.Scan()
}
