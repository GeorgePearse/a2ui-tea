package catalog

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
)

// Text renders a Text component with light Markdown support.
type Text struct{}

// Render implements Component.
func (Text) Render(f Frame, n a2ui.Node, width int) string {
	th := f.Theme()
	raw := f.Eval().String(n.Component.Props["text"], n.Scope)
	base := th.Body
	if n.Component.StringProp("variant", "body") == "caption" {
		base = th.Caption
	}
	return Markdown(raw, width, th, base)
}

var (
	boldRe   = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	italicRe = regexp.MustCompile(`(^|[^*\w])\*([^*\s][^*]*?)\*|(^|[^_\w])_([^_\s][^_]*?)_`)
	codeRe   = regexp.MustCompile("`([^`]+)`")
	linkRe   = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// Markdown renders a small Markdown subset (headings, bold, italic, code,
// links, bullet lists) and wraps the result to width.
func Markdown(src string, width int, th *Theme, base lipgloss.Style) string {
	if width < 1 {
		width = 1
	}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		style := base
		styled := base.GetBold() || base.GetFaint() || base.GetItalic()
		trimmed := strings.TrimLeft(line, " ")
		switch {
		case strings.HasPrefix(trimmed, "#"):
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level < len(trimmed) && trimmed[level] == ' ' {
				line = strings.TrimSpace(trimmed[level:])
				style, styled = th.Heading, true
			}
		case line == trimmed && (strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")):
			line = "• " + line[2:]
		case strings.HasPrefix(trimmed, "> "):
			line = "│ " + trimmed[2:]
			style, styled = th.Muted, true
		case trimmed == "---" || trimmed == "***":
			line = strings.Repeat("─", Bounded(width, 3))
			style, styled = th.Border, true
		}
		if styled {
			out = append(out, wrap(style.Render(plain(line)), width))
		} else {
			out = append(out, wrap(inline(line, th), width))
		}
	}
	return strings.Join(out, "\n")
}

// plain strips inline Markdown markers without styling, for lines that
// already carry a block style (styles cannot nest).
func plain(s string) string {
	s = codeRe.ReplaceAllString(s, "$1")
	s = linkRe.ReplaceAllString(s, "$1 ($2)")
	s = boldRe.ReplaceAllString(s, "$1$2")
	s = italicRe.ReplaceAllString(s, "$1$2$3$4")
	return s
}

func inline(s string, th *Theme) string {
	s = codeRe.ReplaceAllStringFunc(s, func(m string) string {
		return th.Code.Render(m[1 : len(m)-1])
	})
	s = linkRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := linkRe.FindStringSubmatch(m)
		return th.Link.Render(sub[1]) + th.Muted.Render(" ("+sub[2]+")")
	})
	s = boldRe.ReplaceAllStringFunc(s, func(m string) string {
		return th.Heading.Render(m[2 : len(m)-2])
	})
	s = italicRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := italicRe.FindStringSubmatch(m)
		if sub[2] != "" {
			return sub[1] + lipgloss.NewStyle().Italic(true).Render(sub[2])
		}
		return sub[3] + lipgloss.NewStyle().Italic(true).Render(sub[4])
	})
	return s
}

func wrap(s string, width int) string {
	if width < 1 {
		return s
	}
	return ansi.Wrap(s, width, "")
}

// Icon renders a named icon as a glyph.
type Icon struct{}

// Render implements Component.
func (Icon) Render(f Frame, n a2ui.Node, width int) string {
	raw, _ := n.Component.Prop("name")
	if m, ok := raw.(map[string]any); ok {
		if _, ok := m["svgPath"]; ok {
			return "◆"
		}
	}
	name := f.Eval().String(raw, n.Scope)
	if g, ok := Glyphs[name]; ok {
		return g
	}
	if name == "" {
		return "·"
	}
	return "[" + name + "]"
}

// Divider renders a horizontal or vertical rule.
type Divider struct{}

// Render implements Component.
func (Divider) Render(f Frame, n a2ui.Node, width int) string {
	if n.Component.StringProp("axis", "horizontal") == "vertical" {
		return f.Theme().Border.Render("│")
	}
	return f.Theme().Border.Render(strings.Repeat("─", max(1, Bounded(width, 3))))
}

// Glyphs maps basic catalog icon names to terminal glyphs.
var Glyphs = map[string]string{
	"accountCircle": "◉", "add": "+", "arrowBack": "←", "arrowForward": "→", "attachFile": "📎",
	"calendarToday": "📅", "call": "☎", "camera": "📷", "check": "✓", "close": "✕", "delete": "🗑",
	"download": "⤓", "edit": "✎", "event": "📅", "error": "⊗", "fastForward": "⏩", "favorite": "♥",
	"favoriteOff": "♡", "folder": "📁", "help": "?", "home": "⌂", "info": "ⓘ", "locationOn": "📍",
	"lock": "🔒", "lockOpen": "🔓", "mail": "✉", "menu": "☰", "moreVert": "⋮", "moreHoriz": "…",
	"notificationsOff": "🔕", "notifications": "🔔", "pause": "⏸", "payment": "💳", "person": "👤",
	"phone": "☎", "photo": "🖼", "play": "▶", "print": "⎙", "refresh": "↻", "rewind": "⏪",
	"search": "🔍", "send": "➤", "settings": "⚙", "share": "⇪", "shoppingCart": "🛒", "skipNext": "⏭",
	"skipPrevious": "⏮", "star": "★", "starHalf": "⯨", "starOff": "☆", "stop": "■", "upload": "⤒",
	"visibility": "👁", "visibilityOff": "⊘", "volumeDown": "🔉", "volumeMute": "🔇", "volumeOff": "🔇",
	"volumeUp": "🔊", "warning": "⚠",
}
