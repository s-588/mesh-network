// Package cli pretty-prints mesh-node IPC data for the terminal and hosts
// the local IPC HTTP server used by the CLI and the TUI.
package cli

import (
	"context"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/cmd/style"
	"github.com/s-588/mesh-network/internal/ipc"
)

// Column widths fit a full uint64 node ID (20 digits), an IPv4 host:port
// ("255.255.255.255:65535"), and a "2006-01-02 15:04:05" timestamp.
const (
	colNodeID = 20
	colAddr   = 22
	colSeen   = 20
	colIface  = 12
	colHops   = 6
	colSeq    = 10
)

// GetMsgs receives messages from the node's IPC server.
func GetMsgs(ctx context.Context) (string, error) {
	msgs, err := ipc.GetMessages(ctx)
	if err != nil {
		return "", err
	}
	if len(msgs) == 0 {
		return "No messages yet.\n", nil
	}
	text := strings.Join(msgs, "\n")
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text, nil
}

// GetNeighbours receives neighbours data from the node's IPC server.
func GetNeighbours(ctx context.Context) (string, error) {
	neighbours, err := ipc.GetNeighbours(ctx)
	if err != nil {
		return "", err
	}

	s := strings.Builder{}
	s.WriteString(style.TitleStyle.Render("\n Neighbours table "))
	s.WriteString("\n")
	if len(neighbours) == 0 {
		s.WriteString("  No neighbours yet.\n")
		return s.String(), nil
	}

	c1 := lipgloss.NewStyle().Width(colNodeID).Render
	c2 := lipgloss.NewStyle().Width(colAddr).Render
	c3 := lipgloss.NewStyle().Width(colSeen).Render
	c4 := lipgloss.NewStyle().Width(colIface).Render

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		c1(style.ResultHeaderStyle.Render("ID")),
		c2(style.ResultHeaderStyle.Render("Address")),
		c3(style.ResultHeaderStyle.Render("Last Seen")),
		c4(style.ResultHeaderStyle.Render("Iface")),
	)
	s.WriteString(header)
	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Foreground(style.Muted).Render(strings.Repeat("─", colNodeID+colAddr+colSeen+colIface)))
	s.WriteString("\n")

	for _, n := range neighbours {
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			c1(lipgloss.NewStyle().Foreground(style.Accent).Render(fmt.Sprint(n.ID))),
			c2(lipgloss.NewStyle().Foreground(style.TextColor).Render(n.Addr)),
			c3(lipgloss.NewStyle().Foreground(style.Warn).Render(n.LastSeen)),
			c4(lipgloss.NewStyle().Foreground(style.Accent2).Render(n.Iface)),
		)
		s.WriteString(row)
		s.WriteString("\n")
	}
	return s.String(), nil
}

// GetRoutes receives routes data from the node's IPC server.
func GetRoutes(ctx context.Context) (string, error) {
	routes, err := ipc.GetRoutes(ctx)
	if err != nil {
		return "", err
	}

	s := strings.Builder{}
	s.WriteString(style.TitleStyle.Render("\n Routes table "))
	s.WriteString("\n")
	if len(routes) == 0 {
		s.WriteString("  No routes yet.\n")
		return s.String(), nil
	}

	c1 := lipgloss.NewStyle().Width(colNodeID).Render
	c2 := lipgloss.NewStyle().Width(colAddr).Render
	c3 := lipgloss.NewStyle().Width(colHops).Render
	c4 := lipgloss.NewStyle().Width(colSeq).Render
	c5 := lipgloss.NewStyle().Width(colIface).Render

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		c1(style.ResultHeaderStyle.Render("DstID")),
		c2(style.ResultHeaderStyle.Render("NextHop")),
		c3(style.ResultHeaderStyle.Render("Hops")),
		c4(style.ResultHeaderStyle.Render("Seq")),
		c5(style.ResultHeaderStyle.Render("Iface")),
	)
	s.WriteString(header)
	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Foreground(style.Muted).Render(strings.Repeat("─", colNodeID+colAddr+colHops+colSeq+colIface)))
	s.WriteString("\n")

	for _, r := range routes {
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			c1(lipgloss.NewStyle().Foreground(style.Accent).Render(fmt.Sprint(r.DstID))),
			c2(lipgloss.NewStyle().Foreground(style.TextColor).Render(r.NextHop)),
			c3(lipgloss.NewStyle().Foreground(style.TextColor).Render(fmt.Sprint(r.Hops))),
			c4(lipgloss.NewStyle().Foreground(style.TextColor).Render(fmt.Sprint(r.Seq))),
			c5(lipgloss.NewStyle().Foreground(style.Accent2).Render(r.Iface)),
		)
		s.WriteString(row)
		s.WriteString("\n")
	}
	return s.String(), nil
}

// GetConfig fetches the running node's configuration from the IPC server.
func GetConfig(ctx context.Context) (string, error) {
	cfg, err := ipc.GetConfig(ctx)
	if err != nil {
		return "", err
	}

	logFile := cfg.LogFile
	if strings.TrimSpace(logFile) == "" {
		logFile = "(stderr)"
	}
	ifaces := strings.Join(cfg.Interfaces, ", ")
	if ifaces == "" {
		ifaces = "(none)"
	}

	s := strings.Builder{}
	s.WriteString(style.TitleStyle.Render("\n Node configuration "))
	s.WriteString("\n")
	fmt.Fprintf(&s, "  ID:             %d\n", cfg.ID)
	fmt.Fprintf(&s, "  Port:           %d\n", cfg.Port)
	fmt.Fprintf(&s, "  Interfaces:     %s\n", ifaces)
	fmt.Fprintf(&s, "  TTL:            %d\n", cfg.TTL)
	fmt.Fprintf(&s, "  Lifetime:       %d s\n", cfg.Lifetime)
	fmt.Fprintf(&s, "  Hello interval: %d s\n", cfg.HelloInterval)
	fmt.Fprintf(&s, "  Log level:      %s\n", cfg.LogLevel)
	fmt.Fprintf(&s, "  Log file:       %s\n", logFile)
	fmt.Fprintf(&s, "  Daemon:         %t\n", cfg.IsDaemon)
	return s.String(), nil
}
