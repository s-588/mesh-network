// mesh-node is an app that immulate mesh network on a virtual level
// using the AODV protocol mechanisms.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
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

	app := setupCLI(cfg)
	if err := app.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "can't run app: %s\n", err)
	}
}

//nolint:funlen
func setupCLI(cfg config.Config) *urfave.Command {
	return &urfave.Command{
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

			port := c.Uint("port")
			if port < 1 || port > math.MaxUint16 {
				return fmt.Errorf("invalid port number: %d", port)
			}
			cfg.Port = uint16(port)

			if ifaces := c.String("interface"); ifaces != "" {
				cfg.Interfaces = strings.Split(ifaces, ",")
			}
			cfg.ID = c.Uint64("id")

			ttl := c.Uint("ttl")
			if ttl < 1 || ttl > math.MaxUint8 {
				return fmt.Errorf("invalid TTL value: %d", ttl)
			}
			cfg.TTL = uint8(ttl)

			lifetime := c.Uint("lifetime")
			if lifetime < 1 || lifetime > math.MaxUint32 {
				return fmt.Errorf("invalid lifetime value: %d", lifetime)
			}
			cfg.Lifetime = uint32(lifetime)

			cfg.HelloInterval = int(c.Uint("hello-interval"))
			if lf := c.String("log-file"); lf != "" {
				cfg.LogFile = lf
			}
			if ll := c.String("log-level"); ll != "" {
				cfg.Level = strings.ToUpper(ll)
			}

			return runNode(ctx, cfg)
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
							result, err := cli.SendRREQ(ctx, c.Int64Arg("target"))
							if err != nil {
								return err
							}
							_, _ = fmt.Fprint(os.Stdout, result)
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
							result, err := cli.SendMsg(ctx, c.Int64Arg("target"), c.StringArg("msg"))
							if err != nil {
								return err
							}
							_, _ = fmt.Fprint(os.Stdout, result)
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
						Action: func(ctx context.Context, _ *urfave.Command) error {
							result, err := cli.GetMsgs(ctx)
							if err != nil {
								return err
							}
							_, _ = fmt.Fprint(os.Stdout, result)
							return nil
						},
					},
					{
						Name:    "neighbours",
						Aliases: []string{"n"},
						Action: func(ctx context.Context, _ *urfave.Command) error {
							result, err := cli.GetNeighbours(ctx)
							if err != nil {
								return err
							}
							_, _ = fmt.Fprint(os.Stdout, result)
							return nil
						},
					},
					{
						Name:    "routes",
						Aliases: []string{"r"},
						Action: func(ctx context.Context, _ *urfave.Command) error {
							result, err := cli.GetRoutes(ctx)
							if err != nil {
								return err
							}
							_, _ = fmt.Fprint(os.Stdout, result)
							return nil
						},
					},
				},
			},
		},
	}
}

func runNode(ctx context.Context, cfg config.Config) error {
	tuiLogChan := make(chan string, 10)
	tuiLogger := &tui.RouterLogHandler{
		Logs: tuiLogChan,
	}
	err := logger.SetupSlog(cfg, tuiLogger)
	if err != nil {
		return err
	}

	slog.Info("Starting node")
	slog.Info("Configuration parsed", "config", cfg.String())

	t, err := socket.NewSocket(ctx, cfg.AppConfig)
	if err != nil {
		return err
	}
	go t.Start(ctx)
	go t.ProcessMessages(ctx)
	go t.StartHelloSender(ctx)
	go t.StartNeighbourCollector(ctx)

	go cli.StartIPCServer(t)
	if cfg.IsDaemon {
		slog.Info("Node started as daemon")
		<-ctx.Done()
	}

	tuiModel := tui.InitialModel(cfg.ID, cfg.Interfaces, tuiLogChan, t)
	p := tea.NewProgram(tuiModel)
	if _, err := p.Run(); err != nil {
		slog.Error("Fatal TUI component crash", "error", err)
	}
	return nil
}
