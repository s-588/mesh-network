// mesh-node is an app that emulates a mesh network on a virtual level
// using the AODV protocol mechanisms.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	urfave "github.com/urfave/cli/v3"

	"github.com/s-588/mesh-network/cmd/cli"
	"github.com/s-588/mesh-network/cmd/tui"
	"github.com/s-588/mesh-network/internal/config"
	"github.com/s-588/mesh-network/internal/ipc"
	"github.com/s-588/mesh-network/internal/logger"
	"github.com/s-588/mesh-network/internal/socket"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "invalid configuration: %s\n", err)
		os.Exit(1)
	}

	app := setupCLI(cfg)
	if err := app.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", friendlyCLIError(err))
		os.Exit(1)
	}
}

//nolint:funlen,gocyclo
func setupCLI(cfg config.Config) *urfave.Command {
	return &urfave.Command{
		Name:  "mesh-node",
		Usage: "AODV mesh network node — run a node, send messages, inspect routes & neighbours",
		Description: `mesh-node implements a virtual AODV mesh network node.

When started without subcommands it launches the interactive TUI.
If no background server is running it automatically starts one
(mesh-node --daemon) in a separate process and connects the TUI to it
via the local IPC port.

Use the "send" and "show" subcommands to interact with an already-running
node from the shell. Stop that node with "mesh-node kill".`,
		Flags: []urfave.Flag{
			&urfave.BoolFlag{
				Name:    "daemon",
				Aliases: []string{"server", "d"},
				Usage:   "Start as background server (no TUI). Required for IPC clients.",
				Value:   false,
				Sources: urfave.EnvVars("DAEMON"),
			},
			&urfave.UintFlag{
				Name:    "port",
				Usage:   "UDP/TCP port the node listens on for mesh traffic",
				Value:   uint(cfg.Port),
				Sources: urfave.EnvVars("PORT"),
			},
			&urfave.StringFlag{
				Name:    "interface",
				Aliases: []string{"i"},
				Usage:   "Comma-separated network interfaces to bind (e.g. eth0,wlan0)",
				Value:   strings.Join(cfg.Interfaces, ","),
				Sources: urfave.EnvVars("INTERFACE"),
			},
			&urfave.Uint64Flag{
				Name:    "id",
				Usage:   "Unique node ID (must be set via flag or ID env var)",
				Value:   cfg.ID,
				Sources: urfave.EnvVars("ID"),
			},
			&urfave.UintFlag{
				Name:    "ttl",
				Usage:   "Default Time-To-Live for outgoing messages",
				Value:   uint(cfg.TTL),
				Sources: urfave.EnvVars("TTL"),
			},
			&urfave.UintFlag{
				Name:    "lifetime",
				Usage:   "Lifetime (seconds) of routes and cached messages",
				Value:   uint(cfg.Lifetime),
				Sources: urfave.EnvVars("LIFETIME"),
			},
			&urfave.UintFlag{
				Name:    "hello-interval",
				Usage:   "Interval (seconds) between HELLO broadcasts (1-86400)",
				Value:   uint(cfg.HelloInterval),
				Sources: urfave.EnvVars("HELLO_INTERVAL"),
			},
			&urfave.StringFlag{
				Name:    "log-file",
				Usage:   "Log file path (empty = stderr only)",
				Value:   cfg.LogFile,
				Sources: urfave.EnvVars("LOG_FILE"),
			},
			&urfave.StringFlag{
				Name:    "log-level",
				Usage:   "Log level: DEBUG, INFO, WARN, ERROR",
				Value:   cfg.Level,
				Sources: urfave.EnvVars("LOG_LEVEL"),
			},
		},
		// Default action: start TUI (and auto-start daemon if needed)
		Action: func(ctx context.Context, c *urfave.Command) error {
			cfg.IsDaemon = c.Bool("daemon")

			port := c.Uint("port")
			if port < 1 || port > math.MaxUint16 {
				return fmt.Errorf("invalid port number: %d", port)
			}
			cfg.Port = uint16(port)

			if ifaces := c.String("interface"); ifaces != "" {
				cfg.Interfaces = splitInterfaces(ifaces)
				if len(cfg.Interfaces) == 0 {
					return fmt.Errorf("no network interfaces given")
				}
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

			hello := c.Uint("hello-interval")
			if hello < 1 || hello > 24*60*60 {
				return fmt.Errorf("invalid hello interval: %d (want 1-86400 seconds)", hello)
			}
			cfg.HelloInterval = int(hello)
			if lf := c.String("log-file"); lf != "" {
				cfg.LogFile = lf
			}
			if ll := c.String("log-level"); ll != "" {
				cfg.Level = strings.ToUpper(ll)
			}

			if cfg.IsDaemon {
				return runDaemon(ctx, cfg)
			}

			// Default: ensure a background server is running, then launch TUI client
			return runTUI(ctx, cfg, c)
		},
		Commands: []*urfave.Command{
			{
				Name:  "send",
				Usage: "Send data or route requests to a node via the running background server",
				Commands: []*urfave.Command{
					{
						Name:  "rreq",
						Usage: "Send a Route REQuest (RREQ) to discover a path to the target node",
						Arguments: []urfave.Argument{
							&urfave.Uint64Args{
								Name:      "target",
								Min:       1,
								Max:       1,
								UsageText: "ID of the destination node",
							},
						},
						Action: func(ctx context.Context, c *urfave.Command) error {
							if c.NArg() > 0 {
								return fmt.Errorf("too many arguments; usage: mesh-node send rreq <target-id>")
							}
							targets := c.Uint64Args("target")
							if len(targets) == 0 {
								return fmt.Errorf("destination ID is required\nusage: mesh-node send rreq <target-id>")
							}
							result, err := ipc.SendRREQ(ctx, targets[0])
							if err != nil {
								return err
							}
							_, _ = fmt.Fprint(os.Stdout, result)
							return nil
						},
					},
					{
						Name:  "msg",
						Usage: "Send a text message to a node",
						Arguments: []urfave.Argument{
							&urfave.Uint64Args{
								Name:      "target",
								Min:       1,
								Max:       1,
								UsageText: "ID of the destination node",
							},
							&urfave.StringArgs{
								Name:      "msg",
								Min:       1,
								Max:       1,
								UsageText: "message text; quote it if it contains spaces",
							},
						},
						Action: func(ctx context.Context, c *urfave.Command) error {
							if c.NArg() > 0 {
								return fmt.Errorf("too many arguments; quote the message if it contains spaces\nusage: mesh-node send msg <target-id> \"message\"")
							}
							targets := c.Uint64Args("target")
							msgs := c.StringArgs("msg")
							if len(targets) == 0 || len(msgs) == 0 {
								return fmt.Errorf("destination ID and message are required\nusage: mesh-node send msg <target-id> \"message\"")
							}
							result, err := ipc.SendMsg(ctx, targets[0], msgs[0])
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
				Usage: "Display information from the running background server",
				Commands: []*urfave.Command{
					{
						Name:    "messages",
						Aliases: []string{"m", "msgs"},
						Usage:   "Show received messages",
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
						Aliases: []string{"n", "neigh"},
						Usage:   "Show the neighbours table",
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
						Usage:   "Show the routing table",
						Action: func(ctx context.Context, _ *urfave.Command) error {
							result, err := cli.GetRoutes(ctx)
							if err != nil {
								return err
							}
							_, _ = fmt.Fprint(os.Stdout, result)
							return nil
						},
					},
					{
						Name:    "config",
						Aliases: []string{"c", "cfg"},
						Usage:   "Show the running node's configuration",
						Action: func(ctx context.Context, _ *urfave.Command) error {
							result, err := cli.GetConfig(ctx)
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
				Name:    "kill",
				Aliases: []string{"stop"},
				Usage:   "Stop the background mesh node",
				Action: func(ctx context.Context, _ *urfave.Command) error {
					if ipc.IsNodeRunning(ctx) {
						result, err := ipc.KillNode(ctx)
						if err != nil {
							return err
						}
						_, _ = fmt.Fprint(os.Stdout, result)
						return nil
					}
					return stopByPID()
				},
			},
		},
	}
}

// runDaemon starts the node as a background process (no TUI).
func runDaemon(ctx context.Context, cfg config.Config) error {
	// Generate snowflake ID only when actually starting a node.
	if err := cfg.EnsureID(); err != nil {
		return err
	}

	tuiLogChan := make(chan string, 10) // drained; the TUI client has its own channel
	go func() {
		for range tuiLogChan {
		}
	}()
	tuiLogger := &tui.RouterLogHandler{Logs: tuiLogChan}
	if err := logger.SetupSlog(cfg, tuiLogger); err != nil {
		return err
	}

	slog.Info("Starting node in daemon mode")

	if ipc.IsNodeRunning(ctx) {
		return fmt.Errorf("a mesh node is already running; stop it with: mesh-node kill")
	}
	if err := writePIDFile(); err != nil {
		return err
	}
	defer removePIDFile()

	t, err := socket.NewSocket(ctx, cfg.AppConfig)
	if err != nil {
		return err
	}
	go t.Start(ctx)
	go t.ProcessMessages(ctx)
	go t.StartHelloSender(ctx)
	go t.StartNeighbourCollector(ctx)

	slog.Info("Node started as daemon", "id", cfg.ID, "ipc", ipc.IPCPort)
	slog.Info("Stop this node with: mesh-node kill")
	return cli.StartIPCServer(ctx, t, cfg)
}

// runTUI ensures a background server is available and launches the TUI client.
func runTUI(ctx context.Context, cfg config.Config, c *urfave.Command) error {
	startedHere := false
	if !ipc.IsNodeRunning(ctx) {
		slog.Info("No background server detected — starting one…")
		output, err := startBackgroundDaemon(c)
		if err != nil {
			msg := fmt.Sprintf("failed to start background server: %v", err)
			if output != "" {
				msg += "\n--- daemon output ---\n" + output
			}
			return fmt.Errorf("%s\n\nHint: start the server manually:\n  mesh-node --daemon", msg)
		}
		// Give the daemon time to initialise and bind the IPC port.
		deadline := time.Now().Add(8 * time.Second)
		ready := false
		for time.Now().Before(deadline) {
			if ipc.IsNodeRunning(ctx) {
				slog.Info("Background server is ready")
				ready = true
				break
			}
			time.Sleep(150 * time.Millisecond)
		}
		if !ready {
			msg := "background server did not become ready in time"
			if output != "" {
				msg += "\n--- daemon output ---\n" + output
			}
			//nolint:staticcheck // returned error will be displaed to the user so formating is ok.
			return fmt.Errorf("%s\n\nHint: start the server manually in another terminal:\n  mesh-node --daemon\nand then re-run the TUI.", msg)
		}
		startedHere = true
	} else {
		slog.Info("Connecting to existing background server")
	}

	err := runTUIClient(ctx, cfg)
	if startedHere && ipc.IsNodeRunning(context.Background()) {
		_, _ = fmt.Fprintf(os.Stderr, "Background node is still running. Stop it with: mesh-node kill\n")
	}
	return err
}

// cliFlagPresent reports whether name appears as a real command-line flag in
// os.Args. Environment-sourced values are ignored so Docker Compose env vars
// are not re-passed as CLI flags (the child inherits the parent environment).
func cliFlagPresent(name string) bool {
	hyphen := "--" + name
	short := "-" + name
	for _, a := range os.Args[1:] {
		if a == hyphen || a == short ||
			strings.HasPrefix(a, hyphen+"=") ||
			strings.HasPrefix(a, short+"=") {
			return true
		}
	}
	return false
}

// startBackgroundDaemon launches the same binary with --daemon.
// Only flags present on the real command line are forwarded. Values that came
// solely from environment variables are NOT re-passed — the child inherits the
// parent env and applies its own defaults (including snowflake ID).
func startBackgroundDaemon(c *urfave.Command) (stderrOut string, err error) {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	// #nosec G703 // os.Executable()/os.Args[0] finds the same trusted binary.
	if _, statErr := os.Stat(exe); statErr != nil {
		exe = os.Args[0]
	}

	args := parseArgs(c)

	// #nosec G204,G702 same trusted local binary, fixed allowlist of flags.
	cmd := exec.Command(exe, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}

	if err := cmd.Start(); err != nil {
		return daemonOutput(&stdoutBuf, &stderrBuf), fmt.Errorf("exec %s %v: %w", exe, args, err)
	}

	go func() { _ = cmd.Wait() }()

	time.Sleep(300 * time.Millisecond)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return daemonOutput(&stdoutBuf, &stderrBuf), fmt.Errorf("background server exited immediately")
	}

	slog.Info("started background server", "pid", cmd.Process.Pid, "args", args)
	return daemonOutput(&stdoutBuf, &stderrBuf), nil
}

func parseArgs(c *urfave.Command) []string {
	args := []string{"--daemon"}

	if cliFlagPresent("port") {
		args = append(args, "--port", fmt.Sprintf("%d", c.Uint("port")))
	}
	if cliFlagPresent("interface") || cliFlagPresent("i") {
		args = append(args, "--interface", c.String("interface"))
	}
	if cliFlagPresent("id") {
		args = append(args, "--id", fmt.Sprintf("%d", c.Uint64("id")))
	}
	if cliFlagPresent("ttl") {
		args = append(args, "--ttl", fmt.Sprintf("%d", c.Uint("ttl")))
	}
	if cliFlagPresent("lifetime") {
		args = append(args, "--lifetime", fmt.Sprintf("%d", c.Uint("lifetime")))
	}
	if cliFlagPresent("hello-interval") {
		args = append(args, "--hello-interval", fmt.Sprintf("%d", c.Uint("hello-interval")))
	}
	if cliFlagPresent("log-file") {
		args = append(args, "--log-file", c.String("log-file"))
	}
	if cliFlagPresent("log-level") {
		args = append(args, "--log-level", c.String("log-level"))
	}
	return args
}

// runTUIClient starts the interactive TUI that talks to the IPC server.
func runTUIClient(ctx context.Context, cfg config.Config) error {
	// Minimal logger for the client process itself
	tuiLogChan := make(chan string, 10)
	tuiLogger := &tui.RouterLogHandler{Logs: tuiLogChan}
	if err := logger.SetupSlog(cfg, tuiLogger); err != nil {
		return err
	}

	// Prefer the *server's* identity over the client process config.
	nodeID := cfg.ID
	ifaces := cfg.Interfaces
	if serverCfg, err := ipc.GetConfig(ctx); err == nil {
		nodeID = serverCfg.ID
		if len(serverCfg.Interfaces) > 0 {
			ifaces = serverCfg.Interfaces
		}
	}

	slog.Info("Starting TUI client", "nodeID", nodeID)

	// Pass nil socket — the model will use the CLI HTTP client instead.
	tuiModel := tui.InitialModel(nodeID, ifaces, tuiLogChan, nil)
	p := tea.NewProgram(tuiModel)
	if _, err := p.Run(); err != nil {
		slog.Error("TUI crashed", "error", err)
		return err
	}
	return nil
}

func friendlyCLIError(err error) string {
	msg := err.Error()
	const marker = "sufficient count of arg "
	if i := strings.Index(msg, marker); i >= 0 {
		rest := msg[i+len(marker):]
		name := rest
		if sp := strings.IndexByte(rest, ' '); sp >= 0 {
			name = rest[:sp]
		}
		switch name {
		case "target":
			return "destination ID is required\nusage: mesh-node send rreq <target-id>\n       mesh-node send msg <target-id> \"message\""
		case "msg":
			return "message text is required\nusage: mesh-node send msg <target-id> \"message\""
		default:
			return fmt.Sprintf("missing required argument %q", name)
		}
	}
	return msg
}

func splitInterfaces(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func daemonOutput(stdout, stderr *bytes.Buffer) string {
	var b strings.Builder
	if stderr.Len() > 0 {
		b.WriteString(stderr.String())
	}
	if stdout.Len() > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(stdout.String())
	}
	return b.String()
}

func pidFilePath() string {
	return filepath.Join(os.TempDir(), "mesh-node.pid")
}

func readPIDFile() (int, error) {
	data, err := os.ReadFile(pidFilePath())
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid file")
	}
	return pid, nil
}

func writePIDFile() error {
	if pid, err := readPIDFile(); err == nil && pid != os.Getpid() && pidIsLiveMesh(pid) {
		return fmt.Errorf("mesh-node is already running (pid %d); stop it with: mesh-node kill", pid)
	}
	return os.WriteFile(pidFilePath(), []byte(strconv.Itoa(os.Getpid())), 0o600)
}

func removePIDFile() {
	pid, err := readPIDFile()
	if err != nil || pid != os.Getpid() {
		return
	}
	_ = os.Remove(pidFilePath())
}

func pidIsLiveMesh(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS != "windows" {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return false
		}
	}
	if runtime.GOOS == "linux" {
		return cmdlineMatchesUs(pid)
	}
	return true
}

func cmdlineMatchesUs(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return false
	}
	argv0 := string(data)
	if i := strings.IndexByte(argv0, 0); i >= 0 {
		argv0 = argv0[:i]
	}
	exe, err := os.Executable()
	if err != nil {
		return true
	}
	return filepath.Clean(argv0) == filepath.Clean(exe) || filepath.Base(argv0) == filepath.Base(exe)
}

func stopByPID() error {
	pid, err := readPIDFile()
	if err != nil || !pidIsLiveMesh(pid) {
		_ = os.Remove(pidFilePath())
		return fmt.Errorf("no background node is running")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	var sigErr error
	if runtime.GOOS == "windows" {
		sigErr = proc.Kill()
	} else {
		sigErr = proc.Signal(syscall.SIGTERM)
	}
	if sigErr != nil {
		return fmt.Errorf("couldn't stop process %d: %w", pid, sigErr)
	}
	_ = os.Remove(pidFilePath())
	_, _ = fmt.Fprintf(os.Stdout, "Stopped background node (pid %d)\n", pid)
	return nil
}
