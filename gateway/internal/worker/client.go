package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
)

const maxNaverResponseBytes = 10 << 20

// JavaClient calls the private Java worker and coordinates its lifecycle.
type JavaClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	supervisor Supervisor
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
	}
}

// SearchNaver retrieves the raw JSON returned by Java.
func (c *JavaClient) SearchNaver(ctx context.Context, query string, page int) ([]byte, error) {
	var release func()
	if c.supervisor != nil {
		var err error
		release, err = c.supervisor.Acquire(ctx)
		if err != nil {
			return nil, err
		}
		defer release()
	}

	target := *c.baseURL
	target.Path = path.Join("/", target.Path, "api/naver-map/search")
	queryValues := target.Query()
	queryValues.Set("q", query)
	queryValues.Set("page", strconv.Itoa(page))
	target.RawQuery = queryValues.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Naver worker request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Naver worker: %w", err)
	}
	defer response.Body.Close()

	body, err := readJSONResponse(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Naver worker returned HTTP %d: %s", response.StatusCode, body)
	}

	return body, nil
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
