package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/charmbracelet/lipgloss"
	"github.com/s-588/mesh-network/internal/config"
)

func SetupSlog(cfg config.Config) error {
	var logFile *os.File
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("open home directory: %w", err)
	} else {
		var err error
		logFile, err = os.Open(path.Join(homeDir, "mesh-network"))
		if err != nil {
			slog.Error("open log file, logs will appear only in stdout", "error", err)
		}
	}

	var lvl slog.Level
	err = lvl.UnmarshalText([]byte(cfg.Level))

	stdOutHandler := PrettyHandler{
		out:  os.Stdout,
		opts: slog.HandlerOptions{Level: lvl},
	}
	var h slog.Handler
	if logFile != nil {
		h = slog.NewMultiHandler(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			Level: lvl,
		}), stdOutHandler)
	} else {
		h = stdOutHandler
	}
	slog.SetDefault(slog.New(h))
	return nil
}

type PrettyHandler struct {
	opts slog.HandlerOptions
	out  io.Writer
}

func (h PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	var levelStyle lipgloss.Style

	switch r.Level {
	case slog.LevelDebug:
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D7D7D"))
	case slog.LevelInfo:
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5BBA7D")).Bold(true)
	case slog.LevelWarn:
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4A261")).Bold(true)
	case slog.LevelError:
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E76F51")).Bold(true)
	}

	// Format: TIME [LEVEL] MSG (KEY=VAL)
	timeStr := r.Time.Format("15:04:05")
	fmt.Fprintf(h.out, "%s %s %s",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(timeStr),
		levelStyle.Render("["+level+"]"),
		lipgloss.NewStyle().Bold(true).Render(r.Message),
	)

	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.out, " %s=%s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#A3BFFA")).Render(a.Key),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Render(fmt.Sprintf("%v", a.Value.Any())),
		)
		return true
	})

	fmt.Fprintln(h.out)
	return nil
}

func (h PrettyHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.opts.Level.Level()
}
func (h PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h PrettyHandler) WithGroup(name string) slog.Handler       { return h }
