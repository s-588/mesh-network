package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/s-588/mesh-network/internal/config"
	"github.com/s-588/mesh-network/internal/logger"
	"github.com/s-588/mesh-network/internal/routing"
	"github.com/s-588/mesh-network/internal/socket"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		panic(err)
	}

	err = logger.SetupSlog(cfg)
	if err != nil {
		panic(err)
	}

	fmt.Fprintln(os.Stdout, "Starting node")
	fmt.Fprintf(os.Stdout, "Using %s", cfg.String())

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

	if cfg.IsDaemon {
		fmt.Fprintf(os.Stdout, "Node started as daemon")
		<-ctx.Done()
		return
	}

	fmt.Fprintln(os.Stdout, "Node started. Type 'help' for commands.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		if !scanner.Scan() {
		}

		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		switch command {
		case "send":
			if len(parts) < 3 {
				fmt.Fprintln(os.Stdout, "Usage: send <dstID> <message>")
				continue
			}
			dstID, _ := strconv.ParseUint(parts[1], 10, 64)
			payload := strings.Join(parts[2:], " ")
			t.SendData(dstID, []byte(payload))
		case "status":
			fmt.Fprintf(os.Stdout, "Neighbours table:\n%s\n", &routing.NeighboursTable)
			fmt.Fprintf(os.Stdout, "Routing table:\n%s\n", &routing.RoutesTable)
		case "exit":
			fmt.Fprintln(os.Stdout, "Exiting...")
			return
		default:
			fmt.Fprintln(os.Stdout, "Unknown command. Use: send, status, exit")
		}
	}
}
