package servicep

import (
	"iter"
	"strings"
)

// Includable lets you define which inclusions are allowed, and which of those are included.
type Includable struct {
	allowed map[string]bool
	// map of included association, to bool whether it's the full path specified by user
	includes map[string]bool
}

func NewIncludable() *Includable {
	return &Includable{
		includes: make(map[string]bool),
	}
}

// Allow sets associations that are allowed to be included.
// Only enforced at add time, doesn't retroactively remove anything
func (i *Includable) Allow(allows ...string) *Includable {
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

func (i *Includable) IsAllowed(s string) bool {
	if i.allowed == nil {
		return true // no restrictions, all inclusions are allowed
	}
	_, ok := i.allowed[s]
	return ok
}

func (i *Includable) Include(associations ...string) *Includable {
	for _, assoc := range associations {
		components := strings.Split(assoc, ".")
		for j := range len(components) {
			path := strings.Join(components[:j+1], ".")
			if !i.IsAllowed(path) {
				continue
			}
			i.includes[path] = j == len(components)-1
		}
	}
	return i
}

// All returns an iterator over all user set includes
func (i *Includable) All() iter.Seq[string] {
	return func(yield func(string) bool) {
		for i, v := range i.includes {
			if !v { // only yield full paths
				continue
			}
			if !yield(i) {
				return // stop iteration if yield returns false
			}
		}
	}
}

func (i *Includable) IsIncluded(association string) bool {
	_, ok := i.includes[association]
	return ok
}
