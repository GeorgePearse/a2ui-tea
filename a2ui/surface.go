package a2ui

import (
	"fmt"
	"strconv"
)

// RootID is the id of the component mounted under the canonical Surface.
const RootID = "root"

// ProtocolError is a surface-level error with a renderer-to-agent error code.
type ProtocolError struct {
	Code      string
	SurfaceID string
	Message   string
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("a2ui: %s (%s): %s", e.Code, e.SurfaceID, e.Message)
}

// Surface holds one surface's components and data model.
type Surface struct {
	ID            string
	CatalogID     string
	SendDataModel bool
	Components    map[string]*Component
	Data          *DataModel
	seq           int
}

// Store keeps every live surface in creation order.
type Store struct {
	surfaces map[string]*Surface
	seq      int
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{surfaces: map[string]*Surface{}}
}

// Apply updates the store with one message. Function call messages are not
// handled here; see Evaluator.
func (st *Store) Apply(m Message) error {
	if err := m.Validate(); err != nil {
		return err
	}
	switch {
	case m.CreateSurface != nil:
		return st.create(m.CreateSurface)
	case m.UpdateComponents != nil:
		s, err := st.target(m.UpdateComponents.SurfaceID)
		if err != nil {
			return err
		}
		s.update(m.UpdateComponents.Components)
	case m.UpdateDataModel != nil:
		s, err := st.target(m.UpdateDataModel.SurfaceID)
		if err != nil {
			return err
		}
		if err := s.Data.Set(m.UpdateDataModel.Path, m.UpdateDataModel.Value); err != nil {
			return &ProtocolError{Code: "VALIDATION_FAILED", SurfaceID: s.ID, Message: err.Error()}
		}
	case m.DeleteSurface != nil:
		delete(st.surfaces, m.DeleteSurface.SurfaceID)
	}
	return nil
}

func (st *Store) create(c *CreateSurface) error {
	if _, exists := st.surfaces[c.SurfaceID]; exists {
		return &ProtocolError{Code: "INTEGRITY_ERROR", SurfaceID: c.SurfaceID, Message: "surface already exists"}
	}
	st.seq++
	s := &Surface{
		ID:            c.SurfaceID,
		CatalogID:     c.CatalogID,
		SendDataModel: c.SendDataModel,
		Components:    map[string]*Component{},
		Data:          NewDataModel(),
		seq:           st.seq,
	}
	if c.DataModel != nil {
		s.Data.root = c.DataModel
	}
	s.update(c.Components)
	st.surfaces[c.SurfaceID] = s
	return nil
}

func (st *Store) target(id string) (*Surface, error) {
	s, ok := st.surfaces[id]
	if !ok {
		return nil, &ProtocolError{Code: "INTEGRITY_ERROR", SurfaceID: id, Message: "surface not found; send createSurface first"}
	}
	return s, nil
}

func (s *Surface) update(comps []Component) {
	for i := range comps {
		c := comps[i]
		s.Components[c.ID] = &c
	}
}

// Surface returns a surface by id.
func (st *Store) Surface(id string) (*Surface, bool) {
	s, ok := st.surfaces[id]
	return s, ok
}

// Surfaces returns the live surfaces in creation order.
func (st *Store) Surfaces() []*Surface {
	out := make([]*Surface, 0, len(st.surfaces))
	for _, s := range st.surfaces {
		out = append(out, s)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].seq < out[j-1].seq; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Len returns the number of live surfaces.
func (st *Store) Len() int {
	return len(st.surfaces)
}

// DataModels returns a snapshot of every surface's data model keyed by id,
// for the renderer data model sync payload.
func (st *Store) DataModels(onlySync bool) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, s := range st.surfaces {
		if onlySync && !s.SendDataModel {
			continue
		}
		out[s.ID] = s.Data.Snapshot()
	}
	return out
}

// Component returns a component by id.
func (s *Surface) Component(id string) (*Component, bool) {
	c, ok := s.Components[id]
	return c, ok
}

// Root returns the root node, if the root component has arrived.
func (s *Surface) Root() (Node, bool) {
	c, ok := s.Components[RootID]
	if !ok {
		return Node{}, false
	}
	return Node{Surface: s, Component: c, Scope: RootScope(s.Data), Key: RootID}, true
}

// Node is one rendered instance of a component: the component, the scope it
// evaluates in and a key unique within the surface (template expansions
// instantiate the same component under different keys).
type Node struct {
	Surface   *Surface
	Component *Component
	Scope     *Scope
	Key       string
}

// ID returns the component id.
func (n Node) ID() string {
	if n.Component == nil {
		return ""
	}
	return n.Component.ID
}

// Child returns the node for a child component referenced by id in the same
// scope. Missing components yield ok=false so callers can render a
// placeholder.
func (n Node) Child(id string) (Node, bool) {
	c, ok := n.Surface.Components[id]
	if !ok {
		return Node{Surface: n.Surface, Scope: n.Scope, Key: n.Key + "/" + id}, false
	}
	return Node{Surface: n.Surface, Component: c, Scope: n.Scope, Key: n.Key + "/" + id}, true
}

// Children expands the node's children property: static ids in the current
// scope, or one instance of the template per array item, each in its own
// collection scope.
func (n Node) Children(prop string) ([]Node, error) {
	raw, _ := n.Component.Prop(prop)
	list, err := ParseChildList(raw)
	if err != nil {
		return nil, err
	}
	if list.Template == nil {
		out := make([]Node, 0, len(list.IDs))
		for _, id := range list.IDs {
			child, _ := n.Child(id)
			out = append(out, child)
		}
		return out, nil
	}
	tpl, ok := n.Surface.Components[list.Template.ComponentID]
	if !ok {
		return nil, nil
	}
	base := n.Scope.Absolute(list.Template.Path)
	items, _ := n.Scope.Data.Get(base)
	arr, ok := items.([]any)
	if !ok {
		return nil, nil
	}
	out := make([]Node, 0, len(arr))
	for i := range arr {
		idx := strconv.Itoa(i)
		out = append(out, Node{
			Surface:   n.Surface,
			Component: tpl,
			Scope:     n.Scope.Item(base+"/"+idx, i),
			Key:       n.Key + "/" + tpl.ID + "[" + idx + "]",
		})
	}
	return out, nil
}
