package servicep

import (
	"iter"
	"strings"
)

// IncludeSchema lets you define your inclusion schema, and handle individaul include requests.
type IncludeSchema struct {
	allowed map[string]bool
}

func NewIncludeSchema() *IncludeSchema {
	return &IncludeSchema{}
}

// Allow sets associations that are allowed to be included.
// Only enforced at add time, doesn't retroactively remove anything
func (i *IncludeSchema) Allow(allows ...string) *IncludeSchema {
	if i.allowed == nil && len(allows) > 0 {
		i.allowed = make(map[string]bool, len(allows))
	}
	for _, assoc := range allows {
		components := strings.Split(assoc, ".")
		for j := range len(components) {
			i.allowed[strings.Join(components[:j+1], ".")] = j == len(components)-1
		}
	}
	return i
}

func (i *IncludeSchema) IsAllowed(s string) bool {
	if i == nil || i.allowed == nil {
		return true // no restrictions, all inclusions are allowed
	}
	_, ok := i.allowed[s]
	return ok
}

// Include returns a new include request against this schema.
func (i *IncludeSchema) Include(associations ...string) *IncludeRequest {
	return NewIncludeRequest().SetSchema(i).Include(associations...)
}

////////////////////////////////////////////////////////////////////////////////

type IncludeRequest struct {
	// schema to use, if any
	schema *IncludeSchema
	// map of included association, to bool whether it's the full path specified by user
	includes map[string]bool
}

// NewIncludeRequest sets up a new wrapper around request level includes.
func NewIncludeRequest() *IncludeRequest {
	return &IncludeRequest{
		includes: make(map[string]bool),
	}
}

func (req *IncludeRequest) SetSchema(schema *IncludeSchema) *IncludeRequest {
	if req == nil {
		req = NewIncludeRequest()
	}
	req.schema = schema
	return req
}

func (req *IncludeRequest) Include(associations ...string) *IncludeRequest {
	if req == nil {
		req = NewIncludeRequest()
	}
	for _, assoc := range associations {
		components := strings.Split(assoc, ".")
		for j := range len(components) {
			path := strings.Join(components[:j+1], ".")
			if !req.schema.IsAllowed(path) {
				continue
			}
			req.includes[path] = j == len(components)-1
		}
	}
	return req
}

// All returns an iterator over all user set includes
func (req *IncludeRequest) All() iter.Seq[string] {
	return func(yield func(string) bool) {
		if req == nil {
			return
		}
		for i, v := range req.includes {
			if !v { // only yield full paths
				continue
			}
			if !yield(i) {
				return // stop iteration if yield returns false
			}
		}
	}
}

func (req *IncludeRequest) Filter(f func(s string) bool) iter.Seq[string] {
	return func(yield func(string) bool) {
		for v := range req.All() {
			if f(v) {
				if !yield(v) {
					return // stop iteration if yield returns false
				}
			}
		}
	}
}

func (req *IncludeRequest) IsIncluded(association string) bool {
	if req == nil {
		return false
	}
	_, ok := req.includes[association]
	return ok
}
