package main

import (
	"fmt"
	"log/slog"
	"os"
	"path"
)

func setupSlog(cfg Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("can't open home directory: %w", err)
	}

	logFile, err := os.Open(path.Join(homeDir, "mesh-network"))
	if err != nil {
		return fmt.Errorf("can't open log file: %w", err)
	}
	var lvl slog.Level
	err = lvl.UnmarshalText([]byte(cfg.Level))

	h := slog.NewMultiHandler(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}), slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: cfg.LogConfig.Level == "DEBUG",
		Level:     lvl,
	}))
	slog.SetDefault(slog.New(h))
	return nil
}
