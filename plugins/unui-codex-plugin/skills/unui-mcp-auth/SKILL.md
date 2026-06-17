---
name: unui-mcp-auth
description: Check UNUI MCP auth, membership, block, and throttle status through the public unui_auth_status MCP tool, with a script fallback when the tool is unavailable.
---

# UNUI MCP Auth

Use this skill when the user asks whether UNUI MCP is logged in, why MCP access
is not working, whether membership can authorize Codex, or how to repair the
current UNUI MCP session.

## Primary workflow

1. Call the MCP tool `unui_auth_status`.
2. Parse `result.content[0].text` as JSON.
3. Report a short localized diagnosis from these fields:
   - `auth.account`
   - `auth.loginVerified`
   - `auth.blocked`
   - `membership.level`
   - `membership.expiresAt`
   - `membership.canAuthorizeCodex`
   - `simulatedMCP`
   - `usageThrottle.active`
   - `usageThrottle.throttleUntil`
   - `problems`
   - `recommendedAction`

`unui_auth_status` is a public diagnostic tool. It does not require login,
membership, or normal MCP usage quota. It may return no content if its own
small abuse-prevention limit is exceeded; in that case, ask the user to wait a
minute and try again.

## User-facing guidance

Keep the response compact and action-oriented. Do not expose raw local Codex
fields, cache paths, internal IDs, or token details.

Map status to guidance this way:

- `auth.loginVerified` is `false`: say UNUI MCP is not logged in and recommend
  asking Codex to start UNUI login. Also provide
  `codex mcp login unui-mcp --scopes style:read` as a manual CLI alternative.
  Do not label the CLI alternative as the fix command.
- `auth.blocked` is `true` or `problems` contains `account_blocked`: say the
  current account is blocked and the user should switch accounts or contact
  support.
- `membership.canAuthorizeCodex` is `false` or `problems` contains
  `membership_inactive`: say the account does not currently have active Codex
  MCP access. Recommend renewing or switching to an eligible account; after
  access is restored, recommend asking Codex to start UNUI login. Also provide
  `codex mcp logout unui-mcp && codex mcp login unui-mcp --scopes style:read`
  as a manual CLI alternative.
- `usageThrottle.active` is `true`: say MCP usage is temporarily limited and
  include `usageThrottle.throttleUntil`.
- `simulatedMCP` is `true`: say the account, membership, and current MCP access
  check passed.

Only run logout when the user explicitly asks to sign out, reset, repair,
re-login, or switch accounts.

When a script result includes `suggestedPrompt`, present that as the primary
Codex-assisted path. When it includes `cliCommand`, present it as the manual
alternative. Localize surrounding prose for the user, but keep skill and script
content in English.

## Fallback

If `unui_auth_status` is not visible or cannot be called at all, run the
auth diagnostic fallback:

```bash
python3 <plugin-root>/scripts/mcp_diagnose_auth.py --format json
```

Use this fallback only as a wrapper around the public `unui_auth_status` MCP
diagnostic. Do not use local Codex config, local token files, HTTP status
codes, or other side channels to decide login, membership, block, or throttle
state.

If the script reports `auth_status_unavailable`, run the health diagnostic:

```bash
python3 <plugin-root>/scripts/mcp_diagnose_health.py --format json
```

Use the health result only to explain setup or connectivity problems. Do not
use it to decide membership or login validity.
