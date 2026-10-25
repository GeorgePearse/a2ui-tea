package a2ui

import "strings"

// Scope resolves paths against a data model. The root scope resolves only
// absolute pointers; a collection scope created by a template also resolves
// relative paths against its item.
type Scope struct {
	Data   *DataModel
	Base   string
	Index  int
	parent *Scope
	inList bool
}

// RootScope returns the scope in which top-level components evaluate.
func RootScope(d *DataModel) *Scope {
	return &Scope{Data: d}
}

// Item returns a collection scope for the item at base (an absolute pointer)
// with the given iteration index.
func (s *Scope) Item(base string, index int) *Scope {
	return &Scope{Data: s.Data, Base: base, Index: index, parent: s, inList: true}
}

// Absolute converts a path to an absolute pointer. Relative paths are joined
// to the scope's base; in the root scope they are treated as absolute.
func (s *Scope) Absolute(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	if !s.inList {
		return "/" + path
	}
	return JoinPointer(s.Base, path)
}

// Get reads the value at path.
func (s *Scope) Get(path string) (any, bool) {
	if s == nil || s.Data == nil {
		return nil, false
	}
	return s.Data.Get(s.Absolute(path))
}

// Set writes v at path.
func (s *Scope) Set(path string, v any) error {
	return s.Data.Set(s.Absolute(path), v)
}

// ItemIndex returns the innermost template iteration index.
func (s *Scope) ItemIndex() (int, bool) {
	for c := s; c != nil; c = c.parent {
		if c.inList {
			return c.Index, true
		}
	}
	return 0, false
}

// InTemplate reports whether the scope is inside a template expansion.
func (s *Scope) InTemplate() bool {
	_, ok := s.ItemIndex()
	return ok
}
