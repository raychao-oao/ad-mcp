package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	ldapclient "github.com/raychao-oao/ad-mcp/internal/ldap"
	"github.com/raychao-oao/mcp-policy/pkg/yamlengine"
)

// Register adds all tools to the MCP server.
func Register(s *server.MCPServer, lc *ldapclient.Client, engine *yamlengine.Engine) {
	registerUserTools(s, lc, engine)
	registerGroupTools(s, lc, engine)
	registerReportTools(s, lc, engine)
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
