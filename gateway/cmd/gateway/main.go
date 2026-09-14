package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/YatinKare/map-data-fetcher/gateway/internal/config"
	"github.com/YatinKare/map-data-fetcher/gateway/internal/server"
)

func main() {
	cfg, err := config.LoadFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("invalid gateway configuration: %v", err)
	}

	gateway := server.New(cfg)
	if err := gateway.Start(); err != nil {
		log.Fatalf("failed to start gateway: %v", err)
	}

	log.Printf("gateway listening on %s", gateway.Address())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case signal := <-signals:
		log.Printf("received %s; shutting down", signal)
	case err := <-gateway.Errors():
		log.Fatalf("gateway server failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := gateway.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shut down gateway: %v", err)
	}
}
