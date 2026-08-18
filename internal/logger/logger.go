// package logger contains logic for setting up log/slog logger and
// configuring output log file.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/s-588/mesh-network/internal/config"
)

func openLogFile(filename string) *os.File {
	var logFile *os.File
	if filename != "" {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			slog.Warn("file passed as log file can't be openned. Default log file will be used", "error", err, "filename", filename)
		} else {
			logFile = f
		}
	} else {
		slog.Warn("log file name has not been passed in config. Default log file will be used")
		filename = time.Now().Format(strings.ReplaceAll(time.DateTime, " ", "_")) + ".log"

		tempDir := os.TempDir()
		if tempDir == "" {
			slog.Warn("temp dir can't be found, home directory will be used instead")
			homeDir, err := os.UserHomeDir()
			if err != nil {
				slog.Warn("home dir can't be found, log file will be created here")
				f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					slog.Error("log file cannot be created", "error", err)
					return nil
				}
				logFile = f
			} else {
				f, err := os.OpenFile(path.Join(homeDir, filename), os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					slog.Error("log file cannot be created", "error", err)
					return nil
				}
				logFile = f
			}
		} else {
			f, err := os.OpenFile(path.Join(tempDir, filename), os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				slog.Error("log file cannot be created", "error", err)
				return nil
			}
			logFile = f
		}
	}

	return logFile
}

func SetupSlog(cfg config.Config, tuiHandler slog.Handler) error {
	logFile := openLogFile(cfg.LogFile)

	var lvl slog.Level
	err := lvl.UnmarshalText([]byte(cfg.Level))
	if err != nil {
		slog.Warn("log level cannot be parsed. Default INFO level will be used", "error", err)
		lvl = slog.LevelInfo
	}

	var h slog.Handler
	h = PrettyHandler{
		out:  os.Stdout,
		opts: slog.HandlerOptions{Level: lvl},
	}
	if tuiHandler != nil {
		h = tuiHandler
	}

	if logFile != nil {
		h = slog.NewMultiHandler(slog.NewTextHandler(logFile, &slog.HandlerOptions{
			Level: lvl,
		}), h)
	}
	if cfg.IsDaemon {
		h = slog.NewMultiHandler(slog.NewTextHandler(logFile, &slog.HandlerOptions{
			Level: lvl,
		}), slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: lvl,
		}), h)
	}
	slog.SetDefault(slog.New(h))
	return nil
}

// PrettyHandler struct used for displaying logs in a BubbleTea TUI in a nice way.
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
