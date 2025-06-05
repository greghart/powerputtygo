package query

import "fmt"

// Args are a way to build placeholder arguments for queries in a composable way.
// This is a very simple paradigm that's not particularly useful by itself, but used with the
// named argument query builders
type Args struct {
	placeholderer Placeholderer
	args          []any
}

type Placeholderer func(i int) string

func NewArgs() *Args {
	return &Args{
		placeholderer: SqlitePlaceholderer, // Default to SQLite placeholder style
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
