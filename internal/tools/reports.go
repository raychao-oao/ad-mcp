package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	ldapclient "github.com/raychao-oao/ad-mcp/internal/ldap"
)

func registerReportTools(s *server.MCPServer, lc *ldapclient.Client) {
	s.AddTool(mcp.NewTool("ad.find_locked_users",
		mcp.WithDescription("Find all currently locked-out AD accounts."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		users, err := lc.FindLockedUsers()
		if err != nil {
			return toolErr(err), nil
		}
		if len(users) == 0 {
			return toolText("No locked accounts found."), nil
		}
		out := fmt.Sprintf("%d locked account(s):\n\n", len(users))
		for _, u := range users {
			out += fmt.Sprintf("- %s (%s) — %s | OU: %s | Bad pwd count: %d\n",
				u.DisplayName, u.SamAccountName, u.Department, u.OU, u.BadPwdCount)
		}
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.find_inactive_users",
		mcp.WithDescription("Find accounts with no logon in the past N days."),
		mcp.WithNumber("days", mcp.Description("Number of days of inactivity (default: 90)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		days := req.GetInt("days", 90)
		users, err := lc.FindInactiveUsers(days)
		if err != nil {
			return toolErr(err), nil
		}
		if len(users) == 0 {
			return toolText(fmt.Sprintf("No accounts inactive for %d+ days.", days)), nil
		}
		out := fmt.Sprintf("%d account(s) inactive for %d+ days:\n\n", len(users), days)
		for _, u := range users {
			logon := "never"
			if u.LastLogon != nil {
				logon = u.LastLogon.Format("2006-01-02")
			}
			status := ""
			if u.Disabled {
				status = " [DISABLED]"
			}
			out += fmt.Sprintf("- %s (%s) — last logon: %s%s\n", u.DisplayName, u.SamAccountName, logon, status)
		}
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.find_disabled_users",
		mcp.WithDescription("Find all disabled AD accounts."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		users, err := lc.FindDisabledUsers()
		if err != nil {
			return toolErr(err), nil
		}
		if len(users) == 0 {
			return toolText("No disabled accounts found."), nil
		}
		out := fmt.Sprintf("%d disabled account(s):\n\n", len(users))
		for _, u := range users {
			logon := "never"
			if u.LastLogon != nil {
				logon = u.LastLogon.Format("2006-01-02")
			}
			out += fmt.Sprintf("- %s (%s) — %s | last logon: %s\n",
				u.DisplayName, u.SamAccountName, u.Department, logon)
		}
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.find_expiring_accounts",
		mcp.WithDescription("Find accounts expiring within the next N days."),
		mcp.WithNumber("days", mcp.Description("Look-ahead window in days (default: 30)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		days := req.GetInt("days", 30)
		users, err := lc.FindExpiringAccounts(days)
		if err != nil {
			return toolErr(err), nil
		}
		if len(users) == 0 {
			return toolText(fmt.Sprintf("No accounts expiring within %d days.", days)), nil
		}
		out := fmt.Sprintf("%d account(s) expiring within %d days:\n\n", len(users), days)
		for _, u := range users {
			exp := "unknown"
			if u.AccountExpires != nil {
				exp = u.AccountExpires.Format("2006-01-02")
			}
			out += fmt.Sprintf("- %s (%s) — expires: %s | %s\n",
				u.DisplayName, u.SamAccountName, exp, u.Department)
		}
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.find_password_never_expires",
		mcp.WithDescription("Find accounts with the DONT_EXPIRE_PASSWORD flag set."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		users, err := lc.FindPasswordNeverExpires()
		if err != nil {
			return toolErr(err), nil
		}
		if len(users) == 0 {
			return toolText("No accounts with password-never-expires flag found."), nil
		}
		out := fmt.Sprintf("%d account(s) with password never expires:\n\n", len(users))
		for _, u := range users {
			status := ""
			if u.Disabled {
				status = " [DISABLED]"
			}
			out += fmt.Sprintf("- %s (%s) — %s%s\n", u.DisplayName, u.SamAccountName, u.Department, status)
		}
		return toolText(out), nil
	})

	s.AddTool(mcp.NewTool("ad.generate_account_review_report",
		mcp.WithDescription("Generate a structured account hygiene report covering locked, inactive, disabled, expiring, and password-never-expires accounts."),
		mcp.WithNumber("inactive_days", mcp.Description("Inactivity threshold in days (default: 90)")),
		mcp.WithNumber("expiring_days", mcp.Description("Expiry look-ahead in days (default: 30)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		inactiveDays := req.GetInt("inactive_days", 90)
		expiringDays := req.GetInt("expiring_days", 30)

		locked, err := lc.FindLockedUsers()
		if err != nil {
			return toolErr(fmt.Errorf("locked users: %w", err)), nil
		}
		inactive, err := lc.FindInactiveUsers(inactiveDays)
		if err != nil {
			return toolErr(fmt.Errorf("inactive users: %w", err)), nil
		}
		expiring, err := lc.FindExpiringAccounts(expiringDays)
		if err != nil {
			return toolErr(fmt.Errorf("expiring accounts: %w", err)), nil
		}
		pwdNoExp, err := lc.FindPasswordNeverExpires()
		if err != nil {
			return toolErr(fmt.Errorf("pwd never expires: %w", err)), nil
		}

		out := "# AD Account Review Report\n\n"
		out += "## Summary\n\n"
		out += "| Category | Count |\n"
		out += "|----------|-------|\n"
		out += "| Locked accounts | " + strconv.Itoa(len(locked)) + " |\n"
		out += "| Inactive accounts (>" + strconv.Itoa(inactiveDays) + "d) | " + strconv.Itoa(len(inactive)) + " |\n"
		out += "| Expiring accounts (<" + strconv.Itoa(expiringDays) + "d) | " + strconv.Itoa(len(expiring)) + " |\n"
		out += "| Password never expires | " + strconv.Itoa(len(pwdNoExp)) + " |\n\n"

		if len(locked) > 0 {
			out += "## Locked Accounts\n\n"
			for _, u := range locked {
				out += fmt.Sprintf("- %s (%s) — %s\n", u.DisplayName, u.SamAccountName, u.Department)
			}
			out += "\n"
		}
		if len(expiring) > 0 {
			out += "## Expiring Soon\n\n"
			for _, u := range expiring {
				exp := ""
				if u.AccountExpires != nil {
					exp = " expires: " + u.AccountExpires.Format("2006-01-02")
				}
				out += fmt.Sprintf("- %s (%s)%s\n", u.DisplayName, u.SamAccountName, exp)
			}
			out += "\n"
		}
		if len(inactive) > 0 {
			out += fmt.Sprintf("## Inactive (>%d days)\n\n", inactiveDays)
			for _, u := range inactive {
				logon := "never"
				if u.LastLogon != nil {
					logon = u.LastLogon.Format("2006-01-02")
				}
				out += fmt.Sprintf("- %s (%s) — last logon: %s\n", u.DisplayName, u.SamAccountName, logon)
			}
			out += "\n"
		}
		if len(pwdNoExp) > 0 {
			out += "## Password Never Expires\n\n"
			for _, u := range pwdNoExp {
				out += fmt.Sprintf("- %s (%s) — %s\n", u.DisplayName, u.SamAccountName, u.Department)
			}
		}
		return toolText(out), nil
	})
}
