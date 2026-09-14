package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHost            = "127.0.0.1"
	defaultPort            = 3000
	defaultServiceName     = "map-data-gateway"
	defaultVersion         = "0.1.0"
	defaultShutdownTimeout = 5 * time.Second
)

// Config contains the runtime settings for the gateway process.
type Config struct {
	Host            string
	Port            int
	ServiceName     string
	Version         string
	ShutdownTimeout time.Duration
}

// LoadFromEnv loads gateway configuration from environment variables.
func LoadFromEnv(getenv func(string) string) (Config, error) {
	port, err := parsePort(getenv("GATEWAY_PORT"))
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := parseDuration(
		getenv("GATEWAY_SHUTDOWN_TIMEOUT"), defaultShutdownTimeout,
	)
	if err != nil {
		return Config{}, fmt.Errorf("invalid GATEWAY_SHUTDOWN_TIMEOUT: %w", err)
	}

	return Config{
		Host:            withDefault(getenv("GATEWAY_HOST"), defaultHost),
		Port:            port,
		ServiceName:     withDefault(getenv("GATEWAY_SERVICE_NAME"), defaultServiceName),
		Version:         withDefault(getenv("GATEWAY_VERSION"), defaultVersion),
		ShutdownTimeout: shutdownTimeout,
	}, nil
}

// Address returns the TCP address used by the HTTP server.
func (c Config) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func parsePort(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return defaultPort, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("GATEWAY_PORT must be an integer between 1 and 65535")
	}

	return port, nil
}

func parseDuration(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("duration must be positive, for example 5s")
	}

	return duration, nil
}

func withDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
