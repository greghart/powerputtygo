package queryp

import (
	"fmt"
	"strings"
)

// Args are a way to build placeholder arguments for queries in a composable way.
// This is a very simple paradigm that's not particularly useful by itself, but used with the
// named argument query builders
type Args struct {
	placeholderer Placeholderer
	args          []any
}

type Placeholderer func(i int) string

func NewArgs(_capacity ...int) *Args {
	capacity := 0
	if len(_capacity) == 1 {
		capacity = _capacity[0]
	}
	return &Args{
		placeholderer: SqlitePlaceholderer, // Default to SQLite placeholder style
		args:          make([]any, 0, capacity),
	}
}

func (a *Args) WithPlaceholderer(p Placeholderer) *Args {
	if p != nil {
		a.placeholderer = p
	}
	return a
}

// Add adds an argument and returns a placeholder for it.
func (a *Args) Add(arg any) string {
	a.args = append(a.args, arg)
	return a.placeholderer(len(a.args) - 1)
}

// Slice supports IN style parameters, expanding them out to CSVs
// Eg. Slice([]int{1, 2, 3}) will return "?,?,?" and add the values 1, 2, 3 to the args slice.
func (a *Args) Slice(s []any) string {
	placeholders := make([]string, len(s))
	for i, v := range s {
		placeholders[i] = a.Add(v)
	}
	return strings.Join(placeholders, ",")
}

func (a *Args) Args() []any {
	return a.args
}

////////////////////////////////////////////////////////////////////////////////

var SqlitePlaceholderer = func(i int) string {
	return "?"
}

var PostgresPlaceholderer = func(i int) string {
	return fmt.Sprintf("$%d", i+1) // Postgres placeholders start at $1, so we add 1 to the index
}
