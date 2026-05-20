package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/internal/socket"
)

type model struct {
	width  int
	height int

	nodeIDTextInput  textinput.Model
	payloadAreaInput textarea.Model
	sendButton       string
	resultViewport   viewport.Model

	focused int
	logs    []string
	logChan <-chan string

	nodeID int
	ifaces []string

	sock *socket.Socket

	leftWidth      int
	rightWidth     int
	inputWidth     int
	textareaWidth  int
	viewportWidth  int
	viewportHeight int
}

func InitialModel(nodeID int, ifaces []string, logChan <-chan string, sock *socket.Socket) model {
	ti := textinput.New()
	ti.Placeholder = "Enter ID of receiver node..."
	ti.Focus()
	ti.CharLimit = 50

	ta := textarea.New()
	ta.Placeholder = "Write message to receiver..."
	ta.SetHeight(5)

	vp := viewport.New()
	vp.SetContent("Results will appear here.")

	return model{
		nodeIDTextInput:  ti,
		payloadAreaInput: ta,
		resultViewport:   vp,
		sendButton:       " Send ",
		focused:          0,
		nodeID:           nodeID,
		ifaces:           ifaces,
		logs:             make([]string, 0),
		logChan:          logChan,
		sock:             sock,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, listenForLogs(m.logChan))
}

// updateSizes — центральня функция адаптивности
func (m *model) updateSizes(width, height int) {
	m.width = width
	m.height = height

	if m.width < 60 {
		m.width = 60
	}
	if m.height < 20 {
		m.height = 20
	}

	// Пропорции: ~45% левая панель, остальное — правая
	m.leftWidth = (m.width * 45) / 100
	m.rightWidth = m.width - m.leftWidth - 3 // 3 — отступ между панелями

	// Минимальные размеры
	if m.leftWidth < 45 {
		m.leftWidth = 45
		m.rightWidth = m.width - m.leftWidth - 3
	}
	if m.rightWidth < 35 {
		m.rightWidth = 35
		m.leftWidth = m.width - m.rightWidth - 3
	}

	// Внутренние размеры с учётом padding и border
	innerWidth := m.leftWidth - 4 // padding + border

	m.inputWidth = innerWidth - 4
	if m.inputWidth > 50 {
		m.inputWidth = 50
	}

	m.textareaWidth = innerWidth - 2

	// Viewport
	m.viewportWidth = m.rightWidth - 4
	vpHeight := m.height - 16 // заголовки, отступы
	if vpHeight < 8 {
		vpHeight = 8
	}
	m.viewportHeight = vpHeight
}

func (m *model) nextFocus() {
	m.blurCurrent()

	m.focused = (m.focused + 1) % 4

	switch m.focused {
	case 0:
		m.nodeIDTextInput.Focus()
	case 1:
		m.payloadAreaInput.Focus()
	case 2, 3:
		// buttons/viewport don't need focus
	}
}

func (m *model) prevFocus() {
	m.blurCurrent()

	m.focused = (m.focused + 3) % 4 // -1 mod 4

	switch m.focused {
	case 0:
		m.nodeIDTextInput.Focus()
	case 1:
		m.payloadAreaInput.Focus()
	}
}

func (m *model) blurCurrent() {
	switch m.focused {
	case 0:
		m.nodeIDTextInput.Blur()
	case 1:
		m.payloadAreaInput.Blur()
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case LogMsg:
		m.logs = append(m.logs, string(msg))
		m.resultViewport.SetContent(strings.Join(m.logs, "\n"))
		m.resultViewport.GotoBottom() // ← This was the main request
		return m, listenForLogs(m.logChan)

	case tea.WindowSizeMsg:
		m.updateSizes(msg.Width, msg.Height)

		m.nodeIDTextInput.SetWidth(m.inputWidth)
		m.payloadAreaInput.SetWidth(m.textareaWidth)
		m.resultViewport.SetWidth(m.viewportWidth)
		m.resultViewport.SetHeight(m.viewportHeight)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			// Simple behavior: blur current input or quit
			switch m.focused {
			case 0:
				if m.nodeIDTextInput.Focused() {
					m.nodeIDTextInput.Blur()
				} else {
					return m, tea.Quit
				}
			case 1:
				if m.payloadAreaInput.Focused() {
					m.payloadAreaInput.Blur()
				} else {
					return m, tea.Quit
				}
			default:
				return m, tea.Quit
			}

		case "tab":
			m.nextFocus()

		case "shift+tab":
			m.prevFocus()

		case "enter":
			switch m.focused {
			case 0: // node ID
				if m.nodeIDTextInput.Focused() {
					m.nodeIDTextInput.Blur()
					m.focused = 1
					m.payloadAreaInput.Focus()
				}
			case 1: // payload
				if m.payloadAreaInput.Focused() {
					m.payloadAreaInput.Blur()
					m.focused = 2
				}
			case 2: // send button
				nodeID, err := strconv.ParseUint(m.nodeIDTextInput.Value(), 10, 64)
				if err != nil {
					m.logs = append(m.logs, "Error: invalid node ID")
					m.resultViewport.SetContent(strings.Join(m.logs, "\n"))
					m.resultViewport.GotoBottom()
					return m, nil
				}
				payload := []byte(m.payloadAreaInput.Value())
				m.sock.SendData(nodeID, payload)
				m.payloadAreaInput.SetValue("")
			case 3:
				m.resultViewport.GotoBottom()
			}
		}
	}

	// Update focused component
	var cmd tea.Cmd
	switch {
	case m.nodeIDTextInput.Focused():
		m.nodeIDTextInput, cmd = m.nodeIDTextInput.Update(msg)
		cmds = append(cmds, cmd)
	case m.payloadAreaInput.Focused():
		m.payloadAreaInput, cmd = m.payloadAreaInput.Update(msg)
		cmds = append(cmds, cmd)
	}
		m.resultViewport, cmd = m.resultViewport.Update(msg)
		cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() (view tea.View) {
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	// No need for the old view.AltScreen etc. in newer Bubble Tea, just return string
	if len(m.logs) > 0 {
		m.resultViewport.SetContent(strings.Join(m.logs, "\n"))
	}

	// LEFT PANEL
	nodeStyle := blurFieldStyle
	if m.focused == 0 {
		nodeStyle = focusFieldStyle
	}
	nodeIDView := nodeStyle.Width(m.inputWidth + 2).Render(m.nodeIDTextInput.View())

	btnStyle := sendButtonBlurred
	if m.focused == 2 {
		btnStyle = sendButtonFocused
	}
	sendBtn := btnStyle.Render(m.sendButton)

	topRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		nodeIDView,
		lipgloss.NewStyle().PaddingLeft(2).Render(sendBtn),
	)

	payloadStyle := blurFieldStyle
	if m.focused == 1 {
		payloadStyle = focusFieldStyle
	}
	payloadView := payloadStyle.Width(m.textareaWidth + 2).Render(m.payloadAreaInput.View())

	leftContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(fmt.Sprintf("Node ID: %d | Active interfaces: %s", m.nodeID, strings.Join(m.ifaces, ", "))),
		topRow,
		payloadView,
	)

	leftPanel := leftPanelStyle.Width(m.leftWidth).Render(leftContent)

	// RIGHT PANEL
	rightContent := lipgloss.JoinVertical(
		lipgloss.Left,
		resultHeaderStyle.Render("Result / Logs"),
		m.resultViewport.View(),
	)
	rightPanel := rightPanelStyle.Width(m.rightWidth).Render(rightContent)

	layout := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		lipgloss.NewStyle().PaddingLeft(1).Render(rightPanel),
	)

	view.SetContent(appStyle.Width(m.width).Render(layout))
	return view
}
