# ad-mcp

Active Directory MCP server — remote HTTP/SSE transport for AI agents.

Exposes read-only (MVP1) and write (MVP2+) AD operations as MCP tools, allowing AI agents like Claude Code to query and manage Active Directory without direct LDAP access from the user's device.

## Architecture

```
Claude Code (local)
  → ad-mcp [HTTP/SSE, remote]
      → LDAP → Domain Controller
```

ad-mcp runs on a domain-joined Linux server with LDAP access to the DC. The AI client connects via HTTP/SSE (optionally behind Cloudflare Tunnel + Caddy).

## Available Tools (MVP1 — read-only)

**Users**
- `ad.search_user` — Search by name, email, or username
- `ad.get_user` — Full user details by sAMAccountName or DN
- `ad.get_user_groups` — List groups a user belongs to

**Groups**
- `ad.search_group` — Search groups by name or description
- `ad.get_group` — Group details by name or DN
- `ad.list_group_members` — Direct members of a group

**Reports**
- `ad.find_locked_users` — All currently locked-out accounts
- `ad.find_inactive_users` — Accounts with no logon in N days (default: 90)
- `ad.find_disabled_users` — All disabled accounts
- `ad.find_expiring_accounts` — Accounts expiring within N days (default: 30)
- `ad.find_password_never_expires` — Accounts with DONT_EXPIRE_PASSWORD flag
- `ad.generate_account_review_report` — Combined hygiene report

## Running

**Requirements**: Go 1.21+, network access to Domain Controller (LDAP port 389 or LDAPS 636)

```bash
git clone https://github.com/raychao-oao/ad-mcp.git
cd ad-mcp

export AD_MCP_LDAP_URL=ldap://dc.example.com:389
export AD_MCP_LDAP_BIND_DN="CN=svc-admcp,OU=Service,DC=example,DC=com"
export AD_MCP_LDAP_BIND_PASSWORD="..."
export AD_MCP_LDAP_BASE_DN="DC=example,DC=com"
# Optional:
# AD_MCP_ADDR=:8080          (default)
# AD_MCP_POLICY_FILE=/etc/ad-mcp/policy.yaml

go run .
```

## Roadmap

| MVP | Scope |
|-----|-------|
| MVP1 | 12 read-only tools (current) |
| MVP2 | Write tools: unlock_user, enable_user, disable_user, set_account_expiry, reset_password (requires approval token) |
| MVP3 | mcp-policy middleware (role-based access control) |
| MVP4 | Group membership management |

## Related Projects

- [cred-mcp](https://github.com/raychao-oao/cred-mcp) — Credential management MCP (approval token authority)
- [mcp-policy](https://github.com/raychao-oao/mcp-policy) — Policy engine used by ad-mcp
