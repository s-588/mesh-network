package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/cmd/style"
)

func (m model) renderTiny() string {
	w := max(m.width, 1)
	h := max(m.height, 1)
	msg := fmt.Sprintf("Terminal too small (%d×%d)\n\nNeed at least %d×%d", m.width, m.height, tinyBelowWidth, tinyBelowHeight)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, style.LabelStyle.Render(msg))
}

func (m model) renderWide() string {
	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderPanel(m.composeW, m.composeH, false, m.renderComposeInner()),
		lipgloss.NewStyle().PaddingLeft(1).Render(
			m.renderPanel(m.logsW, m.logsH, true, m.renderLogsInner()),
		),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		top,
		"",
		m.renderNeighboursBlock(),
		"",
		m.renderRoutesBlock(),
	)
}

func (m model) renderSplit() string {
	tables := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderTablePanel(m.neighW, m.neighH, m.renderNeighInner()),
		lipgloss.NewStyle().PaddingLeft(1).Render(
			m.renderTablePanel(m.routesW, m.routesH, m.renderRoutesInner()),
		),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderPanel(m.composeW, m.composeH, false, m.renderComposeInner()),
		"",
		m.renderPanel(m.logsW, m.logsH, true, m.renderLogsInner()),
		"",
		tables,
	)
}

func (m model) renderStack() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderPanel(m.composeW, m.composeH, false, m.renderComposeInner()),
		"",
		m.renderPanel(m.logsW, m.logsH, true, m.renderLogsInner()),
		"",
		m.renderNeighboursBlock(),
		"",
		m.renderRoutesBlock(),
	)
}

func (m model) renderCompact() string {
	var body string
	switch m.compactSection() {
	case 1:
		body = m.renderPanel(m.logsW, m.logsH, true, m.renderLogsInner())
	case 2:
		body = m.renderTablePanel(m.neighW, m.neighH, m.renderNeighInner())
	case 3:
		body = m.renderTablePanel(m.routesW, m.routesH, m.renderRoutesInner())
	default:
		body = m.renderPanel(m.composeW, m.composeH, false, m.renderComposeInner())
	}
	parts := []string{m.renderTabBar(), body}
	if m.showHint {
		parts = append(parts, m.renderHint())
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) compactSection() int {
	switch m.focused {
	case focusLogs:
		return 1
	case focusNeighboursTable:
		return 2
	case focusRoutesTable:
		return 3
	default:
		return 0
	}
}

func (m model) renderTabBar() string {
	labels := []string{"Compose", "Logs", "Neighbours", "Routes"}
	if m.width < 64 {
		labels = []string{"Msg", "Logs", "Neigh", "Routes"}
	}
	if m.width < 40 {
		labels = []string{"Msg", "Log", "Nbr", "Rte"}
	}
	active := m.compactSection()
	parts := make([]string, len(labels))
	for i, label := range labels {
		if i == active {
			parts[i] = style.TitleStyle.Underline(true).Render(" " + label + " ")
		} else {
			parts[i] = style.LabelStyle.Render(" " + label + " ")
		}
	}
	return oneLine(lipgloss.JoinHorizontal(lipgloss.Left, parts...), m.width)
}

func (m model) renderHint() string {
	hint := "1-4 jump   tab focus   enter send   esc quit"
	if m.width < 52 {
		hint = "1-4 jump  tab  enter  esc"
	}
	return oneLine(style.LabelStyle.Render(hint), m.width)
}
