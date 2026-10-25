package a2ui

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Component is one entry of a surface's flat component list. Everything
// except the envelope fields is kept verbatim in Props so catalogs can define
// any property set.
type Component struct {
	ID        string
	Type      string
	CatalogID string
	Props     map[string]any
}

// UnmarshalJSON decodes the flat v1.0 component object.
func (c *Component) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Props = make(map[string]any, len(raw))
	for k, v := range raw {
		switch k {
		case "id":
			c.ID, _ = v.(string)
		case "component":
			c.Type, _ = v.(string)
		case "catalogId":
			c.CatalogID, _ = v.(string)
		default:
			c.Props[k] = v
		}
	}
	return nil
}

// MarshalJSON encodes the component back to its flat wire form.
func (c Component) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(c.Props)+3)
	for k, v := range c.Props {
		out[k] = v
	}
	out["id"] = c.ID
	out["component"] = c.Type
	if c.CatalogID != "" {
		out["catalogId"] = c.CatalogID
	}
	return json.Marshal(out)
}

func (c Component) validate() error {
	if c.ID == "" {
		return errors.New("a2ui: component without id")
	}
	if c.Type == "" {
		return fmt.Errorf("a2ui: component %q without component type", c.ID)
	}
	if c.Type == "Surface" {
		return fmt.Errorf("a2ui: component %q uses the reserved type Surface", c.ID)
	}
	return nil
}

// Prop returns a raw property.
func (c *Component) Prop(name string) (any, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Props[name]
	return v, ok
}

// StringProp returns a property that is a literal string, or def.
func (c *Component) StringProp(name, def string) string {
	if v, ok := c.Prop(name); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

// NumberProp returns a property that is a literal number, or def.
func (c *Component) NumberProp(name string, def float64) float64 {
	if v, ok := c.Prop(name); ok {
		if f, ok := ToNumber(v); ok {
			return f
		}
	}
	return def
}

// BoolProp returns a property that is a literal boolean, or def.
func (c *Component) BoolProp(name string, def bool) bool {
	if v, ok := c.Prop(name); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

// Weight is the component's flex weight inside a Row or Column (0 if unset).
func (c *Component) Weight() float64 {
	return c.NumberProp("weight", 0)
}

// ChildList is the resolved form of a container's children property: either
// a static list of IDs or a template bound to a data model array.
type ChildList struct {
	IDs      []string
	Template *Template
}

// Template instantiates ComponentID once per item of the array at Path.
type Template struct {
	ComponentID string
	Path        string
}

// ParseChildList interprets a raw children value.
func ParseChildList(v any) (ChildList, error) {
	switch t := v.(type) {
	case nil:
		return ChildList{}, nil
	case []any:
		ids := make([]string, 0, len(t))
		for _, e := range t {
			s, ok := e.(string)
			if !ok {
				return ChildList{}, fmt.Errorf("a2ui: child id must be a string, got %T", e)
			}
			ids = append(ids, s)
		}
		return ChildList{IDs: ids}, nil
	case []string:
		return ChildList{IDs: t}, nil
	case map[string]any:
		id, _ := t["componentId"].(string)
		path, _ := t["path"].(string)
		if id == "" || path == "" {
			return ChildList{}, errors.New("a2ui: template children need componentId and path")
		}
		return ChildList{Template: &Template{ComponentID: id, Path: path}}, nil
	}
	return ChildList{}, fmt.Errorf("a2ui: children must be an array or template, got %T", v)
}
