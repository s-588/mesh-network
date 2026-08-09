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
	timeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	debugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Bold(true)

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	nodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	ifaceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117"))

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Italic(true)
)

type RouterLogHandler struct {
	Logs  chan<- string
	level slog.Level
}

func (h *RouterLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *RouterLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.Enabled(ctx, r.Level) {
		return nil
	}

	attrs := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	var formatted strings.Builder

	switch t := attrs["type"].(type) {
	case logger.LogMsgType:
		switch t {
		case logger.LogTypeDATAReceived:
			formatted.WriteString(formatDataReceived(r, attrs))

		case logger.LogTypeDATASent:
			formatted.WriteString(formatDataSent(r, attrs))

		case logger.LogTypeRREQReceived, logger.LogTypeRREQSent:
			formatted.WriteString(formatRREQ(r, attrs))

		case logger.LogTypeRREPReceived, logger.LogTypeRREPSent:
			formatted.WriteString(formatRREP(r, attrs))

		case logger.LogTypeRRERReceived:
			formatted.WriteString(formatRRER(r, attrs))
		}
	default:
		formatted.WriteString(formatDefault(r, attrs))
	}

	if formatted.Len() > 0 {
		h.Logs <- formatted.String()
	}

	return nil
}

func safeNode(v any) string {
	if v == nil {
		return "Unknown"
	}
	return fmt.Sprint(v)
}

func safeIface(v any) string {
	if v == nil {
		return "?"
	}
	return fmt.Sprint(v)
}

func renderPrefix(r slog.Record, category string) string {
	timestamp := timeStyle.Render(r.Time.Format("15:04:05"))

	return fmt.Sprintf(
		"%s  %s  %-6s ",
		timestamp,
		renderLevel(r.Level),
		category,
	)
}

func renderLevel(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return debugStyle.Render("DEBUG")

	case slog.LevelInfo:
		return infoStyle.Render("INFO ")

	case slog.LevelWarn:
		return warnStyle.Render("WARN ")

	case slog.LevelError:
		return errorStyle.Render("ERROR")

	default:
		return level.String()
	}
}

func formatDataReceived(r slog.Record, attrs map[string]any) string {
	return fmt.Sprintf(
		"%s from %s:  %s",
		renderPrefix(r, "DATA"),
		nodeStyle.Render("Node "+fmt.Sprint(attrs["from"])),
		valueStyle.Render(fmt.Sprintf("%q", attrs["payload"])),
	)
}

func formatDataSent(r slog.Record, attrs map[string]any) string {
	return fmt.Sprintf(
		"%s to %s %s sent",
		renderPrefix(r, "DATA"),
		nodeStyle.Render("Node"+fmt.Sprint(attrs["to"])),
		valueStyle.Render(fmt.Sprintf("%q", attrs["payload"])),
	)
}

func formatRREQ(r slog.Record, attrs map[string]any) string {
	return fmt.Sprintf(
		"%s %s → %s  hops=%v ttl=%v seq=%v bcast=%v %s",
		renderPrefix(r, "RREQ"),
		nodeStyle.Render("Node"+fmt.Sprint(attrs["from"])),
		nodeStyle.Render("Node"+fmt.Sprint(attrs["to"])),
		attrs["hops"],
		attrs["ttl"],
		attrs["seq"],
		attrs["bcastID"],
		ifaceStyle.Render(fmt.Sprintf("iface=%v", attrs["interface"])),
	)
}

func formatRREP(r slog.Record, attrs map[string]any) string {
	return fmt.Sprintf(
		"%s to %s recieved",
		renderPrefix(r, "RREP"),
		nodeStyle.Render("Node"+fmt.Sprint(attrs["to"])),
	)
}

func formatRRER(r slog.Record, attrs map[string]any) string {
	return fmt.Sprintf(
		"%s broken route %s ✖ %s",
		renderPrefix(r, "RERR"),
		nodeStyle.Render("Node"+fmt.Sprint(attrs["from"])),
		nodeStyle.Render("Node"+fmt.Sprint(attrs["to"])),
	)
}

func formatDefault(r slog.Record, attrs map[string]any) string {
	var b strings.Builder

	category := "SYS"

	if msgType, ok := attrs["type"]; ok {
		category = fmt.Sprint(msgType)
	}

	b.WriteString(renderPrefix(r, category))
	b.WriteString(r.Message)

	for k, v := range attrs {
		if k == "type" {
			continue
		}

		b.WriteString("\n")
		b.WriteString("                    ")

		b.WriteString(keyStyle.Render(k))
		b.WriteString("=")

		if k == "error" {
			b.WriteString(errorMsgStyle.Render(fmt.Sprint(v)))
		} else {
			b.WriteString(valueStyle.Render(fmt.Sprint(v)))
		}
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
