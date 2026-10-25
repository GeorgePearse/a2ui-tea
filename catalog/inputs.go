package catalog

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
)

// Button renders its child inside brackets and dispatches its action on
// Enter or Space. Failing checks disable it.
type Button struct{}

// Children implements Container.
func (Button) Children(f Frame, n a2ui.Node) []a2ui.Node {
	child, _ := n.Child(n.Component.StringProp("child", ""))
	return []a2ui.Node{child}
}

// Focusable implements Focusable.
func (Button) Focusable(f Frame, n a2ui.Node) bool {
	return true
}

// HandleKey implements Focusable.
func (b Button) HandleKey(f Frame, n a2ui.Node, key tea.KeyPressMsg) (bool, tea.Cmd) {
	switch KeyName(key) {
	case "enter", "space":
		if len(a2ui.Failures(f.Eval().Checks(n.Component, n.Scope))) > 0 {
			return true, nil
		}
		return true, f.Dispatch(n)
	}
	return false, nil
}

// Render implements Component.
func (b Button) Render(f Frame, n a2ui.Node, width int) string {
	th := f.Theme()
	child := b.Children(f, n)[0]
	label := ansi.Truncate(strings.ReplaceAll(f.Render(child, max(1, width-4)), "\n", " "), max(1, width-4), "…")
	failures := a2ui.Failures(f.Eval().Checks(n.Component, n.Scope))
	variant := n.Component.StringProp("variant", "default")
	if n.Component.BoolProp("primary", false) {
		variant = "primary"
	}
	focused := f.Focused(n)
	var out string
	label = ansi.Strip(label)
	switch {
	case len(failures) > 0:
		out = th.ButtonDisabled.Render("[ " + label + " ]")
	case focused:
		out = th.ButtonFocused.Render("[ " + label + " ]")
	case variant == "primary":
		out = th.ButtonPrimary.Render("[ " + label + " ]")
	case variant == "borderless":
		out = th.Link.Render(label)
	default:
		out = th.Button.Render("[ " + label + " ]")
	}
	if focused && len(failures) > 0 {
		out += "\n" + th.Error.Render(wrap("! "+failures[0].Message, width))
	}
	return out
}

// TextField is a single-line text input bound to a data model path.
type TextField struct{}

// Focusable implements Focusable.
func (TextField) Focusable(f Frame, n a2ui.Node) bool {
	return true
}

func inputFor(f Frame, n a2ui.Node, placeholder string, obscured bool) *textinput.Model {
	st := f.State(n)
	if st.Input == nil {
		in := textinput.New()
		in.Prompt = ""
		in.SetVirtualCursor(true)
		st.Input = &in
	}
	st.Input.Placeholder = placeholder
	if obscured {
		st.Input.EchoMode = textinput.EchoPassword
	}
	return st.Input
}

// syncInput makes the input show the data model's value: the input writes
// to the model on every keystroke, so a difference means the agent changed
// it.
func syncInput(f Frame, n a2ui.Node, in *textinput.Model) {
	raw, ok := n.Component.Prop("value")
	if !ok {
		return
	}
	v := f.Eval().String(raw, n.Scope)
	if _, bound := a2ui.IsBinding(raw); !bound {
		if st := f.State(n); st.Extra == nil {
			st.Extra = true
			in.SetValue(v)
		}
		return
	}
	if in.Value() != v {
		in.SetValue(v)
		in.CursorEnd()
	}
}

func writeBack(f Frame, n a2ui.Node, value any) {
	raw, ok := n.Component.Prop("value")
	if !ok {
		return
	}
	if p, bound := a2ui.IsBinding(raw); bound {
		_ = n.Scope.Set(p, value)
	}
}

// HandleKey implements Focusable: keys go to the text input.
func (TextField) HandleKey(f Frame, n a2ui.Node, key tea.KeyPressMsg) (bool, tea.Cmd) {
	in := inputFor(f, n, "", false)
	switch KeyName(key) {
	case "tab", "shift+tab", "enter", "esc", "up", "down":
		return false, nil
	}
	in.Focus()
	updated, cmd := in.Update(key)
	*in = updated
	writeBack(f, n, in.Value())
	return true, cmd
}

// Render implements Component.
func (TextField) Render(f Frame, n a2ui.Node, width int) string {
	th := f.Theme()
	ev := f.Eval()
	variant := n.Component.StringProp("variant", n.Component.StringProp("textFieldType", "shortText"))
	in := inputFor(f, n, ev.String(n.Component.Props["placeholder"], n.Scope), variant == "obscured")
	syncInput(f, n, in)
	return renderInput(f, n, in, width, ev.String(n.Component.Props["label"], n.Scope), th)
}

func renderInput(f Frame, n a2ui.Node, in *textinput.Model, width int, label string, th *Theme) string {
	focused := f.Focused(n)
	if focused {
		in.Focus()
	} else {
		in.Blur()
	}
	width = Bounded(width, 40)
	fieldW := max(4, width-2)
	marker := "  "
	if focused {
		marker = th.Focus.Render("› ")
	}
	var lines []string
	if label != "" {
		lines = append(lines, th.Label.Render(ansi.Truncate(label, width, "…")))
	}
	lines = append(lines, marker+fieldView(in, fieldW, focused, th))
	if failures := a2ui.Failures(f.Eval().Checks(n.Component, n.Scope)); len(failures) > 0 && (focused || in.Value() != "") {
		lines = append(lines, th.Error.Render(wrap("! "+failures[0].Message, width)))
	}
	return strings.Join(lines, "\n")
}

// fieldView draws the input's value in a fixed-width underlined field with
// a block cursor when focused; long values scroll to keep the cursor
// visible.
func fieldView(in *textinput.Model, width int, focused bool, th *Theme) string {
	runes := []rune(in.Value())
	if in.EchoMode == textinput.EchoPassword {
		runes = []rune(strings.Repeat(string(in.EchoCharacter), len(runes)))
	}
	line := lipgloss.NewStyle().Underline(true)
	if len(runes) == 0 && !focused && in.Placeholder != "" {
		return th.Placeholder.Underline(true).Render(padRight(ansi.Truncate(in.Placeholder, width, "…"), width))
	}
	pos := min(len(runes), max(0, in.Position()))
	start := 0
	if focused && pos >= width {
		start = pos - width + 1
	}
	visible := runes[start:min(len(runes), start+width)]
	if !focused {
		return line.Render(padRight(ansi.Truncate(string(visible), width, "…"), width))
	}
	cur := min(pos-start, len(visible))
	before, after, cursor := string(visible[:cur]), "", " "
	if cur < len(visible) {
		cursor, after = string(visible[cur]), string(visible[cur+1:])
	}
	pad := max(0, width-len(visible)-1)
	if cur < len(visible) {
		pad = max(0, width-len(visible))
	}
	return line.Render(before) + th.ButtonFocused.Underline(true).Render(cursor) + line.Render(after+strings.Repeat(" ", pad))
}

func padRight(s string, width int) string {
	if pad := width - ansi.StringWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// CheckBox toggles a boolean binding.
type CheckBox struct{}

// Focusable implements Focusable.
func (CheckBox) Focusable(f Frame, n a2ui.Node) bool {
	return true
}

// HandleKey implements Focusable.
func (CheckBox) HandleKey(f Frame, n a2ui.Node, key tea.KeyPressMsg) (bool, tea.Cmd) {
	switch KeyName(key) {
	case "enter", "space":
		writeBack(f, n, !f.Eval().Bool(n.Component.Props["value"], n.Scope))
		return true, nil
	}
	return false, nil
}

// Render implements Component.
func (CheckBox) Render(f Frame, n a2ui.Node, width int) string {
	th := f.Theme()
	ev := f.Eval()
	box := "[ ]"
	if ev.Bool(n.Component.Props["value"], n.Scope) {
		box = "[x]"
	}
	if f.Focused(n) {
		box = th.Focus.Render(box)
	}
	label := ev.String(n.Component.Props["label"], n.Scope)
	out := box + " " + wrapIndent(label, width, 4)
	if failures := a2ui.Failures(ev.Checks(n.Component, n.Scope)); len(failures) > 0 && f.Focused(n) {
		out += "\n" + th.Error.Render(wrap("! "+failures[0].Message, width))
	}
	return out
}

func wrapIndent(s string, width, indent int) string {
	w := max(1, width-indent)
	lines := strings.Split(wrap(s, w), "\n")
	return strings.Join(lines, "\n"+strings.Repeat(" ", indent))
}

// ChoicePicker selects one or more options and writes the selected values
// as a string list.
type ChoicePicker struct{}

type option struct {
	label, value string
}

func options(f Frame, n a2ui.Node) []option {
	raw, _ := n.Component.Prop("options")
	items, _ := raw.([]any)
	out := make([]option, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		value, _ := m["value"].(string)
		out = append(out, option{label: f.Eval().String(m["label"], n.Scope), value: value})
	}
	return out
}

func (ChoicePicker) multiple(n a2ui.Node) bool {
	if v, ok := n.Component.Prop("maxAllowedSelections"); ok {
		if mx, ok := a2ui.ToNumber(v); ok {
			return mx != 1
		}
	}
	return n.Component.StringProp("variant", "mutuallyExclusive") == "multipleSelection"
}

func (ChoicePicker) selected(f Frame, n a2ui.Node) []string {
	raw, ok := n.Component.Prop("value")
	if !ok {
		raw = n.Component.Props["selections"]
	}
	return f.Eval().Strings(raw, n.Scope)
}

func (c ChoicePicker) write(f Frame, n a2ui.Node, values []string) {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	raw, ok := n.Component.Prop("value")
	if !ok {
		raw = n.Component.Props["selections"]
	}
	if p, bound := a2ui.IsBinding(raw); bound {
		_ = n.Scope.Set(p, out)
	}
}

// Focusable implements Focusable.
func (ChoicePicker) Focusable(f Frame, n a2ui.Node) bool {
	return len(options(f, n)) > 0
}

// HandleKey implements Focusable: up/down move, space/enter toggle.
func (c ChoicePicker) HandleKey(f Frame, n a2ui.Node, key tea.KeyPressMsg) (bool, tea.Cmd) {
	opts := options(f, n)
	st := f.State(n)
	chips := n.Component.StringProp("displayStyle", "checkbox") == "chips"
	switch k := KeyName(key); {
	case k == "up" || k == "k" || (chips && k == "left"):
		st.Index = (st.Index + len(opts) - 1) % len(opts)
	case k == "down" || k == "j" || (chips && k == "right"):
		st.Index = (st.Index + 1) % len(opts)
	case k == "enter" || k == "space":
		if st.Index >= len(opts) {
			st.Index = 0
		}
		value := opts[st.Index].value
		cur := c.selected(f, n)
		if !c.multiple(n) {
			c.write(f, n, []string{value})
			return true, nil
		}
		var next []string
		found := false
		for _, v := range cur {
			if v == value {
				found = true
				continue
			}
			next = append(next, v)
		}
		if !found {
			next = append(next, value)
		}
		if next == nil {
			next = []string{}
		}
		c.write(f, n, next)
	default:
		return false, nil
	}
	return true, nil
}

// Render implements Component.
func (c ChoicePicker) Render(f Frame, n a2ui.Node, width int) string {
	th := f.Theme()
	ev := f.Eval()
	opts := options(f, n)
	sel := c.selected(f, n)
	isSel := func(v string) bool {
		for _, s := range sel {
			if s == v {
				return true
			}
		}
		return false
	}
	focused := f.Focused(n)
	st := f.State(n)
	if st.Index >= len(opts) {
		st.Index = 0
	}
	var lines []string
	if label := ev.String(n.Component.Props["label"], n.Scope); label != "" {
		lines = append(lines, th.Label.Render(ansi.Truncate(label, width, "…")))
	}
	multiple := c.multiple(n)
	chips := n.Component.StringProp("displayStyle", "checkbox") == "chips"
	var items []string
	for i, o := range opts {
		var mark string
		switch {
		case chips && isSel(o.value):
			mark = "(•" + o.label + ")"
		case chips:
			mark = "( " + o.label + " )"
		case multiple && isSel(o.value):
			mark = "[x] " + o.label
		case multiple:
			mark = "[ ] " + o.label
		case isSel(o.value):
			mark = "(•) " + o.label
		default:
			mark = "( ) " + o.label
		}
		if focused && i == st.Index {
			mark = th.Focus.Render(mark)
		}
		items = append(items, mark)
	}
	if chips {
		lines = append(lines, wrap(strings.Join(items, " "), width))
	} else {
		for _, it := range items {
			lines = append(lines, wrapIndent(it, width, 4))
		}
	}
	if failures := a2ui.Failures(ev.Checks(n.Component, n.Scope)); len(failures) > 0 && focused {
		lines = append(lines, th.Error.Render(wrap("! "+failures[0].Message, width)))
	}
	return strings.Join(lines, "\n")
}

// Slider renders a numeric range as a bar; left/right adjust the value.
type Slider struct{}

func (Slider) bounds(n a2ui.Node) (lo, hi, step float64) {
	lo = n.Component.NumberProp("min", n.Component.NumberProp("minValue", 0))
	hi = n.Component.NumberProp("max", n.Component.NumberProp("maxValue", 100))
	if hi <= lo {
		hi = lo + 1
	}
	steps := n.Component.NumberProp("steps", 0)
	if steps <= 0 {
		steps = 20
	}
	return lo, hi, (hi - lo) / steps
}

// Focusable implements Focusable.
func (Slider) Focusable(f Frame, n a2ui.Node) bool {
	return true
}

// HandleKey implements Focusable.
func (s Slider) HandleKey(f Frame, n a2ui.Node, key tea.KeyPressMsg) (bool, tea.Cmd) {
	lo, hi, step := s.bounds(n)
	v, _ := f.Eval().Number(n.Component.Props["value"], n.Scope)
	switch KeyName(key) {
	case "left", "h", "down":
		v -= step
	case "right", "l", "up":
		v += step
	case "home":
		v = lo
	case "end":
		v = hi
	default:
		return false, nil
	}
	v = min(hi, max(lo, v))
	writeBack(f, n, roundTo(v, step))
	return true, nil
}

func roundTo(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	scaled := v / step
	r := float64(int64(scaled + 0.5))
	if scaled < 0 {
		r = float64(int64(scaled - 0.5))
	}
	out := r * step
	if out == float64(int64(out)) {
		return out
	}
	return float64(int64(out*1e6+0.5)) / 1e6
}

// Render implements Component.
func (s Slider) Render(f Frame, n a2ui.Node, width int) string {
	th := f.Theme()
	ev := f.Eval()
	lo, hi, _ := s.bounds(n)
	v, _ := ev.Number(n.Component.Props["value"], n.Scope)
	v = min(hi, max(lo, v))
	label := ev.String(n.Component.Props["label"], n.Scope)
	valueText := a2ui.FormatFloat(v)
	barW := min(30, max(4, width-len(valueText)-4))
	filled := int(float64(barW-1) * (v - lo) / (hi - lo))
	bar := "[" + strings.Repeat("=", filled) + ">" + strings.Repeat("-", max(0, barW-1-filled)) + "]"
	if f.Focused(n) {
		bar = th.Focus.Render(bar)
	}
	line := bar + " " + valueText
	if label != "" {
		return th.Label.Render(ansi.Truncate(label, width, "…")) + "\n" + line
	}
	return line
}

// DateTimeInput is a text input for ISO 8601 dates and times.
type DateTimeInput struct{}

// Focusable implements Focusable.
func (DateTimeInput) Focusable(f Frame, n a2ui.Node) bool {
	return true
}

func (DateTimeInput) placeholder(n a2ui.Node) string {
	date := n.Component.BoolProp("enableDate", false)
	tm := n.Component.BoolProp("enableTime", false)
	switch {
	case date && tm:
		return "YYYY-MM-DDTHH:MM"
	case tm:
		return "HH:MM"
	}
	return "YYYY-MM-DD"
}

// HandleKey implements Focusable.
func (d DateTimeInput) HandleKey(f Frame, n a2ui.Node, key tea.KeyPressMsg) (bool, tea.Cmd) {
	return TextField{}.HandleKey(f, n, key)
}

// Render implements Component.
func (d DateTimeInput) Render(f Frame, n a2ui.Node, width int) string {
	th := f.Theme()
	ev := f.Eval()
	in := inputFor(f, n, d.placeholder(n), false)
	syncInput(f, n, in)
	label := ev.String(n.Component.Props["label"], n.Scope)
	if label == "" {
		label = "Date"
		if n.Component.BoolProp("enableTime", false) && !n.Component.BoolProp("enableDate", false) {
			label = "Time"
		}
	}
	return renderInput(f, n, in, width, "📅 "+label, th)
}
