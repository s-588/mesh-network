package tui

import tea "charm.land/bubbletea/v2"

type LogMsg string

func listenForLogs(logChan <-chan string) tea.Cmd {
	return func() tea.Msg {
		for logMsg := range logChan {
			// This loop blocks in the background goroutine,
			// waiting for new logs, then sends them to the UI.
			return LogMsg(logMsg)
		}
		return nil
	}
}
