package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/songgao/water"
	"golang.org/x/sync/errgroup"
)

func main() {
	tun := setupTUN()
	slog.Info("TUN iface created", "name", tun.Name())
	defer tun.Close()
	
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.Go(
		func() error {
			return readTUN(ctx, tun)
		},
	)
	msgChan := make(chan []byte, 1)
	errGroup.Go(
		func() error {
			return writeTUN(ctx, tun, msgChan)
		},
	)

	if err := errGroup.Wait(); err != nil{
		if errors.Is(err, context.Canceled){
		slog.Error("goroutine in errgroup return a error", "error", err)
		}
	}
	slog.Info("shutting down")
}

func setupTUN() *water.Interface{
	cfg := water.Config{
	DeviceType: water.TUN,			
	}
	tun, err := water.New(cfg)
	if err != nil{
		panic(fmt.Errorf("can't create TUN interface: %v", err))
	}
	return tun
}

func readTUN(ctx context.Context, tun *water.Interface) error{
	for {
		select{
		case <-ctx.Done():
			return ctx.Err()
		default:
			b := make([]byte,1024)
			_, err := tun.Read(b)
			if err != nil{
				return fmt.Errorf("can't read from TUN iface: %v",err)
			}
		}
	}
}

func writeTUN(ctx context.Context, tun *water.Interface, msg <- chan []byte) error{
	for {
		select{
		case <-ctx.Done():
			return ctx.Err()
		case m := <- msg:
			_, err := tun.Write(m)
			if err != nil{
				return fmt.Errorf("can't write to TUN iface: %v",err)
			}
		}
	}
}