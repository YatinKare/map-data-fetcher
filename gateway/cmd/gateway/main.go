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
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	defer signal.Stop(signals)

	if err := gateway.Start(); err != nil {
		log.Fatalf("failed to start gateway: %v", err)
	}

	log.Printf("gateway listening on %s", gateway.Address())

	for {
		select {
		case receivedSignal := <-signals:
			if receivedSignal == syscall.SIGUSR1 {
				if gateway.ToggleMCPRequestLogging() {
					log.Print("MCP request logging enabled")
				} else {
					log.Print("MCP request logging disabled")
				}
				continue
			}
			log.Printf("received %s; shutting down", receivedSignal)
		case err := <-gateway.Errors():
			log.Fatalf("gateway server failed: %v", err)
		}
		break
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := gateway.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shut down gateway: %v", err)
	}
}
