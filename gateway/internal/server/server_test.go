package server

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/YatinKare/map-data-fetcher/gateway/internal/config"
)

const testAuthToken = "test-token"

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
		AuthToken:       testAuthToken,
		JavaBaseURL:     mustParseURL(t, "http://127.0.0.1:8080"),
		ServiceName:     "test-gateway",
		Version:         "test",
		ShutdownTimeout: time.Second,
	})
	if err := gateway.Start(); err != nil {
		t.Fatalf("server start failed: %v", err)
	}

	return gateway
}

func mustParseURL(t *testing.T, value string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("failed to parse URL %q: %v", value, err)
	}
	return parsed
}

func shutdownTestServer(t *testing.T, gateway *Server) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := gateway.Shutdown(ctx); err != nil {
		t.Fatalf("server cleanup failed: %v", err)
	}
}
