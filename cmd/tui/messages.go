package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/s-588/mesh-network/internal/ipc"
)

type LogMsg string

type tablesRefreshMsg struct{}

func refreshTablesCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return tablesRefreshMsg{}
	})
}

type logsFetchedMsg struct {
	entries []ipc.LogEntryDTO
}

// logsTickMsg fires every 750 ms; each one fetches new logs since lastLogID.
type logsTickMsg struct{}

func logsTickCmd() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(_ time.Time) tea.Msg {
		return logsTickMsg{}
	})
}

func fetchLogsCmd(since uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		entries, err := ipc.GetLogs(ctx, since)
		if err != nil {
			return logsFetchedMsg{}
		}
		return logsFetchedMsg{entries: entries}
	}
}
