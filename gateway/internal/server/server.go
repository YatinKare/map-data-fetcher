package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/YatinKare/map-data-fetcher/gateway/internal/config"
	"github.com/YatinKare/map-data-fetcher/gateway/internal/mcpserver"
	"github.com/YatinKare/map-data-fetcher/gateway/internal/worker"
)

const healthPath = "/healthz"

// Server owns the gateway HTTP listener and its lifecycle.
type Server struct {
	config      config.Config
	httpServer  *http.Server
	listener    net.Listener
	serveErrors chan error
	supervisor  worker.Supervisor
}

// New creates a gateway server without starting its listener.
func New(cfg config.Config) *Server {
	var supervisor worker.Supervisor
	if cfg.JavaJar != "" {
		supervisor = worker.NewManager(cfg, http.DefaultClient, log.Default())
	}

	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(response http.ResponseWriter, request *http.Request) {
		handleHealth(response, request, cfg)
	})
	javaClient := worker.NewJavaClient(cfg.JavaBaseURL, http.DefaultClient, supervisor)
	mcpHandler := mcpserver.NewHandler(javaClient, cfg.Version)
	mux.Handle("/mcp", requireBearerToken(cfg.AuthToken, mcpHandler))

	return &Server{
		config: cfg,
		httpServer: &http.Server{
			Addr:              cfg.Address(),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
		serveErrors: make(chan error, 1),
		supervisor:  supervisor,
	}
}

// Start binds the configured address and serves requests asynchronously.
func (s *Server) Start() error {
	if s.listener != nil {
		return fmt.Errorf("gateway server is already started")
	}

	listener, err := net.Listen("tcp", s.config.Address())
	if err != nil {
		return err
	}

	s.listener = listener
	go func() {
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.serveErrors <- serveErr
		}
	}()

	return nil
}

// Address returns the address currently bound by the server.
func (s *Server) Address() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Errors returns unexpected serving errors after Start succeeds.
func (s *Server) Errors() <-chan error {
	return s.serveErrors
}

// Shutdown gracefully stops accepting requests and waits for active requests.
func (s *Server) Shutdown(ctx context.Context) error {
	httpErr := s.httpServer.Shutdown(ctx)
	if s.supervisor == nil {
		return httpErr
	}

	workerErr := s.supervisor.Shutdown(ctx)
	return errors.Join(httpErr, workerErr)
}

func handleHealth(response http.ResponseWriter, request *http.Request, cfg config.Config) {
	if request.Method != http.MethodGet {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]string{
			"error": "method_not_allowed",
		})
		return
	}

	writeJSON(response, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": cfg.ServiceName,
		"version": cfg.Version,
	})
}

func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(body); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}
