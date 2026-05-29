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

func registerUserTools(s *server.MCPServer, lc *ldapclient.Client, engine *yamlengine.Engine) {
	s.AddTool(mcp.NewTool("ad.search_user",
		mcp.WithDescription("Search Active Directory users by name, email, or username."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Name, email, or sAMAccountName to search for")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := admcppolicy.Authorize(ctx, engine, "ad.search_user", mcppolicy.Resource{Type: "ad:user"}, ""); err != nil {
			return toolErr(err), nil
		}
		query := req.GetString("query", "")
		users, err := lc.SearchUsers(query)
		if err != nil {
			return toolErr(err), nil
		}
		if len(users) == 0 {
			return toolText("No users found matching: " + query), nil
		}
		out := fmt.Sprintf("Found %d user(s):\n\n", len(users))
		for _, u := range users {
			out += fmt.Sprintf("- %s (%s) — %s | %s | OU: %s\n",
				u.DisplayName, u.SamAccountName, u.Email, u.Department, u.OU)
			if u.Disabled {
				out += "  ⚠ DISABLED\n"
			}
			if u.Locked {
				out += "  ⚠ LOCKED\n"
			}
		}
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.get_user",
		mcp.WithDescription("Get full details for an AD user by sAMAccountName or distinguished name."),
		mcp.WithString("user_id", mcp.Required(), mcp.Description("sAMAccountName or distinguished name")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("user_id", "")
		if err := admcppolicy.Authorize(ctx, engine, "ad.get_user", mcppolicy.Resource{Type: "ad:user", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		u, err := lc.GetUser(id)
		if err != nil {
			return toolErr(err), nil
		}
		out := fmt.Sprintf("User: %s (%s)\n", u.DisplayName, u.SamAccountName)
		out += fmt.Sprintf("Email:      %s\n", u.Email)
		out += fmt.Sprintf("Department: %s\n", u.Department)
		out += fmt.Sprintf("Title:      %s\n", u.Title)
		out += fmt.Sprintf("OU:         %s\n", u.OU)
		out += fmt.Sprintf("Manager DN: %s\n", u.ManagerDN)
		out += fmt.Sprintf("DN:         %s\n\n", u.DN)
		out += "Status:\n"
		out += fmt.Sprintf("  Disabled:          %v\n", u.Disabled)
		out += fmt.Sprintf("  Locked:            %v\n", u.Locked)
		out += fmt.Sprintf("  Password never exp:%v\n", u.PasswordNeverExp)
		out += fmt.Sprintf("  Must change pwd:   %v\n", u.MustChangePwd)
		out += fmt.Sprintf("  Bad pwd count:     %d\n", u.BadPwdCount)
		if u.LastLogon != nil {
			out += fmt.Sprintf("  Last logon:        %s\n", u.LastLogon.Format("2006-01-02 15:04 UTC"))
		} else {
			out += "  Last logon:        never\n"
		}
		if u.AccountExpires != nil {
			out += fmt.Sprintf("  Account expires:   %s\n", u.AccountExpires.Format("2006-01-02"))
		} else {
			out += "  Account expires:   never\n"
		}
		if u.PwdLastSet != nil {
			out += fmt.Sprintf("  Password last set: %s\n", u.PwdLastSet.Format("2006-01-02"))
		}
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.get_user_groups",
		mcp.WithDescription("List all groups a user belongs to."),
		mcp.WithString("user_id", mcp.Required(), mcp.Description("sAMAccountName or distinguished name of the user")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dn := req.GetString("user_id", "")
		if err := admcppolicy.Authorize(ctx, engine, "ad.get_user_groups", mcppolicy.Resource{Type: "ad:user", ID: dn}, ""); err != nil {
			return toolErr(err), nil
		}
		groups, err := lc.GetUserGroups(dn)
		if err != nil {
			return toolErr(err), nil
		}
		if len(groups) == 0 {
			return toolText("User is not a member of any groups."), nil
		}
		out := fmt.Sprintf("Member of %d group(s):\n\n", len(groups))
		for _, g := range groups {
			out += fmt.Sprintf("- %s (OU: %s)\n", g.Name, g.OU)
		}
		return toolText(out), nil
	})
}
