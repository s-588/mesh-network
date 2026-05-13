package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/s-588/mesh-network/internal/config"
)

func SetupSlog(cfg config.Config) error {
	var logFile *os.File
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("can't open home directory: %w", err)
	} else {
		var err error
		logFile, err = os.Open(path.Join(homeDir, "mesh-network"))
		if err != nil {
			slog.Error("open log file", "error", err)
		}
	}

	var lvl slog.Level
	err = lvl.UnmarshalText([]byte(cfg.Level))
	stdOutHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: cfg.LogConfig.Level == "DEBUG",
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "level" || a.Key == "time" {
				return slog.Attr{}
			}
			return a
		},
		Level: lvl,
	})
	var h slog.Handler
	if logFile != nil {
		h = slog.NewMultiHandler(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}), stdOutHandler)
	} else {
		h = stdOutHandler
	}
	slog.SetDefault(slog.New(h))
	return nil
}
