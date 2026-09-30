package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/YatinKare/map-data-fetcher/gateway/internal/mcpserver"
	"github.com/google/jsonschema-go/jsonschema"
)

const maxCapturedMCPBytes = 64 << 10

type requestCorrelationKey struct{}

type requestLogger struct {
	logger  *log.Logger
	enabled atomic.Bool
}

func newRequestLogger(logger *log.Logger) *requestLogger {
	return &requestLogger{logger: logger}
}

func (l *requestLogger) toggle() bool {
	for {
		current := l.enabled.Load()
		if l.enabled.CompareAndSwap(current, !current) {
			return !current
		}
	}
}

func (l *requestLogger) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !l.enabled.Load() {
			next.ServeHTTP(response, request)
			return
		}
		correlation := correlationID(request)
		request = request.WithContext(context.WithValue(request.Context(), requestCorrelationKey{}, correlation))

		started := time.Now()
		requestBody := &capturedBody{ReadCloser: request.Body}
		request.Body = requestBody
		responseCapture := &capturedResponseWriter{ResponseWriter: response}
		next.ServeHTTP(responseCapture, request)

		if responseCapture.status == 0 {
			responseCapture.status = http.StatusOK
		}
		entry := makeMCPLogEntry(request, requestBody.contents, requestBody.size, responseCapture)
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
		entry.Duration = time.Since(started).String()
		entry.Status = responseCapture.status
		entry.ResponseBytes = responseCapture.size
		entry.ResponseContentType = safeValue(response.Header().Get("Content-Type"), 100)
		l.logger.Printf("MCP request %s", encodeLogEntry(entry))
	})
}

// LogToolOutcome writes a structured per-tool summary while outcome logging is enabled.
func (l *requestLogger) LogToolOutcome(ctx context.Context, outcome mcpserver.ToolOutcome) {
	if !l.enabled.Load() {
		return
	}
	entry := toolOutcomeLogEntry{
		Timestamp:             time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID:         correlationFromContext(ctx),
		ToolName:              safeValue(outcome.ToolName, 80),
		IsError:               outcome.IsError,
		DurationMS:            durationMilliseconds(outcome.Duration),
		WorkerColdStart:       outcome.WorkerColdStart,
		WorkerStartupDuration: durationMilliseconds(outcome.WorkerStartupDuration),
		SearchDuration:        durationMilliseconds(outcome.SearchDuration),
		ResultCount:           outcome.ResultCount,
		ResponseBytes:         outcome.ResponseBytes,
		FailureCategory:       safeValue(outcome.FailureCategory, 40),
		FailureStage:          safeValue(outcome.FailureStage, 40),
	}
	l.logger.Printf("MCP tool outcome %s", encodeLogEntry(entry))
}

type toolOutcomeLogEntry struct {
	Timestamp             string  `json:"timestamp"`
	CorrelationID         string  `json:"correlation_id,omitempty"`
	ToolName              string  `json:"tool_name"`
	IsError               bool    `json:"is_error"`
	DurationMS            float64 `json:"duration_ms"`
	WorkerColdStart       bool    `json:"worker_cold_start"`
	WorkerStartupDuration float64 `json:"worker_startup_ms"`
	SearchDuration        float64 `json:"search_ms"`
	ResultCount           *int    `json:"result_count,omitempty"`
	ResponseBytes         int     `json:"response_bytes"`
	FailureCategory       string  `json:"failure_category,omitempty"`
	FailureStage          string  `json:"failure_stage,omitempty"`
}

func durationMilliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func correlationFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestCorrelationKey{}).(string)
	return value
}

type mcpLogEntry struct {
	Timestamp           string          `json:"timestamp"`
	CorrelationID       string          `json:"correlation_id"`
	RequestID           string          `json:"request_id,omitempty"`
	SessionIDHash       string          `json:"session_id_hash,omitempty"`
	Method              string          `json:"method"`
	Path                string          `json:"path"`
	Status              int             `json:"http_status"`
	Duration            string          `json:"duration"`
	RequestBytes        int64           `json:"request_bytes"`
	ResponseBytes       int64           `json:"response_bytes"`
	ProtocolVersion     string          `json:"protocol_version,omitempty"`
	NegotiatedVersion   string          `json:"negotiated_protocol_version,omitempty"`
	RequestContentType  string          `json:"request_content_type,omitempty"`
	ResponseContentType string          `json:"response_content_type,omitempty"`
	RPCMethod           string          `json:"rpc_method,omitempty"`
	ErrorCode           string          `json:"error_code,omitempty"`
	ErrorMessage        string          `json:"error_message,omitempty"`
	ClientInfo          string          `json:"client_info,omitempty"`
	ServerInfo          string          `json:"server_info,omitempty"`
	ToolSchemas         map[string]bool `json:"tool_schemas,omitempty"`
}

func makeMCPLogEntry(request *http.Request, requestBody []byte, requestSize int64, response *capturedResponseWriter) mcpLogEntry {
	var requestEnvelope struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			ProtocolVersion string `json:"protocolVersion"`
			ClientInfo      any    `json:"clientInfo"`
		} `json:"params"`
	}
	_ = json.Unmarshal(requestBody, &requestEnvelope)

	entry := mcpLogEntry{
		CorrelationID:      correlationID(request),
		RequestID:          safeRequestID(requestEnvelope.ID),
		SessionIDHash:      hashValue(firstNonEmpty(request.Header.Get("Mcp-Session-Id"), response.Header().Get("Mcp-Session-Id"))),
		Method:             request.Method,
		Path:               request.URL.Path,
		RequestBytes:       requestSize,
		ProtocolVersion:    safeValue(firstNonEmpty(request.Header.Get("Mcp-Protocol-Version"), requestEnvelope.Params.ProtocolVersion), 40),
		RequestContentType: safeValue(request.Header.Get("Content-Type"), 100),
		RPCMethod:          safeValue(requestEnvelope.Method, 80),
	}
	if requestEnvelope.Params.ClientInfo != nil {
		entry.ClientInfo = compactSafeJSON(requestEnvelope.Params.ClientInfo)
	}

	var responseEnvelope struct {
		Error  json.RawMessage `json:"error"`
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      any    `json:"serverInfo"`
			Tools           []struct {
				Name        string          `json:"name"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if json.Unmarshal(response.body, &responseEnvelope) == nil {
		entry.NegotiatedVersion = safeValue(responseEnvelope.Result.ProtocolVersion, 40)
		if responseEnvelope.Result.ServerInfo != nil {
			entry.ServerInfo = compactSafeJSON(responseEnvelope.Result.ServerInfo)
		}
		if requestEnvelope.Method == "tools/list" && len(responseEnvelope.Result.Tools) > 0 {
			entry.ToolSchemas = make(map[string]bool, len(responseEnvelope.Result.Tools))
			for _, tool := range responseEnvelope.Result.Tools {
				name := safeValue(tool.Name, 80)
				if name != "" {
					entry.ToolSchemas[name] = validJSONSchema(tool.InputSchema)
				}
			}
		}
		if len(responseEnvelope.Error) > 0 {
			var rpcError struct {
				Code    json.RawMessage `json:"code"`
				Message string          `json:"message"`
			}
			if json.Unmarshal(responseEnvelope.Error, &rpcError) == nil && rpcError.Message != "" {
				entry.ErrorCode = safeValue(string(rpcError.Code), 40)
				entry.ErrorMessage = safeValue(rpcError.Message, 200)
			} else {
				var legacy string
				if json.Unmarshal(responseEnvelope.Error, &legacy) == nil {
					entry.ErrorMessage = safeValue(legacy, 200)
				}
			}
		}
	} else if response.status >= http.StatusBadRequest {
		entry.ErrorMessage = safeValue(string(response.body), 200)
	}
	if response.status >= http.StatusBadRequest && entry.ErrorCode == "" {
		entry.ErrorCode = fmt.Sprintf("http_%d", response.status)
	}
	return entry
}

func validJSONSchema(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var schema jsonschema.Schema
	if json.Unmarshal(raw, &schema) != nil {
		return false
	}
	_, err := schema.Resolve(nil)
	return err == nil
}

func safeRequestID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return safeValue(text, 80)
	}
	return safeValue(string(raw), 80)
}

func correlationID(request *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		remoteHost = request.RemoteAddr
	}
	return hashValue(remoteHost + "\x00" + request.UserAgent())
}

func hashValue(value string) string {
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

func compactSafeJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return safeValue(string(encoded), 160)
}

func safeValue(value string, maxLength int) string {
	value = strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f {
			return ' '
		}
		return character
	}, value)
	value = strings.TrimSpace(value)
	if len(value) > maxLength {
		value = value[:maxLength]
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func encodeLogEntry(entry any) string {
	encoded, err := json.Marshal(entry)
	if err != nil {
		return `{"encoding_error":true}`
	}
	return string(encoded)
}

type capturedBody struct {
	io.ReadCloser
	contents []byte
	size     int64
}

func (b *capturedBody) Read(buffer []byte) (int, error) {
	n, err := b.ReadCloser.Read(buffer)
	b.size += int64(n)
	remaining := maxCapturedMCPBytes - len(b.contents)
	if n > 0 && remaining > 0 {
		captured := n
		if captured > remaining {
			captured = remaining
		}
		b.contents = append(b.contents, buffer[:captured]...)
	}
	return n, err
}

type capturedResponseWriter struct {
	http.ResponseWriter
	status int
	size   int64
	body   []byte
}

func (w *capturedResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *capturedResponseWriter) Write(contents []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(contents)
	w.size += int64(n)
	remaining := maxCapturedMCPBytes - len(w.body)
	if n > 0 && remaining > 0 {
		captured := n
		if captured > remaining {
			captured = remaining
		}
		w.body = append(w.body, contents[:captured]...)
	}
	return n, err
}

func (w *capturedResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *capturedResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
