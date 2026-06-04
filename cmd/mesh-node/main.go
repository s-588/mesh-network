package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/s-588/mesh-network/cmd/cli"
	"github.com/s-588/mesh-network/cmd/tui"
	"github.com/s-588/mesh-network/internal/config"
	"github.com/s-588/mesh-network/internal/logger"
	"github.com/s-588/mesh-network/internal/socket"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		panic(err)
	}

	cliArgs := flag.Args()
	if len(cliArgs) > 0 {
		cli.ExecuteCLICommand(cliArgs)
		return
	}

	tuiLogChan := make(chan string, 10)
	tuiLogger := &tui.RouterLogHandler{
		Logs: tuiLogChan,
	}
	err = logger.SetupSlog(cfg, tuiLogger)
	if err != nil {
		panic(err)
	}

	slog.Info("Starting node")
	slog.Info(fmt.Sprintf("Configuration parsed: %s", cfg.String()))

	t, err := socket.NewSocket(cfg.AppConfig)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go t.Start(ctx)
	go t.ProcessMessages(ctx)
	go t.StartHelloSender(ctx)
	go t.StartNeighbourCollector(ctx)

	go cli.StartIPCServer(t)
	if cfg.IsDaemon {
		slog.Info("Node started as daemon")
		<-ctx.Done()
		return
	}

	tuiModel := tui.InitialModel(int(cfg.ID), cfg.Interfaces, tuiLogChan, t)
	p := tea.NewProgram(tuiModel)
	if _, err := p.Run(); err != nil {
		slog.Error("Fatal structural UI application component crash", "error", err)
		os.Exit(1)
	}
}
