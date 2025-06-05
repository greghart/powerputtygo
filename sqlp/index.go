package sqlp

import "github.com/greghart/powerputtygo/sqlp/pkg/query"

// Support top level imports without drilling into our separated packages.
// Just a convenience for users, while letting us keep code organized into sub packages.

func Named(q string) *query.NamedQuery {
	return query.Named(q)
}
