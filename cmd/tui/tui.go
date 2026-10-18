package tui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/s-588/mesh-network/cmd/style"
	"github.com/s-588/mesh-network/internal/ipc"
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
	width  int
	height int
	mode   layoutMode
	dense  bool

	nodeIDTextInput  textinput.Model
	payloadAreaInput textarea.Model
	sendButton       string
	resultViewport   viewport.Model
	neighboursTable  table.Model
	routesTable      table.Model

	focused int

	logs      []string
	lastLogID uint64

	nodeID uint64
	ifaces []string
	sock   *socket.Socket

	// Box sizes include the panel frame. Widget sizes are the inner views.
	composeW, composeH int
	logsW, logsH       int
	neighW, neighH     int
	routesW, routesH   int

	inputWidth       int
	textareaWidth    int
	textareaHeight   int
	viewportWidth    int
	viewportHeight   int
	neighTableW      int
	neighTableH      int
	routeTableW      int
	routeTableH      int
	stackSend        bool
	slimButton       bool
	hideComposeTitle bool
	showHint         bool

	neighbours []neighbour
	routes     []routeEntry
}

func InitialModel(nodeID uint64, ifaces []string, sock *socket.Socket) model {
	ti := textinput.New()
	ti.Placeholder = "Receiver node ID..."
	ti.Focus()
	ti.CharLimit = 50

	ta := textarea.New()
	ta.Placeholder = "Write message to receiver..."
	ta.SetHeight(3)

	vp := viewport.New()
	vp.SetContent("Connected to background node via IPC.\nResults and logs will appear here.")

	neighboursTable := table.New(
		table.WithColumns([]table.Column{{Title: "ID", Width: 4}}),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(3),
	)
	routesTable := table.New(
		table.WithColumns([]table.Column{{Title: "Dst", Width: 4}}),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(3),
	)
	neighboursTable.SetStyles(style.TableStyles())
	routesTable.SetStyles(style.TableStyles())

	m := model{
		nodeIDTextInput:  ti,
		payloadAreaInput: ta,
		resultViewport:   vp,
		neighboursTable:  neighboursTable,
		routesTable:      routesTable,
		sendButton:       " Send ",
		focused:          focusNodeInput,
		nodeID:           nodeID,
		ifaces:           ifaces,
		logs:             make([]string, 0),
		sock:             sock,
	}
	m.updateSizes(80, 24)
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		fetchLogsCmd(0),
		logsTickCmd(),
		refreshTablesCmd(),
	)
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

func (m *model) focusSection(sec int) {
	m.blurCurrent()
	switch sec {
	case 1:
		m.focused = focusLogs
	case 2:
		m.focused = focusNeighboursTable
	case 3:
		m.focused = focusRoutesTable
	default:
		m.focused = focusNodeInput
	}
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

func (m model) typing() bool {
	return m.nodeIDTextInput.Focused() || m.payloadAreaInput.Focused()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tablesRefreshMsg:
		m.refreshNeighboursTable()
		m.refreshRoutesTable()
		return m, refreshTablesCmd()

	case logsTickMsg:
		cmds = append(cmds, fetchLogsCmd(m.lastLogID), logsTickCmd())

	case logsFetchedMsg:
		if len(msg.entries) > 0 {
			for _, e := range msg.entries {
				m.logs = append(m.logs, e.Line)
				m.lastLogID = e.ID
			}
			m.reloadLogs()
		}

	case tea.WindowSizeMsg:
		m.updateSizes(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		var cmd tea.Cmd
		m, cmd = processKeyMsg(msg, m)
		cmds = append(cmds, cmd)
	}

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

func processKeyMsg(msg tea.KeyMsg, m model) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		return processEscPress(m)
	case "tab":
		m.nextFocus()
		return m, nil
	case "shift+tab":
		m.prevFocus()
		return m, nil
	case "enter":
		return processEnterPress(m), nil
	case "1", "2", "3", "4":
		// Digits still type into the fields. Outside of them they jump
		// pages, which is the only way to see every section when the
		// terminal is too small to show them all.
		if m.mode == layoutCompact && !m.typing() {
			m.focusSection(int(msg.String()[0] - '1'))
			return m, nil
		}
	}
	return m, nil
}

func processEnterPress(m model) model {
	switch m.focused {
	case focusNodeInput:
		if m.nodeIDTextInput.Focused() {
			m.nodeIDTextInput.Blur()
			m.focused = focusPayload
			m.payloadAreaInput.Focus()
		}
	case focusPayload:
		if m.payloadAreaInput.Focused() {
			m.payloadAreaInput.Blur()
			m.focused = focusSendButton
		}
	case focusSendButton:
		nodeID, err := strconv.ParseUint(m.nodeIDTextInput.Value(), 10, 64)
		if err != nil {
			m.pushLog("Error: invalid node ID")
			return m
		}
		payload := m.payloadAreaInput.Value()
		if err := ipc.CheckMsg(payload); err != nil {
			m.pushLog("Error: " + err.Error())
			return m
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res, err := ipc.SendMsg(ctx, nodeID, payload)
		if err != nil {
			m.pushLog(fmt.Sprintf("Send failed: %v", err))
			return m
		}
		switch {
		case res.Sent:
			m.pushLog(fmt.Sprintf("→ %d: sent immediately", nodeID))
		case res.Queued:
			m.pushLog(fmt.Sprintf("→ %d: queued (backlog %d) — awaiting RREP", nodeID, res.QueueLen))
		}
		m.payloadAreaInput.SetValue("")
	case focusLogs:
		m.resultViewport.GotoBottom()
	}
	return m
}

func processEscPress(m model) (model, tea.Cmd) {
	switch m.focused {
	case focusNodeInput:
		if m.nodeIDTextInput.Focused() {
			m.nodeIDTextInput.Blur()
		} else {
			return m, tea.Quit
		}
	case focusPayload:
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

func (m model) View() tea.View {
	var view tea.View
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion

	var body string
	switch m.mode {
	case layoutTiny:
		body = m.renderTiny()
	case layoutCompact:
		body = m.renderCompact()
	case layoutStack:
		body = m.renderStack()
	case layoutSplit:
		body = m.renderSplit()
	default:
		body = m.renderWide()
	}
	if m.width > 0 && m.height > 0 {
		body = style.AppStyle.MaxWidth(m.width).MaxHeight(m.height).Render(body)
	} else {
		body = style.AppStyle.Render(body)
	}
	view.SetContent(body)
	return view
}
