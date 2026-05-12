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

	t, err := socket.NewSocket(cfg.AppConfig)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go t.Start(ctx)
	go t.StartHelloSender(ctx)
	go t.ProcessMessages(ctx)

	fmt.Println("Node started. Type 'help' for commands.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
		}

		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		switch command {
		case "broadcast":
			t.Broadcast([]byte(parts[1]))
		case "send":
			if len(parts) < 3 {
				fmt.Println("Usage: send <dstID> <message>")
				continue
			}
			dstID, _ := strconv.ParseUint(parts[1], 10, 64)
			payload := strings.Join(parts[2:], " ")
			t.SendData(dstID, []byte(payload))
		case "status":
			fmt.Printf("Neighbours table:\n%s\n", &routing.NeighboursTable)
			fmt.Printf("Routing table:\n%s\n", &routing.RoutesTable)
		case "exit":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Unknown command. Use: send, status, exit")
		}
	}
}
