package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	urfave "github.com/urfave/cli/v3"

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

	app := &urfave.Command{
		Name:  "mesh-node",
		Usage: "AODV mesh network node",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{
				Name:    "daemon",
				Usage:   "Start without GUI as daemon",
				Value:   false,
				Sources: urfave.EnvVars("DAEMON"),
			},
			&urfave.UintFlag{
				Name:    "port",
				Usage:   "Port to listen on",
				Value:   uint(cfg.Port),
				Sources: urfave.EnvVars("PORT"),
			},
			&urfave.StringFlag{
				Name:    "interface",
				Usage:   "Comma-separated interfaces to listen on",
				Value:   strings.Join(cfg.Interfaces, ","),
				Sources: urfave.EnvVars("INTERFACE"),
			},
			&urfave.Uint64Flag{
				Name:    "id",
				Usage:   "Node ID",
				Value:   cfg.ID,
				Sources: urfave.EnvVars("ID"),
			},
			&urfave.UintFlag{
				Name:    "ttl",
				Usage:   "Time To Live for messages",
				Value:   uint(cfg.TTL),
				Sources: urfave.EnvVars("TTL"),
			},
			&urfave.UintFlag{
				Name:    "lifetime",
				Usage:   "Lifetime of messages and route table entries (seconds)",
				Value:   uint(cfg.Lifetime),
				Sources: urfave.EnvVars("LIFETIME"),
			},
			&urfave.UintFlag{
				Name:    "hello-interval",
				Usage:   "HELLO broadcast interval (seconds)",
				Value:   uint(cfg.HelloInterval),
				Sources: urfave.EnvVars("HELLO_INTERVAL"),
			},
			&urfave.StringFlag{
				Name:    "log-file",
				Usage:   "Log filename or full path",
				Value:   cfg.LogFile,
				Sources: urfave.EnvVars("LOG_FILE"),
			},
			&urfave.StringFlag{
				Name:    "log-level",
				Usage:   "Log level (DEBUG, INFO, WARN, ERROR)",
				Value:   cfg.Level,
				Sources: urfave.EnvVars("LOG_LEVEL"),
			},
		},
		// override environment variables with flags
		Action: func(ctx context.Context, c *urfave.Command) error {
			cfg.IsDaemon = c.Bool("daemon")
			cfg.Port = uint16(c.Uint("port"))
			if ifaces := c.String("interface"); ifaces != "" {
				cfg.Interfaces = strings.Split(ifaces, ",")
			}
			cfg.ID = c.Uint64("id")
			cfg.TTL = uint8(c.Uint("ttl"))
			cfg.Lifetime = uint32(c.Uint("lifetime"))
			cfg.HelloInterval = int(c.Uint("hello-interval"))
			if lf := c.String("log-file"); lf != "" {
				cfg.LogFile = lf
			}
			if ll := c.String("log-level"); ll != "" {
				cfg.Level = strings.ToUpper(ll)
			}

			return runNode(cfg)
		},
		Commands: []*urfave.Command{
			{
				Name:  "send",
				Usage: "send a message to a node",
				Commands: []*urfave.Command{
					{
						Name:  "rreq",
						Usage: "send a Route REQuest for trying to find route to node",
						Arguments: []urfave.Argument{
							&urfave.Int64Arg{
								Name:      "target",
								UsageText: "ID of a node you trying to find",
							},
						},
						Action: func(ctx context.Context, c *urfave.Command) error {
							result, err := cli.SendRREQ(c.Int64Arg("target"))
							if err != nil {
								return err
							}
							fmt.Fprintf(os.Stdout, result)
							return nil
						},
					},
					{
						Name:  "msg",
						Usage: "send a text message to node",
						Arguments: []urfave.Argument{
							&urfave.Int64Arg{
								Name:      "target",
								UsageText: "ID of a destination node",
							},
							&urfave.StringArg{
								Name:      "msg",
								UsageText: "message that you want to send",
							},
						},
						Action: func(ctx context.Context, c *urfave.Command) error {
							result, err := cli.SendMsg(c.Int64Arg("target"), c.StringArg("msg"))
							if err != nil {
								return err
							}
							fmt.Fprintf(os.Stdout, result)
							return nil
						},
					},
				},
			},
			{
				Name:  "show",
				Usage: "show node information",
				Commands: []*urfave.Command{
					{
						Name:    "messages",
						Aliases: []string{"m", "msgs"},
						Action: func(ctx context.Context, c *urfave.Command) error {
							result, err := cli.GetMsgs()
							if err != nil {
								return err
							}
							fmt.Fprint(os.Stdout, result)
							return nil
						},
					},
					{
						Name:    "neighbours",
						Aliases: []string{"n"},
						Action: func(ctx context.Context, c *urfave.Command) error {
							result, err := cli.GetNeighbours()
							if err != nil {
								return err
							}
							fmt.Fprint(os.Stdout, result)
							return nil
						},
					},
					{
						Name:    "routes",
						Aliases: []string{"r"},
						Action: func(ctx context.Context, c *urfave.Command) error {
							result, err := cli.GetRoutes()
							if err != nil {
								return err
							}
							fmt.Fprint(os.Stdout, result)
							return nil
						},
					},
				},
			},
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err)
	}
}

func runNode(cfg config.Config) error {
	tuiLogChan := make(chan string, 10)
	tuiLogger := &tui.RouterLogHandler{
		Logs: tuiLogChan,
	}
	err := logger.SetupSlog(cfg, tuiLogger)
	if err != nil {
		return err
	}

	slog.Info("Starting node")
	slog.Info(fmt.Sprintf("Configuration parsed: %s", cfg.String()))

	t, err := socket.NewSocket(cfg.AppConfig)
	if err != nil {
		return err
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
	}

	tuiModel := tui.InitialModel(int(cfg.ID), cfg.Interfaces, tuiLogChan, t)
	p := tea.NewProgram(tuiModel)
	if _, err := p.Run(); err != nil {
		slog.Error("Fatal TUI component crash", "error", err)
		os.Exit(1)
	}
	return nil
}
