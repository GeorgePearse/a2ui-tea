package catalog

import (
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
)

const gap = 1

// Row lays children out horizontally. Children with a weight share the
// width left after unweighted children take their intrinsic width.
type Row struct{}

// Children implements Container.
func (Row) Children(f Frame, n a2ui.Node) []a2ui.Node {
	kids, _ := n.Children("children")
	return kids
}

// Render implements Component.
func (r Row) Render(f Frame, n a2ui.Node, width int) string {
	kids := r.Children(f, n)
	if len(kids) == 0 {
		return ""
	}
	widths := rowWidths(f, kids, width)
	parts := make([]string, len(kids))
	for i, k := range kids {
		parts[i] = f.Render(k, widths[i])
	}
	stretchDividers(f, kids, parts)
	pos := lipgloss.Top
	switch n.Component.StringProp("align", "stretch") {
	case "center":
		pos = lipgloss.Center
	case "end":
		pos = lipgloss.Bottom
	}
	return joinRow(parts, widths, width, n.Component.StringProp("justify", "start"), pos)
}

func rowWidths(f Frame, kids []a2ui.Node, width int) []int {
	n := len(kids)
	avail := width - gap*(n-1)
	if avail < n {
		avail = n
	}
	widths := make([]int, n)
	weights := make([]float64, n)
	fixed := 0
	totalWeight := 0.0
	for i, k := range kids {
		if k.Component == nil {
			widths[i] = 1
			fixed++
			continue
		}
		weights[i] = k.Component.Weight()
		if width >= Unbounded {
			weights[i] = 0
		}
		if weights[i] > 0 {
			totalWeight += weights[i]
			continue
		}
		w, _ := f.Measure(k)
		widths[i] = max(1, min(w, avail))
		fixed += widths[i]
	}
	if fixed > avail {
		scale := float64(avail) / float64(fixed)
		fixed = 0
		for i := range widths {
			if weights[i] == 0 {
				widths[i] = max(1, int(math.Floor(float64(widths[i])*scale)))
				fixed += widths[i]
			}
		}
	}
	if totalWeight > 0 {
		remain := avail - fixed
		used := 0
		last := -1
		for i := range kids {
			if weights[i] > 0 {
				widths[i] = max(1, int(math.Floor(float64(remain)*weights[i]/totalWeight)))
				used += widths[i]
				last = i
			}
		}
		if last >= 0 && remain-used > 0 {
			widths[last] += remain - used
		}
	}
	return widths
}

func stretchDividers(f Frame, kids []a2ui.Node, parts []string) {
	height := 0
	for _, p := range parts {
		height = max(height, lipgloss.Height(p))
	}
	for i, k := range kids {
		if k.Component != nil && k.Component.Type == "Divider" && k.Component.StringProp("axis", "horizontal") == "vertical" {
			parts[i] = strings.TrimRight(strings.Repeat(f.Theme().Border.Render("│")+"\n", height), "\n")
		}
	}
}

func joinRow(parts []string, widths []int, width int, justify string, pos lipgloss.Position) string {
	n := len(parts)
	for i := range parts {
		parts[i] = lipgloss.NewStyle().Width(widths[i]).Render(parts[i])
	}
	used := gap * (n - 1)
	for _, w := range widths {
		used += w
	}
	if width >= Unbounded {
		width = used
	}
	extra := max(0, width-used)
	var sep []int
	switch justify {
	case "spaceBetween":
		sep = distribute(extra, n-1, 0)
	case "spaceAround":
		sep = distribute(extra, n+1, 1)
	case "spaceEvenly":
		sep = distribute(extra, n+1, 0)
	}
	if sep == nil {
		joined := lipgloss.JoinHorizontal(pos, interleave(parts, strings.Repeat(" ", gap))...)
		switch justify {
		case "center":
			return lipgloss.PlaceHorizontal(width, lipgloss.Center, joined)
		case "end":
			return lipgloss.PlaceHorizontal(width, lipgloss.Right, joined)
		}
		return joined
	}
	items := make([]string, 0, 2*n+1)
	if justify != "spaceBetween" {
		items = append(items, strings.Repeat(" ", sep[0]))
		sep = sep[1:]
	}
	for i, p := range parts {
		items = append(items, p)
		if i < n-1 {
			items = append(items, strings.Repeat(" ", gap+sep[i]))
		}
	}
	if justify != "spaceBetween" && len(sep) >= n {
		items = append(items, strings.Repeat(" ", sep[n-1]))
	}
	return lipgloss.JoinHorizontal(pos, items...)
}

// distribute splits extra space into slots; mode 1 halves the outer slots
// (spaceAround).
func distribute(extra, slots, mode int) []int {
	if slots <= 0 {
		return []int{}
	}
	out := make([]int, slots)
	if mode == 1 && slots >= 2 {
		units := 2*(slots-2) + 2
		unit := extra / units
		for i := range out {
			out[i] = 2 * unit
		}
		out[0], out[slots-1] = unit, unit
		rem := extra - unit*units
		out[slots-1] += rem
		return out
	}
	base, rem := extra/slots, extra%slots
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
	}
	return out
}

func interleave(parts []string, sep string) []string {
	out := make([]string, 0, 2*len(parts))
	for i, p := range parts {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, p)
	}
	return out
}

// Column stacks children vertically.
type Column struct{}

// Children implements Container.
func (Column) Children(f Frame, n a2ui.Node) []a2ui.Node {
	kids, _ := n.Children("children")
	return kids
}

// Render implements Component.
func (c Column) Render(f Frame, n a2ui.Node, width int) string {
	return stack(f, c.Children(f, n), width, n.Component.StringProp("align", "stretch"), "")
}

func stack(f Frame, kids []a2ui.Node, width int, align, separator string) string {
	parts := make([]string, 0, len(kids))
	if width >= Unbounded {
		align = "stretch"
	}
	for _, k := range kids {
		s := f.Render(k, width)
		switch align {
		case "center":
			s = lipgloss.PlaceHorizontal(width, lipgloss.Center, s)
		case "end":
			s = lipgloss.PlaceHorizontal(width, lipgloss.Right, s)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, separator+"\n")
}

// List stacks or rows its children; template children expand per item.
type List struct{}

// Children implements Container.
func (List) Children(f Frame, n a2ui.Node) []a2ui.Node {
	kids, _ := n.Children("children")
	return kids
}

// Render implements Component.
func (l List) Render(f Frame, n a2ui.Node, width int) string {
	kids := l.Children(f, n)
	if n.Component.StringProp("direction", "vertical") == "horizontal" {
		widths := rowWidths(f, kids, width)
		parts := make([]string, len(kids))
		for i, k := range kids {
			parts[i] = f.Render(k, widths[i])
		}
		return joinRow(parts, widths, width, "start", lipgloss.Top)
	}
	return stack(f, kids, width, n.Component.StringProp("align", "stretch"), "")
}

// Card draws a rounded border around its single child.
type Card struct{}

// Children implements Container.
func (Card) Children(f Frame, n a2ui.Node) []a2ui.Node {
	child, _ := n.Child(n.Component.StringProp("child", ""))
	return []a2ui.Node{child}
}

// Render implements Component.
func (c Card) Render(f Frame, n a2ui.Node, width int) string {
	style := f.Theme().Card
	inner := width - style.GetHorizontalFrameSize()
	if inner < 1 {
		inner = 1
	}
	body := f.Render(c.Children(f, n)[0], inner)
	total := width
	if width >= Unbounded {
		total = max(1, lipgloss.Width(body)) + style.GetHorizontalFrameSize()
	}
	return style.Width(total).Render(body)
}

// Tabs shows a header row of titles and the selected tab's child.
type Tabs struct{}

func tabItems(f Frame, n a2ui.Node) (titles []string, kids []a2ui.Node) {
	raw, _ := n.Component.Prop("tabs")
	if raw == nil {
		raw, _ = n.Component.Prop("tabItems")
	}
	items, _ := raw.([]any)
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		titles = append(titles, f.Eval().String(m["title"], n.Scope))
		id, _ := m["child"].(string)
		child, _ := n.Child(id)
		kids = append(kids, child)
	}
	return titles, kids
}

func (Tabs) selected(f Frame, n a2ui.Node, count int) int {
	st := f.State(n)
	if count == 0 {
		return 0
	}
	if st.Index < 0 || st.Index >= count {
		st.Index = 0
	}
	return st.Index
}

// Children implements Container; only the active tab's child is live.
func (t Tabs) Children(f Frame, n a2ui.Node) []a2ui.Node {
	_, kids := tabItems(f, n)
	if len(kids) == 0 {
		return nil
	}
	return []a2ui.Node{kids[t.selected(f, n, len(kids))]}
}

// Focusable implements Focusable.
func (Tabs) Focusable(f Frame, n a2ui.Node) bool {
	titles, _ := tabItems(f, n)
	return len(titles) > 1
}

// HandleKey implements Focusable: left/right switch tabs.
func (t Tabs) HandleKey(f Frame, n a2ui.Node, key tea.KeyPressMsg) (bool, tea.Cmd) {
	titles, _ := tabItems(f, n)
	st := f.State(n)
	switch KeyName(key) {
	case "left", "h":
		st.Index = (t.selected(f, n, len(titles)) + len(titles) - 1) % len(titles)
	case "right", "l":
		st.Index = (t.selected(f, n, len(titles)) + 1) % len(titles)
	default:
		return false, nil
	}
	return true, nil
}

// Render implements Component.
func (t Tabs) Render(f Frame, n a2ui.Node, width int) string {
	th := f.Theme()
	titles, kids := tabItems(f, n)
	if len(titles) == 0 {
		return th.Muted.Render("(no tabs)")
	}
	sel := t.selected(f, n, len(titles))
	focused := f.Focused(n)
	var header []string
	for i, title := range titles {
		label := " " + title + " "
		switch {
		case i == sel && focused:
			label = th.Focus.Render("[" + title + "]")
		case i == sel:
			label = th.TabActive.Render(label)
		default:
			label = th.TabInactive.Render(label)
		}
		header = append(header, label)
	}
	head := strings.Join(header, th.Border.Render("│"))
	body := f.Render(kids[sel], width)
	ruleW := Bounded(width, max(lipgloss.Width(head), lipgloss.Width(body)))
	rule := th.Border.Render(strings.Repeat("─", max(1, ruleW)))
	return head + "\n" + rule + "\n" + body
}

// Modal shows its trigger inline; activating the trigger opens the content
// as an overlay.
type Modal struct{}

func modalParts(n a2ui.Node) (trigger, content a2ui.Node) {
	tid := n.Component.StringProp("trigger", n.Component.StringProp("entryPointChild", ""))
	cid := n.Component.StringProp("content", n.Component.StringProp("contentChild", ""))
	trigger, _ = n.Child(tid)
	content, _ = n.Child(cid)
	return trigger, content
}

// Children implements Container: the trigger is always live.
func (Modal) Children(f Frame, n a2ui.Node) []a2ui.Node {
	trigger, _ := modalParts(n)
	return []a2ui.Node{trigger}
}

// TriggerKey implements Trigger.
func (Modal) TriggerKey(f Frame, n a2ui.Node) string {
	trigger, _ := modalParts(n)
	return trigger.Key
}

// Overlay implements Overlay.
func (Modal) Overlay(f Frame, n a2ui.Node) (a2ui.Node, bool) {
	if !f.State(n).Open {
		return a2ui.Node{}, false
	}
	_, content := modalParts(n)
	return content, content.Component != nil
}

// Render implements Component.
func (Modal) Render(f Frame, n a2ui.Node, width int) string {
	trigger, _ := modalParts(n)
	return f.Render(trigger, width)
}
