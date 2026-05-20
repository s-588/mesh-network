package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/pkg/logger"
)

var (
	nodeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	ifaceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	msgStyle   = lipgloss.NewStyle().Italic(true)
	levelStyle = map[slog.Level]lipgloss.Style{
		slog.LevelDebug: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		slog.LevelInfo:  lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		slog.LevelWarn:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		slog.LevelError: lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
	}
)

type RouterLogHandler struct {
	Logs chan<- string
}

func (h *RouterLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *RouterLogHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	var formatted strings.Builder

	// === Special formatted logs ===
	switch t := attrs["type"].(type) {
	case logger.LogMsgType:
		switch t {
		case logger.LogTypeDATAReceived:
			fmt.Println(attrs["type"])
			formatted.WriteString(formatDataReceived(attrs))

		case logger.LogTypeDATASent:
			fmt.Println(attrs["type"])
			formatted.WriteString(formatDataSent(attrs))

		case logger.LogTypeRREQReceived, logger.LogTypeRREQSent:
			fmt.Println(attrs["type"])
			formatted.WriteString(formatRREQ(attrs))

		case logger.LogTypeRREPReceived, logger.LogTypeRREPSent:
			fmt.Println(attrs["type"])
			formatted.WriteString(formatRREP(attrs))

		case logger.LogTypeRRERReceived:
			fmt.Println(attrs["type"])
			formatted.WriteString(formatRRER(attrs))
		}
	default:
		formatted.WriteString(formatDefault(r, attrs))
	}

	if formatted.Len() > 0 {
		h.Logs <- formatted.String()
	}

	return nil
}

// ─────────────────────────────────────────────────────────────
// Helper formatters
// ─────────────────────────────────────────────────────────────

func formatDataReceived(attrs map[string]any) string {
	return fmt.Sprintf("%s : %s %s %s ... %s",
		nodeStyle.Render(fmt.Sprint(attrs["to"])),
		attrs["payload"],
		ifaceStyle.Render("<-"+fmt.Sprint(attrs["interface"])),
		"<- Node"+fmt.Sprint(attrs["from"]),
		nodeStyle.Render(fmt.Sprint(attrs["to"])),
	)
}

func formatDataSent(attrs map[string]any) string {
	return fmt.Sprintf("-> %s : %s %s -> Node%s ... %s",
		nodeStyle.Render(fmt.Sprint(attrs["to"])),
		attrs["payload"],
		ifaceStyle.Render("-> "+fmt.Sprint(attrs["interface"])),
		attrs["next_hop"],
		nodeStyle.Render(fmt.Sprint(attrs["to"])),
	)
}

func formatRREQ(attrs map[string]any) string {
	return fmt.Sprintf("RREQ : %s %s %s",
		attrs["id"],
		"->",
		ifaceStyle.Render(fmt.Sprint(attrs["interfaces"])),
	)
}

func formatRREP(attrs map[string]any) string {
	to := attrs["to"]
	from := attrs["from"]
	iface := getAttr(attrs, "interface", "")

	return fmt.Sprintf("RREP -> %s : %s <- Node%s ... %s",
		nodeStyle.Render(fmt.Sprint(to)),
		ifaceStyle.Render(fmt.Sprintf("<- %v", iface)),
		fmt.Sprint(from),
		nodeStyle.Render(fmt.Sprint(to)),
	)
}

func formatRRER(attrs map[string]any) string {
	return fmt.Sprintf("RERR <- Node%s : broken path -> %s",
		attrs["from"],
		nodeStyle.Render(fmt.Sprint(attrs["to"])),
	)
}

func formatDefault(r slog.Record, attrs map[string]any) string {
	var b strings.Builder

	// Level
	if style, ok := levelStyle[r.Level]; ok {
		b.WriteString(style.Render(r.Level.String()))
	} else {
		b.WriteString(r.Level.String())
	}
	b.WriteString(" ")

	// Message
	b.WriteString(r.Message)

	// All attributes (except "type" which we already tried to match)
	for k, v := range attrs {
		if k == "type" {
			continue
		}
		b.WriteString(fmt.Sprintf(" %s=%v", k, v))
	}

	return b.String()
}

// Safe attribute getter
func getAttr(attrs map[string]any, key string, defaultVal any) any {
	if v, ok := attrs[key]; ok && v != nil {
		return v
	}
	return defaultVal
}

// ─────────────────────────────────────────────────────────────
// slog.Handler boilerplate
// ─────────────────────────────────────────────────────────────
func (h *RouterLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *RouterLogHandler) WithGroup(name string) slog.Handler       { return h }
