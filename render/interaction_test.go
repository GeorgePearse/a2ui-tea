package render

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
	"github.com/GeorgePearse/a2ui-tea/catalog"
)

func newTestModel(t *testing.T, src string) (*Model, *[]a2ui.Outbound) {
	t.Helper()
	var out []a2ui.Outbound
	m := New(Options{HideHelp: true, Width: 60, Height: 30, Outbound: func(o a2ui.Outbound) { out = append(out, o) }})
	m.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	msgs, err := a2ui.ParseMessages([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ApplyAll(msgs); err != nil {
		t.Fatal(err)
	}
	return m, &out
}

func press(m *Model, keys ...string) tea.Cmd {
	var last tea.Cmd
	for _, k := range keys {
		var key tea.KeyPressMsg
		switch k {
		case "tab":
			key = tea.KeyPressMsg{Code: tea.KeyTab}
		case "shift+tab":
			key = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
		case "enter":
			key = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "space":
			key = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "left":
			key = tea.KeyPressMsg{Code: tea.KeyLeft}
		case "right":
			key = tea.KeyPressMsg{Code: tea.KeyRight}
		case "down":
			key = tea.KeyPressMsg{Code: tea.KeyDown}
		case "esc":
			key = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "backspace":
			key = tea.KeyPressMsg{Code: tea.KeyBackspace}
		default:
			r := []rune(k)[0]
			key = tea.KeyPressMsg{Code: r, Text: k}
		}
		_, last = m.Update(key)
	}
	return last
}

func view(m *Model) string {
	return ansi.Strip(m.View().Content)
}

const formSurface = `[
 {"version":"v1.0","createSurface":{"surfaceId":"form","catalogId":"` + catalog.BasicCatalogID + `","sendDataModel":true}},
 {"version":"v1.0","updateComponents":{"surfaceId":"form","components":[
  {"id":"root","component":"Column","children":["name","agree","size","pick","submit"]},
  {"id":"name","component":"TextField","label":"Name","value":{"@path":"/form/name"},"checks":[{"condition":{"@call":"required","args":{"value":{"@path":"/form/name"}}},"message":"Name is required"}]},
  {"id":"agree","component":"CheckBox","label":"Agree","value":{"@path":"/form/agree"}},
  {"id":"size","component":"Slider","label":"Size","min":0,"max":10,"steps":10,"value":{"@path":"/form/size"}},
  {"id":"pick","component":"ChoicePicker","label":"Pick","variant":"multipleSelection","options":[{"label":"A","value":"a"},{"label":"B","value":"b"}],"value":{"@path":"/form/pick"}},
  {"id":"submit","component":"Button","child":"submit_text","action":{"event":{"name":"submit","context":{"name":{"@path":"/form/name"},"agree":{"@path":"/form/agree"}}}},
   "checks":[{"condition":{"@path":"/form/agree"},"message":"Agree first"}]},
  {"id":"submit_text","component":"Text","text":"Submit"}
 ]}},
 {"version":"v1.0","updateDataModel":{"surfaceId":"form","value":{"form":{"name":"","agree":false,"size":5,"pick":[]}}}}
]`

func TestTypingUpdatesDataModel(t *testing.T) {
	m, _ := newTestModel(t, formSurface)
	if m.focus != "root/name" {
		t.Fatalf("first focusable should be the text field, got %q", m.focus)
	}
	press(m, "A", "d", "a")
	s, _ := m.store.Surface("form")
	if v, _ := s.Data.Get("/form/name"); v != "Ada" {
		t.Errorf("/form/name = %v", v)
	}
	press(m, "backspace")
	if v, _ := s.Data.Get("/form/name"); v != "Ad" {
		t.Errorf("after backspace /form/name = %v", v)
	}
	if !strings.Contains(view(m), "Ad") {
		t.Errorf("view should show the typed value:\n%s", view(m))
	}
	_ = m.Apply(mustMsg(t, `{"version":"v1.0","updateDataModel":{"surfaceId":"form","path":"/form/name","value":"Grace"}}`))
	if !strings.Contains(view(m), "Grace") {
		t.Errorf("agent update should replace the field value:\n%s", view(m))
	}
}

func TestButtonPressEmitsAction(t *testing.T) {
	m, out := newTestModel(t, formSurface)
	press(m, "tab", "tab", "tab", "tab")
	if m.focus != "root/submit" {
		t.Fatalf("focus = %q", m.focus)
	}
	press(m, "enter")
	if len(*out) != 0 {
		t.Fatalf("disabled button must not dispatch: %+v", *out)
	}
	if !strings.Contains(view(m), "Agree first") {
		t.Errorf("disabled reason should show:\n%s", view(m))
	}
	press(m, "shift+tab", "shift+tab", "shift+tab", "space")
	s, _ := m.store.Surface("form")
	if v, _ := s.Data.Get("/form/agree"); v != true {
		t.Fatalf("checkbox should toggle, got %v", v)
	}
	press(m, "shift+tab", "J", "o")
	press(m, "tab", "tab", "tab", "tab", "enter")
	if len(*out) != 1 || (*out)[0].Action == nil {
		t.Fatalf("expected one action, got %+v", *out)
	}
	got, _ := json.Marshal((*out)[0])
	want := `{"version":"v1.0","action":{"name":"submit","surfaceId":"form","sourceComponentId":"submit","timestamp":"2026-01-02T03:04:05Z","context":{"agree":true,"name":"Jo"}}}`
	if string(got) != want {
		t.Errorf("action:\n got %s\nwant %s", got, want)
	}
}

func TestSliderAndChoicePicker(t *testing.T) {
	m, _ := newTestModel(t, formSurface)
	press(m, "tab", "tab", "right", "right", "left")
	s, _ := m.store.Surface("form")
	if v, _ := s.Data.Get("/form/size"); v != 6.0 {
		t.Errorf("slider = %v, want 6", v)
	}
	press(m, "tab", "space", "down", "space")
	if v, _ := s.Data.Get("/form/pick"); !equalStrings(v, []string{"a", "b"}) {
		t.Errorf("pick = %v", v)
	}
	press(m, "space")
	if v, _ := s.Data.Get("/form/pick"); !equalStrings(v, []string{"a"}) {
		t.Errorf("pick after untoggle = %v", v)
	}
	if !strings.Contains(view(m), "[x] A") || !strings.Contains(view(m), "[ ] B") {
		t.Errorf("view:\n%s", view(m))
	}
}

func TestTabsAndModal(t *testing.T) {
	m, out := newTestModel(t, `[
 {"version":"v1.0","createSurface":{"surfaceId":"s","components":[
  {"id":"root","component":"Column","children":["tabs","modal"]},
  {"id":"tabs","component":"Tabs","tabs":[{"title":"One","child":"t1"},{"title":"Two","child":"t2"}]},
  {"id":"t1","component":"Text","text":"first tab body"},
  {"id":"t2","component":"Text","text":"second tab body"},
  {"id":"modal","component":"Modal","trigger":"open","content":"dialog"},
  {"id":"open","component":"Button","child":"open_t","action":{"event":{"name":"opened"}}},
  {"id":"open_t","component":"Text","text":"Open"},
  {"id":"dialog","component":"Column","children":["dialog_t","close"]},
  {"id":"dialog_t","component":"Text","text":"inside the modal"},
  {"id":"close","component":"Button","child":"close_t","action":{"functionCall":{"@call":"not","args":{"value":true}}}},
  {"id":"close_t","component":"Text","text":"Nothing"}
 ]}}]`)
	if v := view(m); !strings.Contains(v, "first tab body") || strings.Contains(v, "second tab body") {
		t.Errorf("tab one should be visible:\n%s", v)
	}
	press(m, "right")
	if v := view(m); !strings.Contains(v, "second tab body") {
		t.Errorf("tab two should be visible after right:\n%s", v)
	}
	press(m, "tab", "enter")
	if m.modal != "root/modal" {
		t.Fatalf("modal should be open, got %q (ring %v)", m.modal, m.ring)
	}
	if len(*out) != 1 || (*out)[0].Action.Name != "opened" {
		t.Errorf("trigger event should still dispatch: %+v", *out)
	}
	v := view(m)
	if !strings.Contains(v, "inside the modal") || !strings.Contains(v, "esc to close") {
		t.Errorf("overlay missing:\n%s", v)
	}
	if m.focus != "root/modal/dialog/close" {
		t.Errorf("focus should move into the modal, got %q", m.focus)
	}
	press(m, "enter")
	if len(*out) != 1 {
		t.Errorf("local function call must not reach the agent: %+v", *out)
	}
	press(m, "esc")
	if m.modal != "" || strings.Contains(view(m), "inside the modal") {
		t.Errorf("esc should close the modal")
	}
	if m.focus != "root/modal/open" {
		t.Errorf("focus should return to the trigger, got %q", m.focus)
	}
}

func TestRuntimeRestructure(t *testing.T) {
	m, _ := newTestModel(t, `[
 {"version":"v1.0","createSurface":{"surfaceId":"s","components":[
  {"id":"root","component":"Column","children":["a"]},
  {"id":"a","component":"Text","text":"alpha"}
 ]}}]`)
	if v := view(m); !strings.Contains(v, "alpha") {
		t.Fatalf("initial:\n%s", v)
	}
	_ = m.Apply(mustMsg(t, `{"version":"v1.0","updateComponents":{"surfaceId":"s","components":[
  {"id":"root","component":"Row","children":["a","b"]},
  {"id":"b","component":"Button","child":"a","action":{"event":{"name":"go"}}}]}}`))
	v := view(m)
	if !strings.Contains(v, "alpha") || !strings.Contains(v, "[ alpha ]") {
		t.Errorf("restructured view:\n%s", v)
	}
	if m.focus != "root/b" {
		t.Errorf("new button should receive focus, got %q", m.focus)
	}
	if err := m.Apply(mustMsg(t, `{"version":"v1.0","updateComponents":{"surfaceId":"nope","components":[{"id":"root","component":"Text","text":"x"}]}}`)); err == nil {
		t.Error("unknown surface must error")
	}
	if !strings.Contains(m.status, "INTEGRITY_ERROR") {
		t.Errorf("status should show the error, got %q", m.status)
	}
}

func TestDeleteLastSurfaceQuits(t *testing.T) {
	m, out := newTestModel(t, `{"version":"v1.0","createSurface":{"surfaceId":"s","components":[{"id":"root","component":"Text","text":"hi"}]}}`)
	m.opts.QuitOnEmpty = true
	_, cmd := m.Update(MessagesMsg{mustMsg(t, `{"version":"v1.0","deleteSurface":{"surfaceId":"s"}}`)})
	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
	if len(*out) != 0 {
		t.Errorf("no outbound expected: %+v", *out)
	}
}

func TestRendererFunctionCall(t *testing.T) {
	m, out := newTestModel(t, `[
 {"version":"v1.0","createSurface":{"surfaceId":"s","dataModel":{"n":2}}},
 {"version":"v1.0","callRendererFunction":{"surfaceId":"s","functionCallId":"c9","callFunction":{"@call":"formatString","catalogId":"x","args":{"value":"n=${/n}"}}}}
]`)
	if len(*out) != 1 || (*out)[0].RendererFunctionResponse == nil || (*out)[0].RendererFunctionResponse.Value != "n=2" {
		t.Errorf("response: %+v", *out)
	}
	_ = m
}

func TestCustomComponent(t *testing.T) {
	reg := catalog.Basic()
	reg.Register("Gauge", gauge{})
	m := New(Options{HideHelp: true, Registry: reg, Width: 40, Height: 10})
	msgs, _ := a2ui.ParseMessages([]byte(`{"version":"v1.0","createSurface":{"surfaceId":"s","catalogId":"example.com:gauges","components":[
	  {"id":"root","component":"Gauge","label":"CPU","value":{"@path":"/cpu"}}],"dataModel":{"cpu":0.5}}}`))
	_ = m.ApplyAll(msgs)
	if v := view(m); !strings.Contains(v, "CPU ●●●●●○○○○○") {
		t.Errorf("custom component:\n%s", v)
	}
}

type gauge struct{}

func (gauge) Render(f catalog.Frame, n a2ui.Node, width int) string {
	v, _ := f.Eval().Number(n.Component.Props["value"], n.Scope)
	filled := int(v * 10)
	return n.Component.StringProp("label", "") + " " + strings.Repeat("●", filled) + strings.Repeat("○", 10-filled)
}

func mustMsg(t *testing.T, src string) a2ui.Message {
	t.Helper()
	m, err := a2ui.ParseMessage([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func equalStrings(v any, want []string) bool {
	got := a2ui.ToStrings(v)
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
