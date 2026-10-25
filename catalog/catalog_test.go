package catalog

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestDistribute(t *testing.T) {
	cases := []struct {
		extra, slots, mode int
		want               []int
	}{
		{10, 2, 0, []int{5, 5}},
		{7, 3, 0, []int{3, 2, 2}},
		{12, 4, 1, []int{2, 4, 4, 2}},
		{0, 0, 0, []int{}},
	}
	for _, c := range cases {
		got := distribute(c.extra, c.slots, c.mode)
		if !equalInts(got, c.want) {
			t.Errorf("distribute(%d,%d,%d) = %v, want %v", c.extra, c.slots, c.mode, got, c.want)
		}
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMarkdown(t *testing.T) {
	th := DefaultTheme()
	cases := map[string]string{
		"# Title":                     "Title",
		"plain **bold** and `code`":   "plain bold and code",
		"- item":                      "• item",
		" - not a bullet":             " - not a bullet",
		"[link](https://x.io)":        "link (https://x.io)",
		"> quoted":                    "│ quoted",
		"a very long line that wraps": "a very long\nline that\nwraps",
	}
	for in, want := range cases {
		width := 40
		if in == "a very long line that wraps" {
			width = 12
		}
		got := ansi.Strip(Markdown(in, width, th, th.Body))
		if got != want {
			t.Errorf("Markdown(%q) = %q, want %q", in, got, want)
		}
	}
	if got := ansi.Strip(Markdown("**bold** caption", 40, th, th.Caption)); got != "bold caption" {
		t.Errorf("styled base strips markers: %q", got)
	}
}

func TestBasicRegistry(t *testing.T) {
	r := Basic()
	names := r.Names()
	if len(names) != 18 {
		t.Errorf("basic catalog has %d components, want 18: %v", len(names), names)
	}
	for _, n := range []string{"Text", "Row", "Column", "List", "Card", "Tabs", "Modal", "Button", "TextField", "CheckBox", "ChoicePicker", "Slider", "DateTimeInput", "Icon", "Image", "Video", "AudioPlayer", "Divider"} {
		if _, ok := r.Lookup(n); !ok {
			t.Errorf("missing %s", n)
		}
	}
	for _, n := range []string{"Button", "TextField", "CheckBox", "ChoicePicker", "Slider", "DateTimeInput", "Tabs"} {
		c, _ := r.Lookup(n)
		if _, ok := c.(Focusable); !ok {
			t.Errorf("%s should be focusable", n)
		}
	}
	var modal Component = Modal{}
	if _, ok := modal.(Overlay); !ok {
		t.Error("Modal should implement Overlay")
	}
}
