package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	ldapclient "github.com/raychao-oao/ad-mcp/internal/ldap"
)

// Register adds all MVP1 tools to the MCP server.
func Register(s *server.MCPServer, lc *ldapclient.Client) {
	registerUserTools(s, lc)
	registerGroupTools(s, lc)
	registerReportTools(s, lc)
}

func toolText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: text},
		},
	}
}

func toolErr(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
		},
	}
}
