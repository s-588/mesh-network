package tui

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

var (
	borderColor = lipgloss.Color("#ff94ee")
	accent      = lipgloss.Color("#7cffd3")
	accent2     = lipgloss.Color("#b967ff")
	warn        = lipgloss.Color("#f59e0b")
	textColor   = lipgloss.Color("#e5e7eb")
	muted       = lipgloss.Color("#94a3b8")

	titleStyle = lipgloss.NewStyle().
			Foreground(accent2).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(muted).
			Bold(true)

	focusFieldStyle = lipgloss.NewStyle().
			Foreground(textColor).
		// Background(panelAltBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1).
		Width(40) // будет переопределяться

	blurFieldStyle = lipgloss.NewStyle().
			Foreground(textColor).
		// Background(panelBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
		// Background(panelBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 1)

	leftPanelStyle = panelStyle.
			Width(45).
			Align(lipgloss.Left)

	rightPanelStyle = panelStyle.
			BorderForeground(accent2).
			Width(50)

	sendButtonFocused = lipgloss.NewStyle().
				Foreground(warn).
		// Background(warn).
		Bold(true).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(warn)

	sendButtonBlurred = lipgloss.NewStyle().
				Foreground(textColor).
		// Background(panelAltBg).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	resultHeaderStyle = lipgloss.NewStyle().
				Foreground(accent).
				Bold(true)

	appStyle = lipgloss.NewStyle().
		// Background(appBg).
		Foreground(textColor)
)

func tableStyles() table.Styles {
	s := table.DefaultStyles()

	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Bold(true).
		Foreground(accent)

	s.Selected = s.Selected.
		Foreground(textColor).
		Background(accent2).
		Bold(true)

	return s
}
