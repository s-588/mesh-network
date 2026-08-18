// package style contains lipgloss styles for other packages.
package style

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

var (
	BorderColor = lipgloss.Color("#ff94ee")
	Accent      = lipgloss.Color("#7cffd3")
	Accent2     = lipgloss.Color("#b967ff")
	Warn        = lipgloss.Color("#f59e0b")
	TextColor   = lipgloss.Color("#e5e7eb")
	Muted       = lipgloss.Color("#94a3b8")

	TitleStyle = lipgloss.NewStyle().
			Foreground(Accent2).
			Bold(true)

	LabelStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Bold(true)

	FocusFieldStyle = lipgloss.NewStyle().
			Foreground(TextColor).
		// Background(panelAltBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Accent).
		Padding(0, 1).
		Width(40) // будет переопределяться

	BlurFieldStyle = lipgloss.NewStyle().
			Foreground(TextColor).
		// Background(panelBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1)

	PanelStyle = lipgloss.NewStyle().
		// Background(panelBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(1, 1)

	LeftPanelStyle = PanelStyle.
			Width(45).
			Align(lipgloss.Left)

	RightPanelStyle = PanelStyle.
			BorderForeground(Accent2).
			Width(50)

	SendButtonFocused = lipgloss.NewStyle().
				Foreground(Warn).
		// Background(warn).
		Bold(true).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Warn)

	SendButtonBlurred = lipgloss.NewStyle().
				Foreground(TextColor).
		// Background(panelAltBg).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor)

	ResultHeaderStyle = lipgloss.NewStyle().
				Foreground(Accent).
				Bold(true)

	AppStyle = lipgloss.NewStyle().
		// Background(appBg).
		Foreground(TextColor)
)

func TableStyles() table.Styles {
	s := table.DefaultStyles()

	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Bold(true).
		Foreground(Accent)

	s.Selected = s.Selected.
		Foreground(TextColor).
		Background(Accent2).
		Bold(true)

	return s
}
