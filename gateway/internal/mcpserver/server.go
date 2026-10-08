package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/YatinKare/map-data-fetcher/gateway/internal/worker"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	naverSearchToolName           = "naver_map_search"
	naverCoordinateSearchToolName = "naver_map_coordinate_search"
)

// NaverSearchClient is the private Java worker boundary used by the MCP tool.
type NaverSearchClient interface {
	SearchNaver(context.Context, string, int) ([]byte, worker.SearchMetadata, error)
	SearchNaverByCoordinate(context.Context, string, float64, float64, int) ([]byte, worker.SearchMetadata, error)
}

// ToolOutcome is the payload emitted after one MCP search tool call.
type ToolOutcome struct {
	ToolName              string
	IsError               bool
	Duration              time.Duration
	WorkerColdStart       bool
	WorkerStartupDuration time.Duration
	SearchDuration        time.Duration
	ResultCount           *int
	ResponseBytes         int
	FailureCategory       string
	FailureStage          string
}

// ToolOutcomeLogger receives privacy-safe summaries of MCP tool calls.
type ToolOutcomeLogger interface {
	LogToolOutcome(context.Context, ToolOutcome)
}

// NaverSearchInput is the public input schema exposed through MCP.
type NaverSearchInput struct {
	Query string `json:"query" jsonschema:"Naver Maps search query"`
	Page  int    `json:"page,omitempty" jsonschema:"optional result page; defaults to 1; allowed values are 1 through 5"`
}

// NaverCoordinateSearchInput is the public coordinate-search schema exposed through MCP.
type NaverCoordinateSearchInput struct {
	Query     string   `json:"query" jsonschema:"Naver Maps search query"`
	Longitude *float64 `json:"longitude" jsonschema:"required WGS84 longitude in decimal degrees from -180 to 180"`
	Latitude  *float64 `json:"latitude" jsonschema:"required WGS84 latitude in decimal degrees from -90 to 90"`
	Page      int      `json:"page,omitempty" jsonschema:"optional result page; defaults to 1; allowed values are 1 through 5"`
}

type naverSearchTool struct {
	client NaverSearchClient
	logger ToolOutcomeLogger
}

type naverCoordinateSearchTool struct {
	client NaverSearchClient
	logger ToolOutcomeLogger
}

// NewHandler creates the stateless Streamable HTTP MCP handler.
func NewHandler(client NaverSearchClient, version string, logger ToolOutcomeLogger) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "map-data-fetcher", Version: version}, nil)
	tool := &naverSearchTool{client: client, logger: logger}
	coordinateTool := &naverCoordinateSearchTool{client: client, logger: logger}

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        naverSearchToolName,
			Description: "Search Naver Maps and return compact place results with normalized fields.",
		},
		tool.handle,
	)
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        naverCoordinateSearchToolName,
			Description: "Search Naver Maps around a WGS84 coordinate and return compact place results with normalized fields. Provide longitude and latitude in decimal degrees.",
		},
		coordinateTool.handle,
	)

	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			JSONResponse:               true,
			Stateless:                  true,
			DisableLocalhostProtection: true,
		},
	)
}

func (t *naverSearchTool) handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input NaverSearchInput,
) (*mcp.CallToolResult, any, error) {
	started := time.Now()
	outcome := ToolOutcome{ToolName: naverSearchToolName}
	defer func() {
		outcome.Duration = time.Since(started)
		if t.logger != nil {
			t.logger.LogToolOutcome(ctx, outcome)
		}
	}()

	query, page, err := normalizeKeywordInput(input)
	if err != nil {
		outcome.IsError = true
		outcome.FailureCategory = "invalid_arguments"
		outcome.FailureStage = "input_validation"
		return nil, nil, err
	}

	rawJSON, metadata, err := t.client.SearchNaver(ctx, query, page)
	applySearchMetadata(&outcome, metadata, rawJSON)
	if err != nil {
		outcome.IsError = true
		outcome.FailureCategory, outcome.FailureStage = worker.FailureDetails(err)
		if errors.Is(err, worker.ErrSearchBusy) {
			return searchBusyResult(), nil, nil
		}
		return nil, nil, fmt.Errorf("Naver search failed: %w", err)
	}
	normalizedJSON, err := normalizeNaverResults(rawJSON)
	if err != nil {
		outcome.IsError = true
		outcome.FailureCategory = "invalid_response"
		outcome.FailureStage = "response_normalization"
		return nil, nil, fmt.Errorf("Naver search returned invalid place results: %w", err)
	}

	outcome.ResponseBytes = len(normalizedJSON)
	setResultCount(&outcome, normalizedJSON)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(normalizedJSON)},
		},
	}, nil, nil
}

func (t *naverCoordinateSearchTool) handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input NaverCoordinateSearchInput,
) (*mcp.CallToolResult, any, error) {
	started := time.Now()
	outcome := ToolOutcome{ToolName: naverCoordinateSearchToolName}
	defer func() {
		outcome.Duration = time.Since(started)
		if t.logger != nil {
			t.logger.LogToolOutcome(ctx, outcome)
		}
	}()

	query, longitude, latitude, page, err := normalizeCoordinateInput(input)
	if err != nil {
		outcome.IsError = true
		outcome.FailureCategory = "invalid_arguments"
		outcome.FailureStage = "input_validation"
		return nil, nil, err
	}

	rawJSON, metadata, err := t.client.SearchNaverByCoordinate(ctx, query, longitude, latitude, page)
	applySearchMetadata(&outcome, metadata, rawJSON)
	if err != nil {
		outcome.IsError = true
		outcome.FailureCategory, outcome.FailureStage = worker.FailureDetails(err)
		if errors.Is(err, worker.ErrSearchBusy) {
			return searchBusyResult(), nil, nil
		}
		return nil, nil, fmt.Errorf("Naver coordinate search failed: %w", err)
	}
	normalizedJSON, err := normalizeNaverResults(rawJSON)
	if err != nil {
		outcome.IsError = true
		outcome.FailureCategory = "invalid_response"
		outcome.FailureStage = "response_normalization"
		return nil, nil, fmt.Errorf("Naver coordinate search returned invalid place results: %w", err)
	}

	outcome.ResponseBytes = len(normalizedJSON)
	setResultCount(&outcome, normalizedJSON)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(normalizedJSON)},
		},
	}, nil, nil
}

func applySearchMetadata(outcome *ToolOutcome, metadata worker.SearchMetadata, rawJSON []byte) {
	outcome.WorkerColdStart = metadata.WorkerColdStart
	outcome.WorkerStartupDuration = metadata.WorkerStartupDuration
	outcome.SearchDuration = metadata.SearchDuration
	outcome.ResponseBytes = len(rawJSON)
}

func setResultCount(outcome *ToolOutcome, rawJSON []byte) {
	var results []json.RawMessage
	if json.Unmarshal(rawJSON, &results) == nil {
		count := len(results)
		outcome.ResultCount = &count
	}
}

func searchBusyResult() *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: worker.ErrSearchBusy.Error()},
		},
		IsError: true,
	}
}

func normalizeKeywordInput(input NaverSearchInput) (string, int, error) {
	query, err := normalizeQuery(input.Query)
	if err != nil {
		return "", 0, err
	}

	page, err := normalizePage(input.Page)
	if err != nil {
		return "", 0, err
	}

	return query, page, nil
}

func normalizeCoordinateInput(input NaverCoordinateSearchInput) (string, float64, float64, int, error) {
	query, err := normalizeQuery(input.Query)
	if err != nil {
		return "", 0, 0, 0, err
	}
	if input.Longitude == nil {
		return "", 0, 0, 0, fmt.Errorf("longitude is required")
	}
	if input.Latitude == nil {
		return "", 0, 0, 0, fmt.Errorf("latitude is required")
	}
	if *input.Longitude < -180 || *input.Longitude > 180 {
		return "", 0, 0, 0, fmt.Errorf("longitude must be between -180 and 180")
	}
	if *input.Latitude < -90 || *input.Latitude > 90 {
		return "", 0, 0, 0, fmt.Errorf("latitude must be between -90 and 90")
	}

	page, err := normalizePage(input.Page)
	if err != nil {
		return "", 0, 0, 0, err
	}

	return query, *input.Longitude, *input.Latitude, page, nil
}

func normalizeQuery(value string) (string, error) {
	query := strings.TrimSpace(value)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	return query, nil
}

func normalizePage(page int) (int, error) {
	if page == 0 {
		page = 1
	}
	if page < 1 || page > 5 {
		return 0, fmt.Errorf("page must be between 1 and 5")
	}
	return page, nil
}
