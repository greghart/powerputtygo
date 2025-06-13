package queryp

import (
	"fmt"
	"strings"
)

// NamedQuery represents a SQL query with named parameters.
// It defers the actually building of the final query and its' arguments until either
// `String()` or `Args()` is called, since order of the replacement matters in cases where the
// driver doesn't use positional arguments (like sqlite)
type NamedQuery struct {
	query         string
	params        map[string]any // Store named parameters
	placeholderer Placeholderer
	builtQuery    string
	builtArgs     *Args
}

func Named(query string) *NamedQuery {
	return &NamedQuery{
		query:  query,
		params: make(map[string]any),
	}
}

// WithPlaceholderer sets the Placeholderer for the NamedQuery.
func (n *NamedQuery) WithPlaceholderer(p Placeholderer) *NamedQuery {
	n.reset()
	n.placeholderer = p
	return n
}

// WithQuery sets the query string for the NamedQuery.
func (n *NamedQuery) WithQuery(q string) *NamedQuery {
	n.reset()
	n.query = q
	return n
}

// Params adds the given map of params to the NamedQuery.
func (n *NamedQuery) Params(m map[string]any) *NamedQuery {
	n.reset()
	for key, value := range m {
		n.params[key] = value
	}
	return n
}

// Param adds a single named parameter to the NamedQuery.
func (n *NamedQuery) Param(key string, v any) *NamedQuery {
	n.reset()
	n.params[key] = v
	return n
}

// String returns the final built query with all named parameters replaced.
func (n *NamedQuery) String() string {
	if n.builtArgs == nil {
		n.build()
	}
	return n.builtQuery
}

// Args returns the arguments for the query, with named parameters replaced by their placeholders.
func (n *NamedQuery) Args() []any {
	if n.builtArgs == nil {
		n.build()
	}
	return n.builtArgs.Args()
}

// Execute returns the query and arguments for the named query.
func (n *NamedQuery) Execute() (string, []any) {
	if n.builtArgs == nil {
		n.build()
	}
	return n.builtQuery, n.builtArgs.Args()
}

////////////////////////////////////////////////////////////////////////////////

func (n *NamedQuery) reset() {
	n.builtArgs = nil
	n.builtQuery = ""
}

// build constructs the final query string and arguments based on the named parameters.
func (n *NamedQuery) build() {
	n.builtArgs = NewArgs().WithPlaceholderer(n.placeholderer)

	q := strings.Builder{}
	// Order matters!
	// Eg. for 'WHERE id = :id AND name = :name', we need to ensure id arg comes before name arg.
	// Performance isn't a huge concern here, so just use a double loop, but we could optimize
	// by setting up a trie or something if needed
	for i := 0; i < len(n.query); i++ {
		c := n.query[i]
		// Is there a matching named parameter starting at i?
		match := false
		if c == ':' {
			for k, v := range n.params {
				if strings.HasPrefix(n.query[i:], fmt.Sprintf(":%s", k)) {
					match = true
					q.WriteString(n.builtArgs.Add(v))
					i += len(k) // skip over the ":key" part
					break
				}
			}
		}
		if !match {
			q.WriteByte(c)
		}
	}
	n.builtQuery = q.String()
}
