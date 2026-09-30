package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	naverSearchToolName           = "naver_map_search"
	naverCoordinateSearchToolName = "naver_map_coordinate_search"
)

// NaverSearchClient is the private Java worker boundary used by the MCP tool.
type NaverSearchClient interface {
	SearchNaver(context.Context, string, int) ([]byte, error)
	SearchNaverByCoordinate(context.Context, string, float64, float64, int) ([]byte, error)
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
}

type naverCoordinateSearchTool struct {
	client NaverSearchClient
}

// NewHandler creates the stateless Streamable HTTP MCP handler.
func NewHandler(client NaverSearchClient, version string) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "map-data-fetcher", Version: version}, nil)
	tool := &naverSearchTool{client: client}
	coordinateTool := &naverCoordinateSearchTool{client: client}

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        naverSearchToolName,
			Description: "Search Naver Maps and return the current raw extracted JSON.",
		},
		tool.handle,
	)
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        naverCoordinateSearchToolName,
			Description: "Search Naver Maps around a WGS84 coordinate and return the current raw extracted JSON. Provide longitude and latitude in decimal degrees.",
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
	query, page, err := normalizeKeywordInput(input)
	if err != nil {
		return nil, nil, err
	}

	rawJSON, err := t.client.SearchNaver(ctx, query, page)
	if err != nil {
		return nil, nil, fmt.Errorf("Naver search failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(rawJSON)},
		},
	}, nil, nil
}

func (t *naverCoordinateSearchTool) handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input NaverCoordinateSearchInput,
) (*mcp.CallToolResult, any, error) {
	query, longitude, latitude, page, err := normalizeCoordinateInput(input)
	if err != nil {
		return nil, nil, err
	}

	rawJSON, err := t.client.SearchNaverByCoordinate(ctx, query, longitude, latitude, page)
	if err != nil {
		return nil, nil, fmt.Errorf("Naver coordinate search failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(rawJSON)},
		},
	}, nil, nil
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
