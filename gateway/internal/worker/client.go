package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

const maxNaverResponseBytes = 10 << 20

// ErrSearchBusy indicates that another Naver search currently owns the worker.
var ErrSearchBusy = errors.New("A Naver search is already running. Retry after it finishes.")

// SearchMetadata contains timing details safe to summarize in gateway logs.
type SearchMetadata struct {
	WorkerColdStart       bool
	WorkerStartupDuration time.Duration
	SearchDuration        time.Duration
}

// ToolFailure carries bounded labels for privacy-safe tool outcome logging.
type ToolFailure struct {
	Category string
	Stage    string
	Err      error
}

func (e *ToolFailure) Error() string { return e.Err.Error() }
func (e *ToolFailure) Unwrap() error { return e.Err }

type workerErrorResponse struct {
	Category string `json:"category"`
	Stage    string `json:"stage"`
}

// FailureDetails returns controlled labels without exposing the error text.
func FailureDetails(err error) (category, stage string) {
	if errors.Is(err, ErrSearchBusy) {
		return "search_busy", "worker_slot"
	}
	var failure *ToolFailure
	if errors.As(err, &failure) {
		return failure.Category, failure.Stage
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled", "worker_request"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", "worker_request"
	}
	return "worker_error", "worker_request"
}

// JavaClient calls the private Java worker and coordinates its lifecycle.
type JavaClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	supervisor Supervisor
	searchSlot chan struct{}
}

// NewJavaClient creates a client for the private Java worker.
func NewJavaClient(baseURL *url.URL, httpClient *http.Client, supervisor Supervisor) *JavaClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &JavaClient{
		baseURL:    baseURL,
		httpClient: httpClient,
		supervisor: supervisor,
		searchSlot: make(chan struct{}, 1),
	}
}

// SearchNaver retrieves the raw JSON returned by Java.
func (c *JavaClient) SearchNaver(ctx context.Context, query string, page int) ([]byte, SearchMetadata, error) {
	queryValues := make(url.Values)
	queryValues.Set("q", query)
	queryValues.Set("page", strconv.Itoa(page))
	return c.getNaverJSON(ctx, "api/naver-map/search", queryValues)
}

// SearchNaverByCoordinate retrieves raw JSON from Java's coordinate search endpoint.
func (c *JavaClient) SearchNaverByCoordinate(
	ctx context.Context,
	query string,
	longitude float64,
	latitude float64,
	page int,
) ([]byte, SearchMetadata, error) {
	queryValues := make(url.Values)
	queryValues.Set("query", query)
	queryValues.Set("longitude", strconv.FormatFloat(longitude, 'f', -1, 64))
	queryValues.Set("latitude", strconv.FormatFloat(latitude, 'f', -1, 64))
	queryValues.Set("page", strconv.Itoa(page))
	return c.getNaverJSON(ctx, "api/naver-map/coordinate", queryValues)
}

func (c *JavaClient) getNaverJSON(
	ctx context.Context,
	endpoint string,
	queryValues url.Values,
) (body []byte, metadata SearchMetadata, resultErr error) {
	select {
	case c.searchSlot <- struct{}{}:
		defer func() { <-c.searchSlot }()
	default:
		return nil, metadata, ErrSearchBusy
	}

	var release func()
	if c.supervisor != nil {
		lease, err := c.supervisor.Acquire(ctx)
		metadata.WorkerColdStart = lease.ColdStart
		metadata.WorkerStartupDuration = lease.StartupDuration
		if err != nil {
			category, _ := FailureDetails(err)
			if category == "worker_error" {
				category = "worker_startup"
			}
			return nil, metadata, &ToolFailure{Category: category, Stage: "worker_startup", Err: err}
		}
		release = lease.Release
		defer release()
	}

	target := *c.baseURL
	target.Path = path.Join("/", target.Path, endpoint)
	target.RawQuery = queryValues.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, metadata, &ToolFailure{Category: "worker_request", Stage: "request_creation", Err: fmt.Errorf("create Naver worker request: %w", err)}
	}
	request.Header.Set("Accept", "application/json")

	searchStarted := time.Now()
	defer func() { metadata.SearchDuration = time.Since(searchStarted) }()
	response, err := c.httpClient.Do(request)
	if err != nil {
		category := "worker_call"
		if errors.Is(err, context.Canceled) {
			category = "cancelled"
		} else if errors.Is(err, context.DeadlineExceeded) {
			category = "timeout"
		}
		return nil, metadata, &ToolFailure{Category: category, Stage: "worker_request", Err: fmt.Errorf("call Naver worker: %w", err)}
	}
	defer response.Body.Close()

	body, err = readJSONResponse(response.Body)
	if err != nil {
		category := "invalid_worker_response"
		if errors.Is(err, context.Canceled) {
			category = "cancelled"
		} else if errors.Is(err, context.DeadlineExceeded) {
			category = "timeout"
		}
		return nil, metadata, &ToolFailure{Category: category, Stage: "response_body", Err: err}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var workerError workerErrorResponse
		if json.Unmarshal(body, &workerError) == nil && validWorkerFailureLabels(workerError.Category, workerError.Stage) {
			return nil, metadata, &ToolFailure{
				Category: workerError.Category,
				Stage:    workerError.Stage,
				Err:      fmt.Errorf("Naver search failed (%s at %s)", workerError.Category, workerError.Stage),
			}
		}
		return nil, metadata, &ToolFailure{Category: "worker_http_error", Stage: "worker_response", Err: fmt.Errorf("Naver worker returned HTTP %d", response.StatusCode)}
	}

	return body, metadata, nil
}

func validWorkerFailureLabels(category, stage string) bool {
	validCategories := map[string]bool{
		"capture_error":    true,
		"request_aborted":  true,
		"invalid_response": true,
		"timeout":          true,
		"cancelled":        true,
	}
	validStages := map[string]bool{
		"browser_startup":   true,
		"page_navigation":   true,
		"search_input":      true,
		"response_matching": true,
		"response_body":     true,
		"pagination":        true,
	}
	return validCategories[category] && validStages[stage]
}

func readJSONResponse(body io.Reader) ([]byte, error) {
	contents, err := io.ReadAll(io.LimitReader(body, maxNaverResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Naver worker response: %w", err)
	}
	if len(contents) > maxNaverResponseBytes {
		return nil, fmt.Errorf("Naver worker response exceeds %d bytes", maxNaverResponseBytes)
	}
	if !json.Valid(contents) {
		return nil, fmt.Errorf("Naver worker returned invalid JSON")
	}

	return contents, nil
}
