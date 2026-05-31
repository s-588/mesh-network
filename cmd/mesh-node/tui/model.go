package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/internal/routing"
	"github.com/s-588/mesh-network/internal/socket"
)

const (
	focusNodeInput = iota
	focusPayload
	focusSendButton
	focusLogs
	focusNeighboursTable
	focusRoutesTable
)

type model struct {
	width            int
	height           int
	topSectionHeight int

	nodeIDTextInput  textinput.Model
	payloadAreaInput textarea.Model
	sendButton       string
	resultViewport   viewport.Model
	neighboursTable  table.Model
	routesTable      table.Model

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

	neighboursColumns := []table.Column{
		{Title: "ID", Width: 10},
		{Title: "Address", Width: 24},
		{Title: "Last Seen", Width: 20},
		{Title: "Iface", Width: 12},
	}

	neighboursTable := table.New(
		table.WithColumns(neighboursColumns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(6),
	)

	routesColumns := []table.Column{
		{Title: "Dst", Width: 10},
		{Title: "Seq", Width: 8},
		{Title: "Hops", Width: 6},
		{Title: "Next Hop", Width: 12},
		{Title: "Address", Width: 24},
		{Title: "Iface", Width: 10},
	}

	routesTable := table.New(
		table.WithColumns(routesColumns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(8),
	)

	neighboursTable.SetStyles(tableStyles())
	routesTable.SetStyles(tableStyles())

	return model{
		nodeIDTextInput:  ti,
		payloadAreaInput: ta,
		resultViewport:   vp,
		neighboursTable:  neighboursTable,
		routesTable:      routesTable,
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
	return tea.Batch(
		textinput.Blink,
		listenForLogs(m.logChan),
		refreshTablesCmd(),
	)
}

// updateSizes — центральня функция адаптивности
func (m *model) updateSizes(width, height int) {
	m.width = width
	m.height = height
	m.topSectionHeight = (m.height * 45) / 100

	if m.width < 60 {
		m.width = 60
	}
	if m.height < 20 {
		m.height = 20
	}
	if m.topSectionHeight < 14 {
		m.topSectionHeight = 14
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
	m.viewportHeight = m.topSectionHeight - 8

	tableHeight := (m.height - m.topSectionHeight - 10) / 2
	if tableHeight < 5 {
		tableHeight = 5
	}
	m.neighboursTable.SetHeight(tableHeight)
	m.routesTable.SetHeight(tableHeight)
}

func (m *model) nextFocus() {
	m.blurCurrent()
	m.focused = (m.focused + 1) % 6
	m.applyFocus()
}

func (m *model) prevFocus() {
	m.blurCurrent()
	m.focused = (m.focused + 5) % 6
	m.applyFocus()
}

func (m *model) applyFocus() {
	switch m.focused {
	case focusNodeInput:
		m.nodeIDTextInput.Focus()

	case focusPayload:
		m.payloadAreaInput.Focus()

	case focusNeighboursTable:
		m.neighboursTable.Focus()

	case focusRoutesTable:
		m.routesTable.Focus()
	}
}

func (m *model) blurCurrent() {
	m.nodeIDTextInput.Blur()
	m.payloadAreaInput.Blur()

	m.neighboursTable.Blur()
	m.routesTable.Blur()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tablesRefreshMsg:
		m.refreshNeighboursTable()
		m.refreshRoutesTable()
		return m, refreshTablesCmd()

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
		m.neighboursTable.SetWidth(m.width - 6)
		m.routesTable.SetWidth(m.width - 6)

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

	m.neighboursTable, cmd = m.neighboursTable.Update(msg)
	cmds = append(cmds, cmd)

	m.routesTable, cmd = m.routesTable.Update(msg)
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

	leftPanel := leftPanelStyle.
		Width(m.leftWidth).
		Height(m.topSectionHeight).
		Render(leftContent)

	// RIGHT PANEL
	rightContent := lipgloss.JoinVertical(
		lipgloss.Left,
		resultHeaderStyle.Render("Result / Logs"),
		m.resultViewport.View(),
	)
	rightPanel := rightPanelStyle.
		Width(m.rightWidth).
		Height(m.topSectionHeight).
		Render(rightContent)

	topLayout := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		lipgloss.NewStyle().PaddingLeft(1).Render(rightPanel),
	)

	neighboursBlock := panelStyle.
		Width(m.width - 2).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				resultHeaderStyle.Render("Neighbours Table"),
				m.neighboursTable.View(),
			),
		)

	routesBlock := panelStyle.
		Width(m.width - 2).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				resultHeaderStyle.Render("Routes Table"),
				m.routesTable.View(),
			),
		)

	layout := lipgloss.JoinVertical(
		lipgloss.Left,
		topLayout,
		"",
		neighboursBlock,
		"",
		routesBlock,
	)

	view.SetContent(appStyle.Width(m.width).Render(layout))
	return view
}

func (m *model) refreshNeighboursTable() {
	entries := routing.NeighboursTable.Snapshot()

	rows := make([]table.Row, 0, len(entries))

	for _, e := range entries {
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", e.ID),
			e.Addr.String(),
			e.LastSeen.Format("15:04:05"),
			e.Interface,
		})
	}

	m.neighboursTable.SetRows(rows)
}

func (m *model) refreshRoutesTable() {
	entries := routing.RoutesTable.Snapshot()

	rows := make([]table.Row, 0, len(entries))

	for _, e := range entries {
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", e.DstID),
			fmt.Sprintf("%d", e.DstSeq),
			fmt.Sprintf("%d", e.HopCount),
			fmt.Sprintf("%d", e.NextHopID),
			e.NextHopAddr.String(),
			e.Interface,
		})
	}

	m.routesTable.SetRows(rows)
}
