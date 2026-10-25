package catalog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
)

// Image renders a labelled placeholder box; inline terminal images are not
// supported yet.
type Image struct{}

// Render implements Component.
func (Image) Render(f Frame, n a2ui.Node, width int) string {
	ev := f.Eval()
	url := ev.String(n.Component.Props["url"], n.Scope)
	desc := ev.String(n.Component.Props["description"], n.Scope)
	switch n.Component.StringProp("variant", "mediumFeature") {
	case "icon":
		return "▣"
	case "avatar":
		if desc == "" {
			desc = "avatar"
		}
		return "(" + ansi.Truncate(desc, 12, "…") + ")"
	}
	label := "image"
	if desc != "" {
		label = desc
	}
	variant := n.Component.StringProp("variant", "mediumFeature")
	pad := 0
	if variant == "largeFeature" || variant == "header" {
		pad = 1
	}
	return placeholderBox(f.Theme(), width, boxWidth(variant, width), pad, "▣ "+label, url)
}

func boxWidth(variant string, width int) int {
	switch variant {
	case "smallFeature":
		return min(width, 20)
	case "largeFeature", "header":
		return Bounded(width, 60)
	}
	return min(width, 40)
}

// Video renders a labelled placeholder box.
type Video struct{}

// Render implements Component.
func (Video) Render(f Frame, n a2ui.Node, width int) string {
	ev := f.Eval()
	url := ev.String(n.Component.Props["url"], n.Scope)
	lines := []string{"▶ video"}
	if poster := ev.String(n.Component.Props["posterUrl"], n.Scope); poster != "" {
		lines = append(lines, "poster: "+poster)
	}
	return placeholderBox(f.Theme(), width, min(width, 40), 2, strings.Join(lines, "\n"), url)
}

// AudioPlayer renders a labelled placeholder with a static control bar.
type AudioPlayer struct{}

// Render implements Component.
func (AudioPlayer) Render(f Frame, n a2ui.Node, width int) string {
	ev := f.Eval()
	url := ev.String(n.Component.Props["url"], n.Scope)
	desc := ev.String(n.Component.Props["description"], n.Scope)
	if desc == "" {
		desc = "audio"
	}
	w := min(width, 40)
	bar := "[▶] " + strings.Repeat("─", max(1, w-12)) + " 0:00"
	return placeholderBox(f.Theme(), width, w, 0, "♪ "+desc+"\n"+bar, url)
}

// placeholderBox draws a bordered box of the given inner width with a label
// and a truncated URL line.
func placeholderBox(th *Theme, width, boxW, pad int, label, url string) string {
	if boxW < 8 {
		boxW = min(8, max(width, 4))
	}
	inner := boxW - 4
	if inner < 1 {
		inner = 1
	}
	var body []string
	for i := 0; i < pad; i++ {
		body = append(body, "")
	}
	for _, l := range strings.Split(label, "\n") {
		body = append(body, ansi.Truncate(l, inner, "…"))
	}
	if url != "" {
		body = append(body, th.Muted.Render(ansi.Truncate(url, inner, "…")))
	}
	for i := 0; i < pad; i++ {
		body = append(body, "")
	}
	content := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center).Render(strings.Join(body, "\n"))
	return th.Border.Border(lipgloss.NormalBorder()).Padding(0, 1).Render(content)
}
