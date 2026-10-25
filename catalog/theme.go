package catalog

import "charm.land/lipgloss/v2"

// Theme holds the styles the basic components use.
type Theme struct {
	Body           lipgloss.Style
	Heading        lipgloss.Style
	Caption        lipgloss.Style
	Muted          lipgloss.Style
	Code           lipgloss.Style
	Label          lipgloss.Style
	Error          lipgloss.Style
	Border         lipgloss.Style
	Card           lipgloss.Style
	Button         lipgloss.Style
	ButtonPrimary  lipgloss.Style
	ButtonFocused  lipgloss.Style
	ButtonDisabled lipgloss.Style
	Link           lipgloss.Style
	TabActive      lipgloss.Style
	TabInactive    lipgloss.Style
	Focus          lipgloss.Style
	Modal          lipgloss.Style
	Placeholder    lipgloss.Style
	Accent         lipgloss.Style
}

// DefaultTheme returns a theme that reads well on dark and light terminals.
func DefaultTheme() *Theme {
	accent := lipgloss.Color("6")
	primary := lipgloss.Color("4")
	grey := lipgloss.Color("8")
	red := lipgloss.Color("1")
	return &Theme{
		Body:           lipgloss.NewStyle(),
		Heading:        lipgloss.NewStyle().Bold(true),
		Caption:        lipgloss.NewStyle().Faint(true).Italic(true),
		Muted:          lipgloss.NewStyle().Faint(true),
		Code:           lipgloss.NewStyle().Foreground(accent),
		Label:          lipgloss.NewStyle().Bold(true),
		Error:          lipgloss.NewStyle().Foreground(red),
		Border:         lipgloss.NewStyle().Foreground(grey),
		Card:           lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(grey).Padding(0, 1),
		Button:         lipgloss.NewStyle(),
		ButtonPrimary:  lipgloss.NewStyle().Bold(true).Foreground(primary),
		ButtonFocused:  lipgloss.NewStyle().Reverse(true),
		ButtonDisabled: lipgloss.NewStyle().Faint(true),
		Link:           lipgloss.NewStyle().Underline(true).Foreground(primary),
		TabActive:      lipgloss.NewStyle().Bold(true).Underline(true),
		TabInactive:    lipgloss.NewStyle().Faint(true),
		Focus:          lipgloss.NewStyle().Foreground(accent).Bold(true),
		Modal:          lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(accent).Padding(0, 1),
		Placeholder:    lipgloss.NewStyle().Faint(true),
		Accent:         lipgloss.NewStyle().Foreground(accent),
	}
}
