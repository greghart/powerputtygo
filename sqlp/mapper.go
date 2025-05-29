package sqlp

import "fmt"

// Mapper powers generic, non reflective mappings of column names to struct fields
type Mapper[E any] map[string]Mapping[*E]

// Mapping is a mapping directly to field on the struct
type Mapping[E any] func(e E) any

// Addr returns an address for given column against the given entity
func (m Mapper[E]) Addr(e *E, col string) (any, bool) {
	mapping, ok := m[col]
	if !ok {
		return nil, false
	}
	return mapping(e), true
}

// MergeMappers merges mappers of different types, to setup a sub mapper in a parent/child.
func MergeMappers[E, T any](m1 Mapper[E], m2 Mapper[T], ns string, get func(*E) *T) Mapper[E] {
	out := make(Mapper[E], len(m1)+len(m2))
	for k, mapping := range m1 {
		out[k] = mapping
	}
	for k, mapping := range m2 {
		out[fmt.Sprintf("%v_%v", ns, k)] = func(e *E) any {
			t := get(e)
			return mapping(t)
		}
	}
	return out
}
