package a2ui

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func mustParse(t *testing.T, src string) Message {
	t.Helper()
	m, err := ParseMessage([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestStoreLifecycle(t *testing.T) {
	st := NewStore()
	if err := st.Apply(mustParse(t, `{"version":"v1.0","updateDataModel":{"surfaceId":"s","value":{}}}`)); err == nil {
		t.Error("update before create must fail")
	} else {
		var pe *ProtocolError
		if !errors.As(err, &pe) || pe.Code != "INTEGRITY_ERROR" {
			t.Errorf("want INTEGRITY_ERROR, got %v", err)
		}
	}
	create := `{"version":"v1.0","createSurface":{"surfaceId":"s","catalogId":"c","sendDataModel":true,"components":[{"id":"root","component":"Text","text":{"@path":"/msg"}}],"dataModel":{"msg":"hi"}}}`
	if err := st.Apply(mustParse(t, create)); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(mustParse(t, create)); err == nil {
		t.Error("duplicate create must fail")
	}
	s, ok := st.Surface("s")
	if !ok || s.CatalogID != "c" || !s.SendDataModel {
		t.Fatalf("surface: %+v", s)
	}
	root, ok := s.Root()
	if !ok || root.ID() != "root" {
		t.Fatal("root missing")
	}
	e := NewEvaluator()
	if got := e.String(root.Component.Props["text"], root.Scope); got != "hi" {
		t.Errorf("bound text = %q", got)
	}
	if err := st.Apply(mustParse(t, `{"version":"v1.0","updateDataModel":{"surfaceId":"s","path":"/msg","value":"bye"}}`)); err != nil {
		t.Fatal(err)
	}
	if got := e.String(root.Component.Props["text"], root.Scope); got != "bye" {
		t.Errorf("after update = %q", got)
	}
	if err := st.Apply(mustParse(t, `{"version":"v1.0","updateComponents":{"surfaceId":"s","components":[{"id":"root","component":"Text","text":"replaced"}]}}`)); err != nil {
		t.Fatal(err)
	}
	root, _ = s.Root()
	if got := e.String(root.Component.Props["text"], root.Scope); got != "replaced" {
		t.Errorf("after component update = %q", got)
	}
	_ = st.Apply(mustParse(t, `{"version":"v1.0","createSurface":{"surfaceId":"t"}}`))
	if ids := st.Surfaces(); len(ids) != 2 || ids[0].ID != "s" || ids[1].ID != "t" {
		t.Errorf("order: %v", ids)
	}
	if err := st.Apply(mustParse(t, `{"version":"v1.0","deleteSurface":{"surfaceId":"s"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(mustParse(t, `{"version":"v1.0","deleteSurface":{"surfaceId":"never"}}`)); err != nil {
		t.Error("deleting an unknown surface is a no-op")
	}
	if st.Len() != 1 {
		t.Errorf("len = %d", st.Len())
	}
	models := st.DataModels(false)
	if _, ok := models["t"]; !ok || len(models) != 1 {
		t.Errorf("data models: %v", models)
	}
}

func TestTemplateExpansion(t *testing.T) {
	st := NewStore()
	msgs, err := ParseMessages([]byte(`[
	 {"version":"v1.0","createSurface":{"surfaceId":"s","catalogId":"c"}},
	 {"version":"v1.0","updateComponents":{"surfaceId":"s","components":[
	   {"id":"root","component":"Column","children":{"path":"/items","componentId":"row"}},
	   {"id":"row","component":"Row","children":["name","idx"]},
	   {"id":"name","component":"Text","text":{"@path":"name"}},
	   {"id":"idx","component":"Text","text":{"@call":"formatString","args":{"value":"#${@index(offset: 1)} of ${/total}"}}}
	 ]}},
	 {"version":"v1.0","updateDataModel":{"surfaceId":"s","value":{"total":2,"items":[{"name":"Apple"},{"name":"Pear"}]}}}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range msgs {
		if err := st.Apply(m); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := st.Surface("s")
	root, _ := s.Root()
	rows, err := root.Children("children")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	e := NewEvaluator()
	want := []struct{ key, name, idx string }{
		{"root/row[0]", "Apple", "#1 of 2"},
		{"root/row[1]", "Pear", "#2 of 2"},
	}
	for i, row := range rows {
		if row.Key != want[i].key {
			t.Errorf("key = %q, want %q", row.Key, want[i].key)
		}
		cells, _ := row.Children("children")
		if len(cells) != 2 {
			t.Fatalf("cells = %d", len(cells))
		}
		if got := e.String(cells[0].Component.Props["text"], cells[0].Scope); got != want[i].name {
			t.Errorf("row %d name = %q", i, got)
		}
		if got := e.String(cells[1].Component.Props["text"], cells[1].Scope); got != want[i].idx {
			t.Errorf("row %d idx = %q", i, got)
		}
	}
	_ = st.Apply(mustParse(t, `{"version":"v1.0","updateDataModel":{"surfaceId":"s","path":"/items/2","value":{"name":"Fig"}}}`))
	rows, _ = root.Children("children")
	if len(rows) != 3 {
		t.Errorf("after append rows = %d", len(rows))
	}
	missing, ok := root.Child("ghost")
	if ok || missing.Component != nil || missing.Key != "root/ghost" {
		t.Errorf("missing child: %+v %v", missing, ok)
	}
}

func TestBuildAction(t *testing.T) {
	st := NewStore()
	_ = st.Apply(mustParse(t, `{"version":"v1.0","createSurface":{"surfaceId":"booking","components":[
	  {"id":"root","component":"Button","child":"l","action":{"event":{"name":"submit_reservation","userMessage":{"@call":"formatString","args":{"value":"Booked for ${/partySize}"}},"context":{"time":{"@path":"/reservationTime"},"size":{"@path":"/partySize"},"static":"x"}}}}
	],"dataModel":{"reservationTime":"7:00 PM","partySize":4}}}`))
	s, _ := st.Surface("booking")
	root, _ := s.Root()
	ev, call := ActionOf(root.Component)
	if ev == nil || call != nil || ev.Name != "submit_reservation" {
		t.Fatalf("ActionOf: %+v %v", ev, call)
	}
	e := NewEvaluator()
	out := e.BuildAction(root, ev, time.Date(2026, 2, 25, 10, 40, 0, 0, time.UTC))
	b, _ := json.Marshal(out)
	want := `{"version":"v1.0","action":{"name":"submit_reservation","userMessage":"Booked for 4","surfaceId":"booking","sourceComponentId":"root","timestamp":"2026-02-25T10:40:00Z","context":{"size":4,"static":"x","time":"7:00 PM"}}}`
	if string(b) != want {
		t.Errorf("action json:\n got %s\nwant %s", b, want)
	}
	fc := &Component{ID: "x", Type: "Button", Props: map[string]any{"action": map[string]any{"functionCall": map[string]any{"@call": "openUrl", "args": map[string]any{"url": "https://x"}}}}}
	if ev, call := ActionOf(fc); ev != nil || call == nil {
		t.Errorf("function action: %+v %v", ev, call)
	}
}

func TestHandleCall(t *testing.T) {
	st := NewStore()
	_ = st.Apply(mustParse(t, `{"version":"v1.0","createSurface":{"surfaceId":"s","dataModel":{"n":1234.5}}}`))
	e := NewEvaluator()
	m := mustParse(t, `{"version":"v1.0","callRendererFunction":{"surfaceId":"s","functionCallId":"c1","callFunction":{"@call":"formatNumber","catalogId":"x","args":{"value":{"@path":"/n"},"decimals":1}}}}`)
	out := e.HandleCall(st, m.CallRendererFunction)
	if out.RendererFunctionResponse == nil || out.RendererFunctionResponse.Value != "1,234.5" || out.RendererFunctionResponse.FunctionCallID != "c1" {
		t.Errorf("response: %+v", out.RendererFunctionResponse)
	}
	bad := mustParse(t, `{"version":"v1.0","callRendererFunction":{"functionCallId":"c2","callFunction":{"@call":"missing","catalogId":"x"}}}`)
	out = e.HandleCall(st, bad.CallRendererFunction)
	if out.RendererFunctionResponse.Error == nil {
		t.Error("unknown function must produce an error response")
	}
	em := ErrorMessage(&ProtocolError{Code: "INTEGRITY_ERROR", SurfaceID: "s", Message: "dup"}, "")
	if em.Error.Code != "INTEGRITY_ERROR" || em.Error.SurfaceID != "s" {
		t.Errorf("error message: %+v", em.Error)
	}
}
