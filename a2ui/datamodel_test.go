package a2ui

import (
	"reflect"
	"testing"
)

func TestDataModelGetSet(t *testing.T) {
	d := NewDataModel()
	if err := d.Set("/", map[string]any{"user": map[string]any{"name": "Alice"}, "items": []any{"Apple", "Banana"}}); err != nil {
		t.Fatal(err)
	}
	cases := map[string]any{
		"/user/name": "Alice",
		"/items/0":   "Apple",
		"/items/1":   "Banana",
	}
	for p, want := range cases {
		got, ok := d.Get(p)
		if !ok || got != want {
			t.Errorf("Get(%q) = %v, %v; want %v", p, got, ok, want)
		}
	}
	if _, ok := d.Get("/items/2"); ok {
		t.Error("index past the end should be missing")
	}
	if _, ok := d.Get("/nope/deeper"); ok {
		t.Error("missing path should be missing")
	}
	if _, ok := d.Get("relative"); ok {
		t.Error("pointer without leading slash is invalid")
	}
}

func TestDataModelUpsert(t *testing.T) {
	d := NewDataModel()
	if err := d.Set("/user/email", "a@b.c"); err != nil {
		t.Fatal(err)
	}
	if err := d.Set("/user/name", "Bob"); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"user": map[string]any{"email": "a@b.c", "name": "Bob"}}
	if !reflect.DeepEqual(d.Root(), want) {
		t.Errorf("root = %#v, want %#v", d.Root(), want)
	}
	if err := d.Set("/user/email", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Get("/user/email"); ok {
		t.Error("nil value should delete the key")
	}
	if err := d.Set("/", "not an object"); err == nil {
		t.Error("root must be an object")
	}
}

func TestDataModelArrays(t *testing.T) {
	d := NewDataModel()
	_ = d.Set("/list", []any{map[string]any{"n": 1.0}})
	if err := d.Set("/list/1", map[string]any{"n": 2.0}); err != nil {
		t.Fatal(err)
	}
	if err := d.Set("/list/-", map[string]any{"n": 3.0}); err != nil {
		t.Fatal(err)
	}
	if err := d.Set("/list/0/n", 10.0); err != nil {
		t.Fatal(err)
	}
	got, _ := d.Get("/list")
	if len(got.([]any)) != 3 {
		t.Fatalf("len = %d, want 3", len(got.([]any)))
	}
	if v, _ := d.Get("/list/0/n"); v != 10.0 {
		t.Errorf("/list/0/n = %v", v)
	}
	if err := d.Set("/list/9", 1); err == nil {
		t.Error("index beyond len+1 must fail")
	}
	if err := d.Delete("/list/1"); err != nil {
		t.Fatal(err)
	}
	got, _ = d.Get("/list")
	if len(got.([]any)) != 2 {
		t.Errorf("after delete len = %d, want 2", len(got.([]any)))
	}
}

func TestPointerEscapes(t *testing.T) {
	d := NewDataModel()
	_ = d.Set("/", map[string]any{"a/b": "slash", "m~n": "tilde"})
	if v, _ := d.Get("/a~1b"); v != "slash" {
		t.Errorf("~1 escape: %v", v)
	}
	if v, _ := d.Get("/m~0n"); v != "tilde" {
		t.Errorf("~0 escape: %v", v)
	}
}

func TestScopeResolution(t *testing.T) {
	d := NewDataModel()
	_ = d.Set("/", map[string]any{
		"company":   "Acme",
		"employees": []any{map[string]any{"name": "Alice"}, map[string]any{"name": "Bob"}},
	})
	root := RootScope(d)
	if v, _ := root.Get("/company"); v != "Acme" {
		t.Errorf("absolute in root: %v", v)
	}
	if v, _ := root.Get("company"); v != "Acme" {
		t.Errorf("relative in root resolves as absolute: %v", v)
	}
	item := root.Item("/employees/1", 1)
	if v, _ := item.Get("name"); v != "Bob" {
		t.Errorf("relative in item: %v", v)
	}
	if v, _ := item.Get("/company"); v != "Acme" {
		t.Errorf("absolute in item: %v", v)
	}
	if i, ok := item.ItemIndex(); !ok || i != 1 {
		t.Errorf("index = %d, %v", i, ok)
	}
	if _, ok := root.ItemIndex(); ok {
		t.Error("root scope has no index")
	}
	if err := item.Set("name", "Bobby"); err != nil {
		t.Fatal(err)
	}
	if v, _ := d.Get("/employees/1/name"); v != "Bobby" {
		t.Errorf("write through item scope: %v", v)
	}
}
