package a2ui

import (
	"encoding/json"
	"fmt"
)

// DataModel is a surface's JSON state addressed by JSON Pointers.
type DataModel struct {
	root map[string]any
}

// NewDataModel returns an empty model.
func NewDataModel() *DataModel {
	return &DataModel{root: map[string]any{}}
}

// Get returns the value at pointer.
func (d *DataModel) Get(pointer string) (any, bool) {
	tokens, err := ParsePointer(pointer)
	if err != nil {
		return nil, false
	}
	return pointerGet(d.root, tokens)
}

// Set stores v at pointer with upsert semantics. Setting the root replaces
// the whole model and requires an object. A nil v deletes the key.
func (d *DataModel) Set(pointer string, v any) error {
	if v == nil {
		return d.Delete(pointer)
	}
	tokens, err := ParsePointer(pointer)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("a2ui: data model root must be an object, got %T", v)
		}
		d.root = obj
		return nil
	}
	root, err := pointerSet(d.root, tokens, v)
	if err != nil {
		return err
	}
	d.root = root.(map[string]any)
	return nil
}

// Delete removes the key at pointer; deleting the root empties the model.
func (d *DataModel) Delete(pointer string) error {
	tokens, err := ParsePointer(pointer)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		d.root = map[string]any{}
		return nil
	}
	d.root = pointerDelete(d.root, tokens).(map[string]any)
	return nil
}

// Root returns the live root object.
func (d *DataModel) Root() map[string]any {
	return d.root
}

// Snapshot returns a deep copy of the model.
func (d *DataModel) Snapshot() map[string]any {
	b, _ := json.Marshal(d.root)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}
