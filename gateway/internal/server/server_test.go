package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/YatinKare/map-data-fetcher/gateway/internal/config"
)

func TestServerStartsSuccessfully(t *testing.T) {
	gateway := startTestServer(t)
	defer shutdownTestServer(t, gateway)

	if gateway.Address() == "" {
		t.Fatal("expected server to expose a bound address")
	}
}

func TestHealthEndpointReturns200(t *testing.T) {
	gateway := startTestServer(t)
	defer shutdownTestServer(t, gateway)

	response, err := http.Get("http://" + gateway.Address() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected /healthz status %d, got %d", http.StatusOK, response.StatusCode)
	}
}

func TestServerShutsDownSuccessfully(t *testing.T) {
	gateway := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := gateway.Shutdown(ctx); err != nil {
		t.Fatalf("server shutdown failed: %v", err)
	}

	if _, err := http.Get("http://" + gateway.Address() + "/healthz"); err == nil {
		t.Fatal("expected requests to fail after server shutdown")
	}
}

func startTestServer(t *testing.T) *Server {
	t.Helper()

	gateway := New(config.Config{
		Host:            "127.0.0.1",
		Port:            0,
		ServiceName:     "test-gateway",
		Version:         "test",
		ShutdownTimeout: time.Second,
	})
	if err := gateway.Start(); err != nil {
		t.Fatalf("server start failed: %v", err)
	}

	return gateway
}

func shutdownTestServer(t *testing.T, gateway *Server) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := gateway.Shutdown(ctx); err != nil {
		t.Fatalf("server cleanup failed: %v", err)
	}
}
