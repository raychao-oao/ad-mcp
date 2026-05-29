package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	admcppolicy "github.com/raychao-oao/ad-mcp/internal/policy"
	ldapclient "github.com/raychao-oao/ad-mcp/internal/ldap"
	mcppolicy "github.com/raychao-oao/mcp-policy/pkg/policy"
	"github.com/raychao-oao/mcp-policy/pkg/yamlengine"
)

func registerGroupTools(s *server.MCPServer, lc *ldapclient.Client, engine *yamlengine.Engine) {
	s.AddTool(mcp.NewTool("ad.search_group",
		mcp.WithDescription("Search Active Directory groups by name or description."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Group name or description to search for")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := admcppolicy.Authorize(ctx, engine, "ad.search_group", mcppolicy.Resource{Type: "ad:group"}, ""); err != nil {
			return toolErr(err), nil
		}
		query := req.GetString("query", "")
		groups, err := lc.SearchGroups(query)
		if err != nil {
			return toolErr(err), nil
		}
		if len(groups) == 0 {
			return toolText("No groups found matching: " + query), nil
		}
		out := fmt.Sprintf("Found %d group(s):\n\n", len(groups))
		for _, g := range groups {
			out += fmt.Sprintf("- %s (OU: %s, members: %d)\n", g.Name, g.OU, g.MemberCount)
			if g.Description != "" {
				out += fmt.Sprintf("  Description: %s\n", g.Description)
			}
			out += fmt.Sprintf("  DN: %s\n", g.DN)
		}
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.get_group",
		mcp.WithDescription("Get details for an AD group by name (cn) or distinguished name."),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("Group cn or distinguished name")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("group_id", "")
		if err := admcppolicy.Authorize(ctx, engine, "ad.get_group", mcppolicy.Resource{Type: "ad:group", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		g, err := lc.GetGroup(id)
		if err != nil {
			return toolErr(err), nil
		}
		out := fmt.Sprintf("Group: %s\n", g.Name)
		out += fmt.Sprintf("Description: %s\n", g.Description)
		out += fmt.Sprintf("OU:          %s\n", g.OU)
		out += fmt.Sprintf("Members:     %d\n", g.MemberCount)
		out += fmt.Sprintf("DN:          %s\n", g.DN)
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.list_group_members",
		mcp.WithDescription("List direct members of an AD group."),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("Group cn (name) or distinguished name")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dn := req.GetString("group_id", "")
		if err := admcppolicy.Authorize(ctx, engine, "ad.list_group_members", mcppolicy.Resource{Type: "ad:group", ID: dn}, ""); err != nil {
			return toolErr(err), nil
		}
		users, err := lc.ListGroupMembers(dn)
		if err != nil {
			return toolErr(err), nil
		}
		if len(users) == 0 {
			return toolText("Group has no members."), nil
		}
		out := fmt.Sprintf("%d member(s):\n\n", len(users))
		for _, u := range users {
			status := ""
			if u.Disabled {
				status += " [DISABLED]"
			}
			if u.Locked {
				status += " [LOCKED]"
			}
			out += fmt.Sprintf("- %s (%s) — %s%s\n", u.DisplayName, u.SamAccountName, u.Department, status)
		}
		return toolText(out), nil
	})
}
