package tui

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/cmd/style"
)

func (m *model) pushLog(line string) {
	m.logs = append(m.logs, line)
	m.reloadLogs()
}

func (m *model) reloadLogs() {
	if m.viewportWidth < 1 || m.viewportHeight < 1 {
		return
	}
	if len(m.logs) == 0 {
		return
	}
	wrapAt := m.viewportWidth
	if wrapAt > 2 {
		wrapAt-- // leave the viewport scrollbar a column
	}
	m.resultViewport.SetContent(wrapANSI(strings.Join(m.logs, "\n"), wrapAt))
	m.resultViewport.GotoBottom()
}

func (m model) renderLogsInner() string {
	w := max(m.logsW-frameX, 1)
	st := style.ResultHeaderStyle
	if m.focused == focusLogs {
		st = style.TitleStyle
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		oneLine(st.Render("Result / Logs"), w),
		m.resultViewport.View(),
	)
}

func oneLine(s string, width int) string {
	if width < 1 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).MaxHeight(1).Render(s)
}

// wrapANSI hard-wraps to a display width while keeping CSI sequences intact.
func wrapANSI(s string, width int) string {
	if width < 1 || s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, wrapANSILine(line, width)...)
	}
	return strings.Join(out, "\n")
}

func wrapANSILine(line string, width int) []string {
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	var lines []string
	var b strings.Builder
	vis := 0
	for i := 0; i < len(line); {
		if line[i] == 0x1b {
			seq, next := consumeANSI(line, i)
			b.WriteString(seq)
			i = next
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		var rw int
		if r == utf8.RuneError && size == 1 {
			rw = 1
		} else {
			rw = lipgloss.Width(string(r))
			if rw < 1 {
				rw = 1
			}
		}
		if vis > 0 && vis+rw > width {
			lines = append(lines, b.String())
			b.Reset()
			vis = 0
		}
		b.WriteRune(r)
		vis += rw
		i += size
	}
	if b.Len() > 0 || len(lines) == 0 {
		lines = append(lines, b.String())
	}
	return lines
}

func consumeANSI(s string, i int) (string, int) {
	if i >= len(s) || s[i] != 0x1b {
		return "", i
	}
	if i+1 >= len(s) {
		return s[i:], len(s)
	}
	if s[i+1] != '[' {
		return s[i : i+2], i + 2
	}
	j := i + 2
	for j < len(s) {
		if s[j] >= 0x40 && s[j] <= 0x7e {
			return s[i : j+1], j + 1
		}
		j++
	}
	return s[i:], len(s)
}
