package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/cmd/style"
)

func (m model) titleText() string {
	suffix := ""
	if m.sock == nil {
		suffix = "  IPC"
	}
	ifaces := m.ifaces
	if len(ifaces) == 0 {
		ifaces = []string{"none"}
	}
	if m.width < 78 {
		return style.TitleStyle.Render(fmt.Sprintf("Node %d · %s%s", m.nodeID, strings.Join(ifaces, ","), suffix))
	}
	return style.TitleStyle.Render(fmt.Sprintf("Node ID: %d | Interfaces: %s%s", m.nodeID, strings.Join(ifaces, ", "), suffix))
}

func (m model) renderComposeInner() string {
	cw := max(m.composeW-frameX, 1)
	parts := make([]string, 0, 3)
	if !m.hideComposeTitle {
		parts = append(parts, oneLine(m.titleText(), cw))
	}
	parts = append(parts, m.renderTopRow(cw), m.renderPayloadInput())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) renderNodeIDInput() string {
	st := style.BlurFieldStyle
	if m.focused == focusNodeInput {
		st = style.FocusFieldStyle
	}
	w := m.inputWidth + fieldFrameX
	if w < fieldFrameX+1 {
		w = fieldFrameX + 1
	}
	return st.Width(w).Render(m.nodeIDTextInput.View())
}

func (m model) renderSendButton() string {
	if m.slimButton {
		label := " Send "
		s := style.LabelStyle
		if m.focused == focusSendButton {
			s = style.TitleStyle
			label = "[Send]"
		}
		return s.Render(label)
	}
	btn := style.SendButtonBlurred
	if m.focused == focusSendButton {
		btn = style.SendButtonFocused
	}
	return btn.Render(m.sendButton)
}

func (m model) renderTopRow(contentW int) string {
	input := m.renderNodeIDInput()
	btn := m.renderSendButton()
	if m.stackSend {
		return lipgloss.NewStyle().MaxWidth(contentW).Render(
			lipgloss.JoinVertical(lipgloss.Left, input, btn),
		)
	}
	return lipgloss.NewStyle().MaxWidth(contentW).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			input,
			lipgloss.NewStyle().PaddingLeft(buttonGap).Render(btn),
		),
	)
}

func (m model) renderPayloadInput() string {
	st := style.BlurFieldStyle
	if m.focused == focusPayload {
		st = style.FocusFieldStyle
	}
	w := m.textareaWidth + fieldFrameX
	if w < fieldFrameX+1 {
		w = fieldFrameX + 1
	}
	return st.Width(w).Render(m.payloadAreaInput.View())
}
