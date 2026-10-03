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
	"github.com/s-588/mesh-network/cmd/style"
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

	nodeID uint64
	ifaces []string

	sock *socket.Socket

	leftWidth      int
	rightWidth     int
	inputWidth     int
	textareaWidth  int
	viewportWidth  int
	viewportHeight int
}

func InitialModel(nodeID uint64, ifaces []string, logChan <-chan string, sock *socket.Socket) model {
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

	neighboursTable.SetStyles(style.TableStyles())
	routesTable.SetStyles(style.TableStyles())

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

	m.inputWidth = min(innerWidth-4, 50)

	m.textareaWidth = innerWidth - 2

	// Viewport
	m.viewportWidth = m.rightWidth - 4
	m.viewportHeight = m.topSectionHeight - 8

	tableHeight := max((m.height-m.topSectionHeight-10)/2, 5)
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

//nolint:funlen // View is long due to layout code, but it's clear and readable.
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
		m.resultViewport.GotoBottom()
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
			return processEscPress(m)

		case "tab":
			m.nextFocus()

		case "shift+tab":
			m.prevFocus()

		case "enter":
			return processEnterPress(m), nil
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

func processEnterPress(m model) tea.Model {
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
			return m
		}
		payload := []byte(m.payloadAreaInput.Value())
		m.sock.SendData(nodeID, payload)
		m.payloadAreaInput.SetValue("")
	case 3:
		m.resultViewport.GotoBottom()
	}
	return m
}

func processEscPress(m model) (tea.Model, tea.Cmd) {
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
	return m, nil
}

func (m model) View() (view tea.View) {
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	if len(m.logs) > 0 {
		m.resultViewport.SetContent(strings.Join(m.logs, "\n"))
	}

	layout := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderTopLayout(),
		"",
		m.renderNeighboursBlock(),
		"",
		m.renderRoutesBlock(),
	)

	view.SetContent(style.AppStyle.Width(m.width).Render(layout))
	return view
}

func (m model) renderNodeIDInput() string {
	nodeStyle := style.BlurFieldStyle
	if m.focused == 0 {
		nodeStyle = style.FocusFieldStyle
	}
	return nodeStyle.Width(m.inputWidth + 2).Render(m.nodeIDTextInput.View())
}

func (m model) renderSendButton() string {
	btnStyle := style.SendButtonBlurred
	if m.focused == 2 {
		btnStyle = style.SendButtonFocused
	}
	return btnStyle.Render(m.sendButton)
}

func (m model) renderTopRow() string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		m.renderNodeIDInput(),
		lipgloss.NewStyle().PaddingLeft(2).Render(m.renderSendButton()),
	)
}

func (m model) renderPayloadInput() string {
	payloadStyle := style.BlurFieldStyle
	if m.focused == 1 {
		payloadStyle = style.FocusFieldStyle
	}
	return payloadStyle.Width(m.textareaWidth + 2).Render(m.payloadAreaInput.View())
}

func (m model) renderLeftContent() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.TitleStyle.Render(fmt.Sprintf("Node ID: %d | Active interfaces: %s", m.nodeID, strings.Join(m.ifaces, ", "))),
		m.renderTopRow(),
		m.renderPayloadInput(),
	)
}

func (m model) renderLeftPanel() string {
	return style.LeftPanelStyle.
		Width(m.leftWidth).
		Height(m.topSectionHeight).
		Render(m.renderLeftContent())
}

func (m model) renderRightContent() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.ResultHeaderStyle.Render("Result / Logs"),
		m.resultViewport.View(),
	)
}

func (m model) renderRightPanel() string {
	return style.RightPanelStyle.
		Width(m.rightWidth).
		Height(m.topSectionHeight).
		Render(m.renderRightContent())
}

func (m model) renderTopLayout() string {
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderLeftPanel(),
		lipgloss.NewStyle().PaddingLeft(1).Render(m.renderRightPanel()),
	)
}

func (m model) renderNeighboursBlock() string {
	return style.PanelStyle.
		Width(m.width - 2).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				style.ResultHeaderStyle.Render("Neighbours Table"),
				m.neighboursTable.View(),
			),
		)
}

func (m model) renderRoutesBlock() string {
	return style.PanelStyle.
		Width(m.width - 2).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				style.ResultHeaderStyle.Render("Routes Table"),
				m.routesTable.View(),
			),
		)
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
