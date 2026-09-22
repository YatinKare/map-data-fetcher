package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const naverSearchToolName = "naver_map_search"

// NaverSearchClient is the private Java worker boundary used by the MCP tool.
type NaverSearchClient interface {
	SearchNaver(context.Context, string, int) ([]byte, error)
}

// NaverSearchInput is the public input schema exposed through MCP.
type NaverSearchInput struct {
	Query string `json:"query" jsonschema:"Naver Maps search query"`
	Page  int    `json:"page,omitempty" jsonschema:"optional result page; defaults to 1; allowed values are 1 through 5"`
}

type naverSearchTool struct {
	client NaverSearchClient
}

// NewHandler creates the stateless Streamable HTTP MCP handler.
func NewHandler(client NaverSearchClient, version string) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "map-data-fetcher", Version: version}, nil)
	tool := &naverSearchTool{client: client}

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        naverSearchToolName,
			Description: "Search Naver Maps and return the current raw extracted JSON.",
		},
		tool.handle,
	)

	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			JSONResponse: true,
			Stateless:    true,
		},
	)
}

func (t *naverSearchTool) handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input NaverSearchInput,
) (*mcp.CallToolResult, any, error) {
	query, page, err := normalizeInput(input)
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

func normalizeInput(input NaverSearchInput) (string, int, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return "", 0, fmt.Errorf("query is required")
	}

	page := input.Page
	if page == 0 {
		page = 1
	}
	if page < 1 || page > 5 {
		return "", 0, fmt.Errorf("page must be between 1 and 5")
	}

	return query, page, nil
}
