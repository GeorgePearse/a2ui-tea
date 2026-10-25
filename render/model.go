// Package render is the Bubble Tea program that draws A2UI surfaces. Its
// View walks the surface tree on every frame, so component updates from the
// agent change the UI while it runs.
package render

import (
	"io"
	"log"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
	"github.com/GeorgePearse/a2ui-tea/catalog"
)

// Options configures a Model.
type Options struct {
	// Registry supplies component renderers; nil means the basic catalog.
	Registry *catalog.Registry
	// Evaluator resolves bindings and functions; nil means the basic set.
	Evaluator *a2ui.Evaluator
	// Theme styles the components; nil means the default theme.
	Theme *catalog.Theme
	// Outbound receives renderer-to-agent messages (actions, errors,
	// function responses).
	Outbound func(a2ui.Outbound)
	// DataModel receives the data model sync payload after each action for
	// surfaces created with sendDataModel.
	DataModel func(a2ui.RendererDataModel)
	// Log receives a debug trace; nil discards it.
	Log *log.Logger
	// AltScreen runs the program in the alternate screen buffer.
	AltScreen bool
	// HideHelp removes the key hint line at the bottom.
	HideHelp bool
	// QuitOnEmpty ends the program when the last surface is deleted.
	QuitOnEmpty bool
	// Width and Height set the initial size before the terminal reports one.
	Width, Height int
}

// MessagesMsg delivers agent messages to the model.
type MessagesMsg []a2ui.Message

// ResetMsg drops every surface (used when a watched file is rewritten).
type ResetMsg struct{}

// StatusMsg shows a transient status line.
type StatusMsg string

// Model is the Bubble Tea model. It implements catalog.Frame.
type Model struct {
	opts     Options
	store    *a2ui.Store
	reg      *catalog.Registry
	eval     *a2ui.Evaluator
	theme    *catalog.Theme
	log      *log.Logger
	width    int
	height   int
	focus    string
	ring     []string
	nodes    map[string]a2ui.Node
	triggers map[string]string
	modal    string
	overlay  a2ui.Node
	states   map[string]*catalog.State
	measures map[string][2]int
	scroll   int
	status   string
	failed   bool
	seen     bool
	quitting bool
	now      func() time.Time
}

// New returns a model with no surfaces.
func New(opts Options) *Model {
	if opts.Registry == nil {
		opts.Registry = catalog.Basic()
	}
	if opts.Evaluator == nil {
		opts.Evaluator = a2ui.NewEvaluator()
	}
	if opts.Theme == nil {
		opts.Theme = catalog.DefaultTheme()
	}
	if opts.Log == nil {
		opts.Log = log.New(io.Discard, "", 0)
	}
	m := &Model{
		opts:     opts,
		store:    a2ui.NewStore(),
		reg:      opts.Registry,
		eval:     opts.Evaluator,
		theme:    opts.Theme,
		log:      opts.Log,
		width:    opts.Width,
		height:   opts.Height,
		nodes:    map[string]a2ui.Node{},
		triggers: map[string]string{},
		states:   map[string]*catalog.State{},
		measures: map[string][2]int{},
		now:      time.Now,
	}
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 0 {
		m.height = 24
	}
	return m
}

// Store exposes the surface store.
func (m *Model) Store() *a2ui.Store {
	return m.store
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Apply handles one agent message outside the Bubble Tea loop (tests and
// static rendering). Errors are also reported to the agent via Outbound.
func (m *Model) Apply(msg a2ui.Message) error {
	m.log.Printf("<- %s %s", msg.Kind(), msg.SurfaceID())
	var err error
	switch {
	case msg.CallRendererFunction != nil:
		m.emit(m.eval.HandleCall(m.store, msg.CallRendererFunction))
	case msg.AgentFunctionResponse != nil:
		m.log.Printf("agentFunctionResponse %s ignored", msg.AgentFunctionResponse.FunctionCallID)
	default:
		err = m.store.Apply(msg)
	}
	if err != nil {
		m.log.Printf("error: %v", err)
		m.status, m.failed = err.Error(), true
		m.emit(a2ui.ErrorMessage(err, msg.SurfaceID()))
	} else if m.store.Len() > 0 {
		m.seen = true
	}
	m.rebuild()
	return err
}

// ApplyAll applies messages in order and returns the first error.
func (m *Model) ApplyAll(msgs []a2ui.Message) error {
	var first error
	for _, msg := range msgs {
		if err := m.Apply(msg); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// SetSize sets the terminal size.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = max(1, width), max(1, height)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case MessagesMsg:
		for _, am := range msg {
			_ = m.Apply(am)
		}
		if m.opts.QuitOnEmpty && m.seen && m.store.Len() == 0 {
			m.quitting = true
			return m, tea.Quit
		}
	case ResetMsg:
		m.store = a2ui.NewStore()
		m.states = map[string]*catalog.State{}
		m.modal = ""
		m.status = ""
		m.rebuild()
	case StatusMsg:
		m.status, m.failed = string(msg), false
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.scroll -= 3
		case tea.MouseWheelDown:
			m.scroll += 3
		}
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(key tea.KeyPressMsg) tea.Cmd {
	m.status = ""
	name := catalog.KeyName(key)
	if name == "ctrl+c" {
		m.quitting = true
		return tea.Quit
	}
	if n, ok := m.nodes[m.focus]; ok {
		if comp, ok := m.reg.Lookup(n.Component.Type); ok {
			if fc, ok := comp.(catalog.Focusable); ok {
				handled, cmd := fc.HandleKey(m, n, key)
				m.rebuild()
				if handled {
					return cmd
				}
			}
		}
	}
	switch name {
	case "q":
		m.quitting = true
		return tea.Quit
	case "tab", "down", "j":
		m.moveFocus(1)
	case "shift+tab", "up", "k":
		m.moveFocus(-1)
	case "esc":
		m.CloseModal()
	case "pgdown":
		m.scroll += max(1, m.height/2)
	case "pgup":
		m.scroll -= max(1, m.height/2)
	case "home":
		m.scroll = 0
	}
	return nil
}

func (m *Model) moveFocus(delta int) {
	if len(m.ring) == 0 {
		return
	}
	idx := -1
	for i, k := range m.ring {
		if k == m.focus {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(m.ring)) % len(m.ring)
	if idx < 0 {
		idx = 0
	}
	m.focus = m.ring[idx]
	m.revealFocus()
}

// rebuild walks every surface to refresh the node index, focus ring and
// modal state after any change.
func (m *Model) rebuild() {
	m.nodes = map[string]a2ui.Node{}
	m.triggers = map[string]string{}
	m.measures = map[string][2]int{}
	m.modal = ""
	m.overlay = a2ui.Node{}
	var main, overlay []string
	for _, s := range m.store.Surfaces() {
		if root, ok := s.Root(); ok {
			m.walk(root, false, &main, &overlay)
		}
	}
	m.ring = main
	if m.modal != "" {
		m.ring = overlay
	}
	if !contains(m.ring, m.focus) {
		m.focus = ""
		if len(m.ring) > 0 {
			m.focus = m.ring[0]
		}
	}
}

func (m *Model) walk(n a2ui.Node, inOverlay bool, main, overlay *[]string) {
	if n.Component == nil {
		return
	}
	comp, ok := m.reg.Lookup(n.Component.Type)
	if !ok {
		return
	}
	m.nodes[n.Key] = n
	if f, ok := comp.(catalog.Focusable); ok && f.Focusable(m, n) {
		if inOverlay {
			*overlay = append(*overlay, n.Key)
		} else {
			*main = append(*main, n.Key)
		}
	}
	if t, ok := comp.(catalog.Trigger); ok {
		m.triggers[t.TriggerKey(m, n)] = n.Key
	}
	if o, ok := comp.(catalog.Overlay); ok && m.modal == "" {
		if content, open := o.Overlay(m, n); open {
			m.modal = n.Key
			m.overlay = content
			m.walk(content, true, main, overlay)
		}
	}
	if c, ok := comp.(catalog.Container); ok {
		for _, k := range c.Children(m, n) {
			m.walk(k, inOverlay, main, overlay)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (m *Model) emit(out a2ui.Outbound) {
	m.log.Printf("-> %+v", out)
	if m.opts.Outbound != nil {
		m.opts.Outbound(out)
	}
}

// Eval implements catalog.Frame.
func (m *Model) Eval() *a2ui.Evaluator {
	return m.eval
}

// Theme implements catalog.Frame.
func (m *Model) Theme() *catalog.Theme {
	return m.theme
}

// Focused implements catalog.Frame.
func (m *Model) Focused(n a2ui.Node) bool {
	return n.Key != "" && n.Key == m.focus
}

// State implements catalog.Frame.
func (m *Model) State(n a2ui.Node) *catalog.State {
	st, ok := m.states[n.Key]
	if !ok {
		st = &catalog.State{}
		m.states[n.Key] = st
	}
	return st
}

// Render implements catalog.Frame.
func (m *Model) Render(n a2ui.Node, width int) string {
	if width < 1 {
		width = 1
	}
	if n.Component == nil {
		id := n.Key[strings.LastIndex(n.Key, "/")+1:]
		return m.theme.Muted.Render("⟨missing " + id + "⟩")
	}
	comp, ok := m.reg.Lookup(n.Component.Type)
	if !ok {
		return m.theme.Muted.Render("⟨unsupported " + n.Component.Type + "⟩")
	}
	return comp.Render(m, n, width)
}

// Measure implements catalog.Frame.
func (m *Model) Measure(n a2ui.Node) (int, int) {
	if n.Key != "" {
		if wh, ok := m.measures[n.Key]; ok {
			return wh[0], wh[1]
		}
	}
	out := m.Render(n, catalog.Unbounded)
	w, h := 0, 0
	for _, line := range strings.Split(out, "\n") {
		w = max(w, ansi.StringWidth(strings.TrimRight(line, " ")))
		h++
	}
	if n.Key != "" {
		m.measures[n.Key] = [2]int{w, h}
	}
	return w, h
}

// Dispatch implements catalog.Frame.
func (m *Model) Dispatch(n a2ui.Node) tea.Cmd {
	if modalKey, ok := m.triggers[n.Key]; ok {
		if modal, ok := m.nodes[modalKey]; ok {
			m.State(modal).Open = true
			m.log.Printf("open modal %s", modalKey)
		}
	}
	event, call := a2ui.ActionOf(n.Component)
	switch {
	case event != nil:
		m.emit(m.eval.BuildAction(n, event, m.now()))
		if n.Surface.SendDataModel && m.opts.DataModel != nil {
			m.opts.DataModel(a2ui.RendererDataModel{Version: a2ui.CurrentVersion, Surfaces: m.store.DataModels(true)})
		}
	case call != nil:
		name, args, ok := a2ui.IsCall(call)
		if !ok {
			m.status, m.failed = "action.functionCall needs @call", true
			break
		}
		if _, err := m.eval.Call(name, args, n.Scope); err != nil {
			m.status, m.failed = err.Error(), true
			m.log.Printf("functionCall %s: %v", name, err)
		}
	}
	m.rebuild()
	return nil
}

// CloseModal implements catalog.Frame.
func (m *Model) CloseModal() {
	if m.modal == "" {
		return
	}
	trigger := ""
	for t, mk := range m.triggers {
		if mk == m.modal {
			trigger = t
		}
	}
	m.State(m.nodes[m.modal]).Open = false
	m.modal = ""
	m.rebuild()
	if contains(m.ring, trigger) {
		m.focus = trigger
	}
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	content := m.content(m.width)
	if m.modal != "" {
		content = m.compose(content)
	}
	lines := strings.Split(content, "\n")
	avail := m.height
	var help string
	if !m.opts.HideHelp {
		help = m.helpLine()
		avail--
	}
	maxScroll := max(0, len(lines)-avail)
	m.scroll = min(maxScroll, max(0, m.scroll))
	if len(lines) > avail {
		lines = lines[m.scroll:min(len(lines), m.scroll+avail)]
	}
	for len(lines) < avail {
		lines = append(lines, "")
	}
	out := strings.Join(lines, "\n")
	if help != "" {
		out += "\n" + help
	}
	v := tea.NewView(out)
	v.AltScreen = m.opts.AltScreen
	return v
}

// RenderSurface draws one surface at the given width with no chrome.
func (m *Model) RenderSurface(id string, width int) string {
	m.measures = map[string][2]int{}
	s, ok := m.store.Surface(id)
	if !ok {
		return ""
	}
	root, ok := s.Root()
	if !ok {
		return m.theme.Muted.Render("waiting for root component…")
	}
	return m.Render(root, width)
}

func (m *Model) content(width int) string {
	m.measures = map[string][2]int{}
	surfaces := m.store.Surfaces()
	if len(surfaces) == 0 {
		return m.theme.Muted.Render("no surface yet — waiting for createSurface")
	}
	parts := make([]string, 0, len(surfaces))
	for _, s := range surfaces {
		body := m.RenderSurface(s.ID, width)
		if len(surfaces) > 1 {
			title := m.theme.Muted.Render("─ " + s.ID + " " + strings.Repeat("─", max(0, width-ansi.StringWidth(s.ID)-3)))
			body = title + "\n" + body
		}
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n")
}

func (m *Model) helpLine() string {
	if m.status != "" && m.failed {
		return m.theme.Error.Render(ansi.Truncate("! "+m.status, m.width, "…"))
	}
	if m.status != "" {
		return m.theme.Accent.Render(ansi.Truncate(m.status, m.width, "…"))
	}
	keys := "tab/shift+tab move · enter/space press · ←/→ adjust · q quit"
	if m.modal != "" {
		keys = "tab/shift+tab move · enter/space press · esc close · q quit"
	}
	return m.theme.Muted.Render(ansi.Truncate(keys, m.width, "…"))
}

// compose draws the open modal's content centred over the base content.
func (m *Model) compose(base string) string {
	boxW := min(max(20, m.width-6), 60)
	inner := boxW - m.theme.Modal.GetHorizontalFrameSize()
	body := m.Render(m.overlay, inner)
	body += "\n" + m.theme.Muted.Render("esc to close")
	dialog := m.theme.Modal.Width(boxW).Render(body)
	baseLines := strings.Split(base, "\n")
	height := max(len(baseLines), lipgloss.Height(dialog)+2)
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	dw, dh := lipgloss.Width(dialog), lipgloss.Height(dialog)
	x := max(0, (m.width-dw)/2)
	y := max(0, (min(height, m.height)-dh)/2)
	dialogLines := strings.Split(dialog, "\n")
	for i, dl := range dialogLines {
		row := y + i
		if row >= len(baseLines) {
			break
		}
		baseLines[row] = overlayLine(baseLines[row], dl, x, dw, m.width)
	}
	return strings.Join(baseLines, "\n")
}

func overlayLine(base, over string, x, w, width int) string {
	left := ansi.Truncate(base, x, "")
	if pad := x - ansi.StringWidth(left); pad > 0 {
		left += strings.Repeat(" ", pad)
	}
	right := ""
	if ansi.StringWidth(base) > x+w {
		right = ansi.TruncateLeft(base, x+w, "")
	}
	return left + over + right
}

// revealFocus scrolls so the focused node is on screen. The node is located
// by searching the rendered content for its first line.
func (m *Model) revealFocus() {
	n, ok := m.nodes[m.focus]
	if !ok {
		return
	}
	first := strings.SplitN(m.Render(n, m.width), "\n", 2)[0]
	needle := strings.TrimSpace(ansi.Strip(first))
	if needle == "" {
		return
	}
	avail := m.height
	if !m.opts.HideHelp {
		avail--
	}
	for i, line := range strings.Split(ansi.Strip(m.content(m.width)), "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		if i < m.scroll {
			m.scroll = i
		} else if i >= m.scroll+avail {
			m.scroll = i - avail + 1
		}
		return
	}
}
