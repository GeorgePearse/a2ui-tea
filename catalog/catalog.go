// Package catalog maps A2UI component types to terminal renderers. The
// BasicCatalog is registered by default; callers register their own
// component types to support custom catalogs.
package catalog

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
)

// BasicCatalogID is the id of the A2UI basic catalog, version 1.0.
const BasicCatalogID = "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json"

// Frame is the host a renderer draws into. It hands out per-instance state,
// renders child nodes, reports focus and dispatches actions.
type Frame interface {
	// Eval resolves dynamic values.
	Eval() *a2ui.Evaluator
	// Theme returns the active styles.
	Theme() *Theme
	// Render draws any node at the given width, dispatching on its type.
	Render(n a2ui.Node, width int) string
	// Measure returns a node's intrinsic width and height.
	Measure(n a2ui.Node) (width, height int)
	// Focused reports whether the node holds keyboard focus.
	Focused(n a2ui.Node) bool
	// State returns the node's UI state, created on first use.
	State(n a2ui.Node) *State
	// Dispatch runs the node's action: an event goes to the agent, a
	// function call runs locally. Modal triggers open their modal.
	Dispatch(n a2ui.Node) tea.Cmd
	// CloseModal closes the open modal, if any.
	CloseModal()
}

// Component renders one A2UI component type.
type Component interface {
	Render(f Frame, n a2ui.Node, width int) string
}

// Container is implemented by components with child nodes; the host walks
// them to build the focus ring.
type Container interface {
	Children(f Frame, n a2ui.Node) []a2ui.Node
}

// Focusable is implemented by interactive components.
type Focusable interface {
	// Focusable reports whether this instance can take focus now.
	Focusable(f Frame, n a2ui.Node) bool
	// HandleKey receives key presses while focused and reports whether it
	// consumed the key.
	HandleKey(f Frame, n a2ui.Node, key tea.KeyPressMsg) (bool, tea.Cmd)
}

// Overlay is implemented by components that show content above the surface
// (Modal). The host renders the returned node centred over everything.
type Overlay interface {
	Overlay(f Frame, n a2ui.Node) (a2ui.Node, bool)
}

// Trigger is implemented by components whose child activation must be
// intercepted (Modal: pressing the trigger opens the modal).
type Trigger interface {
	TriggerKey(f Frame, n a2ui.Node) string
}

// State is the UI state of one rendered instance.
type State struct {
	Input *textinput.Model
	Index int
	Open  bool
	Extra any
}

// Registry maps component type names to renderers.
type Registry struct {
	comps map[string]Component
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{comps: map[string]Component{}}
}

// Basic returns a registry with the BasicCatalog components.
func Basic() *Registry {
	r := New()
	r.Register("Text", Text{})
	r.Register("Icon", Icon{})
	r.Register("Image", Image{})
	r.Register("Video", Video{})
	r.Register("AudioPlayer", AudioPlayer{})
	r.Register("Divider", Divider{})
	r.Register("Row", Row{})
	r.Register("Column", Column{})
	r.Register("List", List{})
	r.Register("Card", Card{})
	r.Register("Tabs", Tabs{})
	r.Register("Modal", Modal{})
	r.Register("Button", Button{})
	r.Register("TextField", TextField{})
	r.Register("CheckBox", CheckBox{})
	r.Register("ChoicePicker", ChoicePicker{})
	r.Register("Slider", Slider{})
	r.Register("DateTimeInput", DateTimeInput{})
	return r
}

// Register adds or replaces a component type.
func (r *Registry) Register(name string, c Component) {
	r.comps[name] = c
}

// Lookup returns the renderer for a component type.
func (r *Registry) Lookup(name string) (Component, bool) {
	c, ok := r.comps[name]
	return c, ok
}

// Names lists the registered component types.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.comps))
	for n := range r.comps {
		out = append(out, n)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Func is a convenience alias for registering catalog functions.
type Func = a2ui.Func

// KeyName normalises a key press for matching: printable keys yield their
// text ("a", "?"), special keys their name ("tab", "shift+tab", "enter",
// "space", "left").
func KeyName(key tea.KeyPressMsg) string {
	k := key.String()
	if k == " " {
		return "space"
	}
	return k
}

// Unbounded is the width a Frame passes to Measure a node's intrinsic size.
// Components that normally fill their width should shrink-wrap when given
// it; Bounded helps pick a sensible fallback.
const Unbounded = 1 << 20

// Bounded returns width unless it is Unbounded, in which case fallback.
func Bounded(width, fallback int) int {
	if width >= Unbounded {
		return fallback
	}
	return width
}
