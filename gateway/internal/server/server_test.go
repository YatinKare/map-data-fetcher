package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestGatewayForwardsRequestToJava(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected method %s, got %s", http.MethodPost, request.Method)
		}
		if request.URL.Path != "/api/test" {
			t.Errorf("expected path /api/test, got %s", request.URL.Path)
		}
		if request.URL.RawQuery != "page=1&q=cafe" {
			t.Errorf("expected query page=1&q=cafe, got %s", request.URL.RawQuery)
		}
		if request.Header.Get("X-Test-Header") != "forwarded" {
			t.Errorf("expected custom header to be forwarded")
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("failed to read forwarded body: %v", err)
		}
		if string(body) != "request-body" {
			t.Errorf("expected forwarded body request-body, got %q", body)
		}

		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{"forwarded":true}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("failed to parse upstream URL: %v", err)
	}

	gateway := startTestServerWithUpstream(t, upstreamURL)
	defer shutdownTestServer(t, gateway)

	request, err := http.NewRequest(
		http.MethodPost,
		"http://"+gateway.Address()+"/api/test?page=1&q=cafe",
		strings.NewReader("request-body"),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	request.Header.Set("X-Test-Header", "forwarded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("gateway request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read gateway response: %v", err)
	}
	if string(body) != `{"forwarded":true}` {
		t.Fatalf("expected upstream response body, got %q", body)
	}
}

func TestGatewayReturnsBadGatewayWhenJavaUnavailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {}))
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("failed to parse upstream URL: %v", err)
	}
	upstream.Close()

	gateway := startTestServerWithUpstream(t, upstreamURL)
	defer shutdownTestServer(t, gateway)

	response, err := http.Get("http://" + gateway.Address() + "/api/test")
	if err != nil {
		t.Fatalf("gateway request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read gateway error response: %v", err)
	}
	if !strings.Contains(string(body), `"error":"java_worker_unavailable"`) {
		t.Fatalf("expected Java worker error response, got %q", body)
	}
}

func startTestServer(t *testing.T) *Server {
	return startTestServerWithUpstream(t, mustParseURL(t, "http://127.0.0.1:8080"))
}

func startTestServerWithUpstream(t *testing.T, upstreamURL *url.URL) *Server {
	t.Helper()

	gateway := New(config.Config{
		Host:            "127.0.0.1",
		Port:            0,
		JavaBaseURL:     upstreamURL,
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
