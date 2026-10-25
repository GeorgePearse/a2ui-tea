package a2ui

import (
	"strings"
	"testing"
)

func TestParseEveryMessageType(t *testing.T) {
	cases := []struct {
		name, json, kind string
	}{
		{"create", `{"version":"v1.0","createSurface":{"surfaceId":"s","catalogId":"c"}}`, "createSurface"},
		{"create inline", `{"version":"v1.0","createSurface":{"surfaceId":"s","components":[{"id":"root","component":"Text","text":"hi"}],"dataModel":{"a":1}}}`, "createSurface"},
		{"update components", `{"version":"v1.0","updateComponents":{"surfaceId":"s","components":[{"id":"root","component":"Column","children":["a"]}]}}`, "updateComponents"},
		{"update data", `{"version":"v1.0","updateDataModel":{"surfaceId":"s","path":"/a","value":{"b":true}}}`, "updateDataModel"},
		{"update data null", `{"version":"v1.0","updateDataModel":{"surfaceId":"s","path":"/a","value":null}}`, "updateDataModel"},
		{"delete", `{"version":"v1.0","deleteSurface":{"surfaceId":"s"}}`, "deleteSurface"},
		{"call", `{"version":"v1.0","callRendererFunction":{"surfaceId":"s","functionCallId":"c1","callFunction":{"@call":"formatNumber","catalogId":"x","args":{"value":1}}}}`, "callRendererFunction"},
		{"agent response", `{"version":"v1.0","agentFunctionResponse":{"functionCallId":"c1","value":42}}`, "agentFunctionResponse"},
		{"v0.9.1 envelope", `{"version":"v0.9.1","updateDataModel":{"surfaceId":"s","path":"/x","value":"y"}}`, "updateDataModel"},
		{"v0.9 envelope", `{"version":"v0.9","createSurface":{"surfaceId":"s","catalogId":"c","theme":{"primaryColor":"#000"}}}`, "createSurface"},
		{"no version", `{"deleteSurface":{"surfaceId":"s"}}`, "deleteSurface"},
	}
	for _, c := range cases {
		m, err := ParseMessage([]byte(c.json))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if m.Kind() != c.kind {
			t.Errorf("%s: kind = %q, want %q", c.name, m.Kind(), c.kind)
		}
	}
}

func TestParseBadInput(t *testing.T) {
	cases := map[string]string{
		"not json":          `{`,
		"empty object":      `{}`,
		"two messages":      `{"createSurface":{"surfaceId":"a"},"deleteSurface":{"surfaceId":"a"}}`,
		"unknown key":       `{"version":"v1.0","renderThing":{"surfaceId":"a"}}`,
		"bad version":       `{"version":"v2.7","deleteSurface":{"surfaceId":"a"}}`,
		"missing surfaceId": `{"version":"v1.0","updateComponents":{"components":[{"id":"root","component":"Text"}]}}`,
		"no components":     `{"version":"v1.0","updateComponents":{"surfaceId":"s","components":[]}}`,
		"component no id":   `{"version":"v1.0","updateComponents":{"surfaceId":"s","components":[{"component":"Text"}]}}`,
		"component no type": `{"version":"v1.0","updateComponents":{"surfaceId":"s","components":[{"id":"x"}]}}`,
		"reserved Surface":  `{"version":"v1.0","updateComponents":{"surfaceId":"s","components":[{"id":"x","component":"Surface"}]}}`,
		"call without id":   `{"version":"v1.0","callRendererFunction":{"callFunction":{"@call":"not"}}}`,
	}
	for name, in := range cases {
		if _, err := ParseMessage([]byte(in)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestParseMessagesShapes(t *testing.T) {
	single := `{"version":"v1.0","deleteSurface":{"surfaceId":"s"}}`
	array := `[` + single + `,` + single + `]`
	wrapped := `{"name":"Example","description":"d","messages":` + array + `}`
	for name, in := range map[string]int{single: 1, array: 2, wrapped: 2, "  \n": 0} {
		msgs, err := ParseMessages([]byte(name))
		if err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		if len(msgs) != in {
			t.Errorf("%q: got %d messages, want %d", name, len(msgs), in)
		}
	}
	if _, err := ParseMessages([]byte(`[{"version":"v1.0","deleteSurface":{"surfaceId":"s"}},{"bad":1}]`)); err == nil {
		t.Error("array with a bad element must fail")
	}
}

func TestReadMessages(t *testing.T) {
	in := strings.Join([]string{
		`{"version":"v1.0","createSurface":{"surfaceId":"s","catalogId":"c"}}`,
		``,
		`[{"version":"v1.0","updateDataModel":{"surfaceId":"s","value":{"a":1}}},{"version":"v1.0",`,
		` "deleteSurface":{"surfaceId":"s"}}]`,
		`{"messages":[{"version":"v1.0","deleteSurface":{"surfaceId":"t"}}]}`,
	}, "\n")
	var kinds []string
	err := ReadMessages(strings.NewReader(in), func(m Message) error {
		kinds = append(kinds, m.Kind())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ReadMessages(strings.NewReader(`{"version":"v1.0","deleteSurface":{"surfaceId":"s"}} {bad`), func(Message) error { return nil }); err == nil {
		t.Error("truncated input must fail")
	}
	want := "createSurface,updateDataModel,deleteSurface,deleteSurface"
	if got := strings.Join(kinds, ","); got != want {
		t.Errorf("kinds = %s, want %s", got, want)
	}
}

func TestComponentRoundTrip(t *testing.T) {
	m, err := ParseMessage([]byte(`{"version":"v1.0","updateComponents":{"surfaceId":"s","components":[{"id":"b","component":"Button","catalogId":"custom","child":"t","weight":2,"action":{"event":{"name":"go"}}}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := m.UpdateComponents.Components[0]
	if c.ID != "b" || c.Type != "Button" || c.CatalogID != "custom" {
		t.Errorf("envelope fields: %+v", c)
	}
	if c.StringProp("child", "") != "t" || c.Weight() != 2 {
		t.Errorf("props: %+v", c.Props)
	}
	if _, ok := c.Props["id"]; ok {
		t.Error("id must not leak into Props")
	}
	out, err := c.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"id":"b"`, `"component":"Button"`, `"catalogId":"custom"`, `"child":"t"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("marshal missing %s: %s", want, out)
		}
	}
}

func TestParseChildList(t *testing.T) {
	if cl, err := ParseChildList([]any{"a", "b"}); err != nil || len(cl.IDs) != 2 {
		t.Errorf("static: %v %v", cl, err)
	}
	if cl, err := ParseChildList(map[string]any{"componentId": "tpl", "path": "/items"}); err != nil || cl.Template == nil || cl.Template.Path != "/items" {
		t.Errorf("template: %v %v", cl, err)
	}
	if _, err := ParseChildList(map[string]any{"componentId": "tpl"}); err == nil {
		t.Error("template without path must fail")
	}
	if _, err := ParseChildList([]any{1}); err == nil {
		t.Error("non-string id must fail")
	}
	if _, err := ParseChildList("x"); err == nil {
		t.Error("string children must fail")
	}
}
