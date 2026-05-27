---
name: ad-mcp
description: This skill should be used when the user asks to "find a user", "search Active Directory", "unlock an account", "disable a user", "reset a password", "check group membership", "find locked accounts", "find inactive users", "generate an account review report", or any task involving Active Directory user and group management.
version: 0.1.2
---

# ad-mcp — Active Directory Operations

ad-mcp gives AI agents controlled access to Active Directory. Read operations run immediately. Write operations (unlock, disable, reset password) require explicit user approval before execution.

**The AI never touches AD directly.** Every operation goes through ad-mcp, which enforces OU scope, logs all actions, and gates writes behind an approval token from cred-mcp.

ad-mcp runs as a **remote MCP server** (HTTP/SSE) on a domain-joined machine in the corporate network. cred-mcp continues to run locally on the user's device for biometric approval. The approval token flows from cred-mcp (local) → Claude Code → ad-mcp (remote).

---

## When to Use

Use ad-mcp when the user asks about:
- Finding or looking up a user account
- Checking group memberships
- Finding locked, inactive, or problematic accounts
- Generating account review reports
- Unlocking, disabling, or modifying accounts (with approval)

Do NOT use ad-mcp for:
- Creating new users (requires MVP4 deployment — check with your operator before attempting)
- Moving OUs or restructuring AD
- Managing GPOs, computers, or non-user objects
- Anything outside the configured OU scope

---

## Decision Flow

```
User request
  │
  ├── Read-only? (search, get, list, find, report)
  │     → Execute immediately, show results
  │
  └── Write? (unlock, disable, reset password, set expiry)
        → Confirm intent with user
        → Request approval via cred-mcp
        → Execute only after approval granted
        → Report outcome
```

---

## Read Operations (no approval needed)

### Finding a User

When the user says: "find alice", "look up john smith", "search for user j.chen"

```
1. search_user(query: "alice")
2. If multiple results → show list, ask user to confirm which one
3. If one result → proceed
4. get_user(user_id) → show details (name, email, department, status, last logon)
```

Always search first. Never assume a user_id — always get it from search_user.

### Checking Group Membership

When the user says: "what groups is alice in?", "who's in Domain Admins?", "list members of IT-Helpdesk"

```
# For a user's groups:
get_user_groups(user_id)

# For a group's members:
list_group_members(group: "IT-Helpdesk")
```

### Finding a Group

When the user says: "find the VPN-Users group", "search for IT groups", "what's the exact name of the helpdesk group"

```
1. search_group(query: "vpn") → list matching groups
2. If one result → proceed
3. get_group(group_id) → show details (name, description, type, OU, member count)
```

Always search first when the exact group identifier is unknown — do not guess group names.

### Finding Problematic Accounts

When the user says: "find all locked accounts", "show me inactive users", "who has password never expires?"

```
find_locked_users()              → accounts locked out right now
find_inactive_users(days: 90)    → no logon in N days (default 90)
find_password_never_expires()    → accounts with DONT_EXPIRE_PASSWORD flag
find_disabled_users()            → all disabled accounts
find_expiring_accounts(days: 30) → accounts expiring within N days
```

These are the building blocks for account hygiene reviews.

### Account Review Report

When the user says: "generate an account review", "I need to audit our AD accounts", "prepare the monthly user review"

```
generate_account_review_report()
```

Returns a structured summary: locked accounts, inactive accounts, password-never-expires, accounts expiring soon. Suitable for sending to a manager or pasting into a ticket.

---

## Write Operations (approval required)

All write operations follow the same pattern:

```
1. search_user → confirm identity (never skip this)
   For high-risk operations (disable, enable, reset password, group changes):
   show samAccountName + email + department + manager + OU before proceeding
2. Tell user what you're about to do and why
3. For high-risk and privileged operations: ask the user for a reason if not already stated.
   Example: "Please provide a reason for this action (e.g., 'Employee terminated 2026-05-25')."
   Do not proceed until a reason is provided.
4. Request approval: cred-mcp.request_authorization(
       item_id: <samAccountName>,
       consumer_id: "ad-mcp",
       purpose: "unlock_user" | "set_account_expiry" | "disable_user" |
                "enable_user" | "reset_password" | "add_to_group" | "remove_from_group",
       reason: "<user-provided reason>"   # required for high/privileged ops
   )
   → Returns auth_token (short-lived, signed by cred-mcp identity key)
5. User approves via biometric prompt on their device
6. Execute the operation immediately after receiving auth_token — the token has a short TTL
   (typically 60–300 seconds). Do not pause for additional user input between step 4 and 6.
7. Report outcome
```

**Never execute a write operation without going through step 3.** Even if the user says "just do it" — the approval step is non-negotiable.

### Tool parameter conventions

- **`user_id`**: always pass `samAccountName` (e.g. `alice.chen`), not DN or UPN. Use the value returned by `search_user`.
- **`auth_token`**: opaque string returned by `cred-mcp.request_authorization`. Pass it verbatim — do not log, print, or store it.
- **`expiry_date`**: ISO 8601 date string (`YYYY-MM-DD`, UTC). Pass `null` or omit to clear expiry (account never expires).

### Unlocking a User

When the user says: "unlock bob's account", "alice is locked out", "unblock j.chen"

```
search_user("bob") → confirm identity
  check get_user: is account actually locked? (lockoutTime != 0)
  if already unlocked → tell user, do not request approval
→ "I'll unlock bob.smith (Bob Smith, IT dept). Requesting approval..."
→ request_authorization(item_id: "bob.smith", purpose: "unlock_user")
→ unlock_user(user_id: "bob.smith", auth_token: <token>)
→ "Bob Smith's account has been unlocked."
```

Note: reason is NOT required for medium-risk operations like unlock_user. Do not ask for one unless the user volunteers it.

### Setting Account Expiry

When the user says: "set account expiry for alice", "extend the contractor account", "alice's account expires next week, extend it"

```
search_user("alice") → confirm identity
  check get_user: show current expiry date if set
→ Clarify target date with user if not stated: "What date should the account expire? (YYYY-MM-DD)"
→ "I'll set alice.chen's account expiry to 2026-09-30. Requesting approval..."
→ request_authorization(item_id: "alice.chen", purpose: "set_account_expiry")
→ set_account_expiry(user_id: "alice.chen", expiry_date: "2026-09-30", auth_token: <token>)
→ "Alice Chen's account expiry has been set to 2026-09-30."
```

To **clear** expiry (account never expires):
```
→ "I'll remove the account expiry for alice.chen. Requesting approval..."
→ request_authorization(item_id: "alice.chen", purpose: "set_account_expiry")
→ set_account_expiry(user_id: "alice.chen", expiry_date: null, auth_token: <token>)
→ "Alice Chen's account expiry has been cleared (account will not expire)."
```

### Disabling a User

When the user says: "disable the account for john who left", "deactivate sarah's account", "someone left, need to disable their AD"

```
search_user("john") → confirm identity (extra care — disabling is high impact)
  show: samAccountName + email + department + manager + OU
→ Ask user: "Please provide a reason (e.g., 'Employee terminated 2026-05-25')."
→ "I'll disable john.doe (John Doe, Finance, reports to: Jane Smith). This will prevent login immediately. Requesting approval..."
→ request_authorization(purpose: "disable_user", reason: "<user-provided reason>")
→ disable_user(user_id, auth_token)
→ "John Doe's account has been disabled."
```

For disable: always state the impact ("will prevent login immediately") before requesting approval.

### Re-enabling a User

When the user says: "re-enable alice's account", "alice was disabled by mistake", "restore bob's access"

```
search_user("alice") → confirm identity
→ Ask user: "Please provide a reason (e.g., 'Disabled in error on 2026-05-25')."
→ "I'll re-enable alice.chen (Alice Chen, IT). This will restore login access. Requesting approval..."
→ request_authorization(purpose: "enable_user", reason: "<user-provided reason>")
→ enable_user(user_id, auth_token)
→ "Alice Chen's account has been re-enabled."
```

Note: enable_user restores login access but does not unlock a locked account. If the account is also locked, follow with unlock_user.

### Resetting a Password

When the user says: "reset alice's password", "alice forgot her password", "force password reset for bob"

```
search_user("alice") → confirm identity
→ Ask user: "Please provide a reason (e.g., 'User forgot password and cannot log in')."
→ "I'll reset alice.chen's password and require change at next logon. Requesting approval..."
→ request_authorization(purpose: "reset_password", reason: "<user-provided reason>")
→ reset_password(user_id, auth_token, require_change_at_logon: true)
→ "Password reset. Temporary password delivered via [configured method]."
```

Always set `require_change_at_logon: true` unless the user explicitly says otherwise.

### Group Membership Changes (MVP4)

Group operations are only available in MVP4. Before attempting any group change, confirm MVP4 is deployed.

When the user says: "add alice to VPN-Users", "remove bob from IT-Helpdesk", "give john access to the VPN group"

```
# Adding a user to a group:
search_user("alice") → confirm identity
search_group("VPN") → confirm group identity (never guess the exact group name)
→ Ask user: "Please provide a reason (e.g., 'Alice needs VPN access for remote work')."
→ "I'll add alice.chen to VPN-Users. Requesting approval..."
→ request_authorization(purpose: "add_to_group", reason: "<user-provided reason>")
→ add_user_to_group(user_id, group_id, auth_token)
→ "Alice Chen has been added to VPN-Users."

# Removing a user from a group:
search_user("bob") → confirm identity
search_group("IT-Helpdesk") → confirm group identity
→ Ask user: "Please provide a reason (e.g., 'Bob transferred to Finance team')."
→ "I'll remove bob.smith from IT-Helpdesk. This may affect their access permissions. Requesting approval..."
→ request_authorization(purpose: "remove_from_group", reason: "<user-provided reason>")
→ remove_user_from_group(user_id, group_id, auth_token)
→ "Bob Smith has been removed from IT-Helpdesk."
```

**Group constraints:**
- Only groups in the configured allowlist can be modified. Tier-0 groups (Domain Admins, Schema Admins, Enterprise Admins) are always denied — do not attempt even if the user requests it.
- If the requested group is not in the allowlist, tell the user clearly: "This group cannot be modified via ad-mcp. Please contact your AD administrator directly."
- Always use `search_group` first — never guess group identifiers.

---

## Boundaries and Constraints

**OU scope:** ad-mcp only operates within configured OUs (defined in registry.yaml). If a user is outside the allowed scope, the operation is rejected — tell the user clearly.

**Identity confirmation:** Always search first. If search returns multiple users with similar names, show the list and ask the user to confirm before proceeding. Never guess.

**Write escalation:** If the user asks for a write that seems out of context (e.g., "disable all users in Finance"), pause and ask for clarification. Bulk write operations are not supported in MVP.

**No secrets in context:** Password reset does not return the new password in the conversation. It is delivered via a separate channel (email, SMS) configured in ad-mcp.

**Audit:** Every operation is logged by ad-mcp. The user does not need to log it separately.

---

## Error Handling

| Error | Meaning | What to do |
|-------|---------|-----------|
| `user not found` | No match in AD | Try different search terms, check spelling |
| `outside OU scope` | User is in a restricted OU | Tell user; cannot proceed |
| `approval denied` | User rejected the biometric prompt | Stop; do not retry automatically |
| `approval expired` | Auth token TTL elapsed before execute | Re-run from request_authorization; tell user the approval window expired |
| `invalid approval token` | Token signature invalid or claims mismatch | Do not retry; tell user approval could not be verified |
| `account already unlocked` | Nothing to do | Tell user account is already active; skip approval |
| `expiry date in the past` | expiry_date is earlier than today | Ask user to provide a future date |

---

## Examples

**"Alice can't log in"**
→ search_user("alice") → get_user(user_id) to check full status:
  - account disabled? → enable_user with approval
  - account locked? → unlock_user with approval
  - account expired? → set_account_expiry with approval
  - password expired or must change at logon? → reset_password with approval
  - none of the above? → report what get_user shows (badPwdCount, lockoutTime, last logon) for the operator to investigate

**"Need to audit inactive accounts before the quarterly review"**
→ find_inactive_users(days: 90) → find_password_never_expires() → generate_account_review_report()

**"John left the company last week"**
→ search_user("john") → confirm it's the right John → disable_user with approval

**"Who has access to the VPN-Users group?"**
→ list_group_members(group: "VPN-Users")
