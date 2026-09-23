package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	AuthModeRequired           = "required"
	AuthModeNone               = "none"
	defaultHost                = "127.0.0.1"
	defaultPort                = 3000
	defaultServiceName         = "map-data-gateway"
	defaultVersion             = "0.1.0"
	defaultShutdownTimeout     = 5 * time.Second
	defaultJavaBaseURL         = "http://127.0.0.1:8080"
	defaultJavaBin             = "java"
	defaultJavaIdleTimeout     = 5 * time.Minute
	defaultJavaStartupTimeout  = 90 * time.Second
	defaultJavaShutdownTimeout = 15 * time.Second
)

// Config contains the runtime settings for the gateway process.
type Config struct {
	Host                string
	Port                int
	AuthMode            string
	AuthToken           string
	JavaBaseURL         *url.URL
	JavaBin             string
	JavaJar             string
	ChromeDriverBin     string
	JavaHeadless        bool
	JavaIdleTimeout     time.Duration
	JavaStartupTimeout  time.Duration
	JavaShutdownTimeout time.Duration
	ServiceName         string
	Version             string
	ShutdownTimeout     time.Duration
}

// LoadFromEnv loads gateway configuration from environment variables.
func LoadFromEnv(getenv func(string) string) (Config, error) {
	authMode := withDefault(getenv("GATEWAY_AUTH_MODE"), AuthModeRequired)
	if authMode != AuthModeRequired && authMode != AuthModeNone {
		return Config{}, fmt.Errorf("GATEWAY_AUTH_MODE must be %q or %q", AuthModeRequired, AuthModeNone)
	}

	authToken := strings.TrimSpace(getenv("GATEWAY_AUTH_TOKEN"))
	if authMode == AuthModeRequired && authToken == "" {
		return Config{}, fmt.Errorf("GATEWAY_AUTH_TOKEN must be configured")
	}

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

	javaBaseURL, err := parseBaseURL(getenv("GATEWAY_JAVA_BASE_URL"), defaultJavaBaseURL)
	if err != nil {
		return Config{}, fmt.Errorf("invalid GATEWAY_JAVA_BASE_URL: %w", err)
	}

	javaHeadless, err := parseBool(getenv("GATEWAY_JAVA_HEADLESS"), true)
	if err != nil {
		return Config{}, fmt.Errorf("invalid GATEWAY_JAVA_HEADLESS: %w", err)
	}

	javaIdleTimeout, err := parseDuration(
		getenv("GATEWAY_JAVA_IDLE_TIMEOUT"), defaultJavaIdleTimeout,
	)
	if err != nil {
		return Config{}, fmt.Errorf("invalid GATEWAY_JAVA_IDLE_TIMEOUT: %w", err)
	}

	javaStartupTimeout, err := parseDuration(
		getenv("GATEWAY_JAVA_STARTUP_TIMEOUT"), defaultJavaStartupTimeout,
	)
	if err != nil {
		return Config{}, fmt.Errorf("invalid GATEWAY_JAVA_STARTUP_TIMEOUT: %w", err)
	}

	javaShutdownTimeout, err := parseDuration(
		getenv("GATEWAY_JAVA_SHUTDOWN_TIMEOUT"), defaultJavaShutdownTimeout,
	)
	if err != nil {
		return Config{}, fmt.Errorf("invalid GATEWAY_JAVA_SHUTDOWN_TIMEOUT: %w", err)
	}

	return Config{
		Host:                withDefault(getenv("GATEWAY_HOST"), defaultHost),
		Port:                port,
		AuthMode:            authMode,
		AuthToken:           authToken,
		JavaBaseURL:         javaBaseURL,
		JavaBin:             withDefault(getenv("GATEWAY_JAVA_BIN"), defaultJavaBin),
		JavaJar:             strings.TrimSpace(getenv("GATEWAY_JAVA_JAR")),
		ChromeDriverBin:     strings.TrimSpace(getenv("GATEWAY_JAVA_CHROMEDRIVER")),
		JavaHeadless:        javaHeadless,
		JavaIdleTimeout:     javaIdleTimeout,
		JavaStartupTimeout:  javaStartupTimeout,
		JavaShutdownTimeout: javaShutdownTimeout,
		ServiceName:         withDefault(getenv("GATEWAY_SERVICE_NAME"), defaultServiceName),
		Version:             withDefault(getenv("GATEWAY_VERSION"), defaultVersion),
		ShutdownTimeout:     shutdownTimeout,
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

func parseBool(value string, fallback bool) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("must be true or false")
	}

	return parsed, nil
}

func parseBaseURL(value string, fallback string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("must be an HTTP(S) URL with a host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("must not include a query or fragment")
	}

	return parsed, nil
}

func withDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
