package worker

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/YatinKare/map-data-fetcher/gateway/internal/config"
)

// Supervisor controls the lifecycle of the Java worker behind the gateway.
type Supervisor interface {
	Acquire(context.Context) (func(), error)
	Shutdown(context.Context) error
}

// Manager starts the Java worker on demand and stops it after an idle period.
type Manager struct {
	config     config.Config
	httpClient *http.Client
	logger     *log.Logger

	lifecycleMu sync.Mutex
	stateMu     sync.Mutex
	command     *exec.Cmd
	waitDone    chan error

	activeRequests int
	lastActivity   time.Time
	closed         bool

	monitorCancel context.CancelFunc
	monitorDone   chan struct{}
}

// NewManager creates a worker manager and starts its idle monitor.
func NewManager(cfg config.Config, httpClient *http.Client, logger *log.Logger) *Manager {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = log.Default()
	}

	monitorContext, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		config:        cfg,
		httpClient:    httpClient,
		logger:        logger,
		monitorCancel: cancel,
		monitorDone:   make(chan struct{}),
		lastActivity:  time.Now(),
	}
	go manager.idleLoop(monitorContext)

	return manager
}

// Acquire ensures that the worker is ready and records one active request.
func (m *Manager) Acquire(ctx context.Context) (func(), error) {
	if err := m.ensureRunning(ctx); err != nil {
		return nil, err
	}

	m.stateMu.Lock()
	if m.closed {
		m.stateMu.Unlock()
		return nil, fmt.Errorf("Java worker manager is shut down")
	}
	m.activeRequests++
	m.lastActivity = time.Now()
	m.stateMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(m.release)
	}, nil
}

func (m *Manager) release() {
	m.stateMu.Lock()
	if m.activeRequests > 0 {
		m.activeRequests--
	}
	m.lastActivity = time.Now()
	m.stateMu.Unlock()
}

func (m *Manager) ensureRunning(ctx context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	m.stateMu.Lock()
	if m.closed {
		m.stateMu.Unlock()
		return fmt.Errorf("Java worker manager is shut down")
	}
	if m.command != nil {
		m.stateMu.Unlock()
		return nil
	}
	m.stateMu.Unlock()

	return m.start(ctx)
}

func (m *Manager) start(ctx context.Context) error {
	command := buildJavaCommand(m.config)
	if err := command.Start(); err != nil {
		return fmt.Errorf("failed to start Java worker: %w", err)
	}

	waitDone := make(chan error, 1)
	m.stateMu.Lock()
	m.command = command
	m.waitDone = waitDone
	m.stateMu.Unlock()

	go func() {
		err := command.Wait()
		waitDone <- err

		m.stateMu.Lock()
		if m.command == command {
			m.command = nil
			m.waitDone = nil
		}
		m.stateMu.Unlock()

		if normalizedErr := normalizeProcessExit(err); normalizedErr != nil {
			m.logger.Printf("Java worker exited: %v", normalizedErr)
		}
	}()

	if err := m.waitUntilReady(ctx); err != nil {
		shutdownContext, cancel := context.WithTimeout(
			context.Background(), m.config.JavaShutdownTimeout,
		)
		defer cancel()
		_ = terminateProcessGroup(shutdownContext, command, waitDone, m.config.JavaShutdownTimeout)

		m.stateMu.Lock()
		if m.command == command {
			m.command = nil
			m.waitDone = nil
		}
		m.stateMu.Unlock()
		return err
	}

	return nil
}

func (m *Manager) waitUntilReady(ctx context.Context) error {
	startupContext, cancel := context.WithTimeout(ctx, m.config.JavaStartupTimeout)
	defer cancel()

	healthURL := workerHealthURL(m.config.JavaBaseURL)
	for {
		request, err := http.NewRequestWithContext(startupContext, http.MethodGet, healthURL.String(), nil)
		if err == nil {
			response, requestErr := m.httpClient.Do(request)
			if requestErr == nil {
				response.Body.Close()
				if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
					return nil
				}
			}
		}

		select {
		case <-startupContext.Done():
			return fmt.Errorf("Java worker did not become ready: %w", startupContext.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (m *Manager) idleLoop(ctx context.Context) {
	interval := m.config.JavaIdleTimeout / 4
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	if interval > time.Minute {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(m.monitorDone)

	for {
		select {
		case <-ticker.C:
			m.stopIfIdle(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) stopIfIdle(ctx context.Context) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	m.stateMu.Lock()
	idle := m.command != nil &&
		m.activeRequests == 0 &&
		time.Since(m.lastActivity) >= m.config.JavaIdleTimeout
	m.stateMu.Unlock()
	if !idle {
		return
	}

	if err := m.stopCurrent(ctx); err != nil {
		m.logger.Printf("failed to stop idle Java worker: %v", err)
	} else {
		m.logger.Printf("stopped Java worker after %s of inactivity", m.config.JavaIdleTimeout)
	}
}

func (m *Manager) stopCurrent(ctx context.Context) error {
	m.stateMu.Lock()
	command := m.command
	waitDone := m.waitDone
	m.stateMu.Unlock()
	if command == nil || waitDone == nil {
		return nil
	}

	err := terminateProcessGroup(ctx, command, waitDone, m.config.JavaShutdownTimeout)
	m.stateMu.Lock()
	if m.command == command {
		m.command = nil
		m.waitDone = nil
	}
	m.stateMu.Unlock()
	return err
}

// Shutdown stops the idle monitor and terminates the worker process group.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.stateMu.Lock()
	if m.closed {
		m.stateMu.Unlock()
		return nil
	}
	m.closed = true
	m.stateMu.Unlock()

	m.monitorCancel()
	select {
	case <-m.monitorDone:
	case <-ctx.Done():
		return ctx.Err()
	}

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.stopCurrent(ctx)
}

func buildJavaCommand(cfg config.Config) *exec.Cmd {
	args := make([]string, 0, 8)
	if cfg.ChromeDriverBin != "" {
		args = append(args, "-Dwebdriver.chrome.driver="+cfg.ChromeDriverBin)
	}
	args = append(args, "-jar", cfg.JavaJar)
	args = append(args, "--server.address=127.0.0.1")
	args = append(args, "--server.port="+javaPort(cfg.JavaBaseURL))
	if cfg.JavaHeadless {
		args = append(args, "--naver.map.selenium.headless=true")
	}

	command := exec.Command(cfg.JavaBin, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return command
}

func workerHealthURL(baseURL *url.URL) *url.URL {
	healthURL := *baseURL
	healthURL.Path = path.Join("/", baseURL.Path, "healthz")
	healthURL.RawQuery = ""
	healthURL.Fragment = ""
	return &healthURL
}

func javaPort(baseURL *url.URL) string {
	if port := baseURL.Port(); port != "" {
		return port
	}
	if baseURL.Scheme == "https" {
		return strconv.Itoa(443)
	}
	return strconv.Itoa(80)
}
