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

var (
	keywordSearchInputSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Required nonblank place, business, or category text to search for on Naver Maps.",
			},
			"page": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     5,
				"default":     1,
				"description": "Result page to return. Defaults to 1; allowed values are 1 through 5.",
			},
		},
		"required":             []string{"query"},
		"additionalProperties": false,
	}
	coordinateSearchInputSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Required nonblank place, business, or category text to search for on Naver Maps.",
			},
			"longitude": map[string]any{
				"type":        "number",
				"minimum":     -180,
				"maximum":     180,
				"description": "Required WGS84 longitude in decimal degrees, from -180 to 180.",
			},
			"latitude": map[string]any{
				"type":        "number",
				"minimum":     -90,
				"maximum":     90,
				"description": "Required WGS84 latitude in decimal degrees, from -90 to 90.",
			},
			"page": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     5,
				"default":     1,
				"description": "Result page to return. Defaults to 1; allowed values are 1 through 5.",
			},
		},
		"required":             []string{"query", "longitude", "latitude"},
		"additionalProperties": false,
	}
	placeSearchOutputSchema = map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"rank": map[string]any{
					"type":        "integer",
					"description": "Place rank on this result page, when supplied by Naver.",
				},
				"id": map[string]any{
					"type":        "string",
					"description": "Naver place identifier.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Place name.",
				},
				"category": map[string]any{
					"type":        "array",
					"description": "Naver place category labels.",
					"items":       map[string]any{"type": "string"},
				},
				"road_address": map[string]any{
					"type":        "string",
					"description": "Road-name address.",
				},
				"coordinates": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"longitude": map[string]any{"type": "number", "description": "WGS84 longitude in decimal degrees."},
						"latitude":  map[string]any{"type": "number", "description": "WGS84 latitude in decimal degrees."},
					},
					"additionalProperties": false,
				},
				"tel": map[string]any{
					"type":        "string",
					"description": "Listed telephone number, falling back to Naver's virtual number when needed.",
				},
				"business_status": map[string]any{
					"type":        "string",
					"description": "Naver's short business status text.",
				},
				"business_hours": map[string]any{
					"type":        "string",
					"description": "Local business hours as HH:mm–HH:mm.",
				},
				"break_time": map[string]any{
					"type":        "string",
					"description": "Local break time as HH:mm–HH:mm.",
				},
				"last_order": map[string]any{
					"type":        "string",
					"description": "Local last-order time as HH:mm.",
				},
				"thumbnail_url": map[string]any{
					"type":   "string",
					"format": "uri",
				},
				"homepage": map[string]any{
					"type":   "string",
					"format": "uri",
				},
				"menu_info": map[string]any{
					"type":        "string",
					"description": "Compact menu text from Naver, when available.",
				},
				"reservation_options": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
						"enum": []string{"reservation", "pre-order", "table", "takeout"},
					},
				},
			},
			"additionalProperties": false,
		},
	}
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
	destructiveHint := false
	openWorldHint := true

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:         naverSearchToolName,
			Title:        "Search Naver Maps by keyword",
			Description:  "Find Naver Maps places from a text query. Use when the user names a place, business, or category, including a location written in prose without numeric coordinates. For a search centered on coordinates the user supplied as numbers, use naver_map_coordinate_search. Returns one page of compact place records.",
			InputSchema:  keywordSearchInputSchema,
			OutputSchema: placeSearchOutputSchema,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: &destructiveHint,
				IdempotentHint:  true,
				OpenWorldHint:   &openWorldHint,
			},
		},
		tool.handle,
	)
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:         naverCoordinateSearchToolName,
			Title:        "Search Naver Maps near coordinates",
			Description:  "Find Naver Maps places near an explicit WGS84 longitude and latitude supplied by the user in decimal degrees. Use this only when both numeric coordinates are available; a location mentioned only in prose is not enough. The query selects the place, business, or category to find near that point. Returns one page of compact place records.",
			InputSchema:  coordinateSearchInputSchema,
			OutputSchema: placeSearchOutputSchema,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: &destructiveHint,
				IdempotentHint:  true,
				OpenWorldHint:   &openWorldHint,
			},
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
		return searchFailureResult("keyword", err), nil, nil
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
	// The SDK creates both structured content and one JSON text fallback.
	return nil, json.RawMessage(normalizedJSON), nil
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
		return searchFailureResult("coordinate", err), nil, nil
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
	// The SDK creates both structured content and one JSON text fallback.
	return nil, json.RawMessage(normalizedJSON), nil
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

func searchFailureResult(searchType string, err error) *mcp.CallToolResult {
	category, _ := worker.FailureDetails(err)
	message := fmt.Sprintf("Naver Maps %s search is temporarily unavailable. Please retry shortly.", searchType)
	if category == "timeout" {
		message = fmt.Sprintf("Naver Maps %s search timed out. Please retry the same search.", searchType)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
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
