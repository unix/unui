---
name: unui-mcp-diagnostics
description: Diagnose UNUI MCP setup and service health when MCP tools are unavailable.
---

# UNUI MCP Diagnostics

Use this skill when the user asks whether the UNUI MCP server is installed,
enabled, reachable, or healthy. This skill is for setup and connectivity only.

## Workflow

1. Prefer `unui_auth_status` or the `unui-mcp-auth` skill for user auth,
   membership, block, and throttle state. This diagnostics skill is only for
   setup and connectivity.
2. Run the bundled diagnostic script when available:

```bash
python3 <plugin-root>/scripts/mcp_diagnose_health.py --format json
```

Resolve `<plugin-root>` to the installed or source plugin directory that
contains this skill. If the script cannot be found, run the equivalent checks
manually:

- `codex mcp list --json`
- `codex mcp get unui-mcp --json`
- `GET /.well-known/oauth-protected-resource/v1/mcp`

3. Interpret only connection-layer state:
   - server missing or disabled: install or enable the `unui-mcp` server.
   - expected MCP tools missing from enabled tools: refresh or reinstall the
     plugin, then open a new Codex thread.
   - service unreachable or metadata invalid: start or fix the UNUI API.
   - all checks pass: use `unui_auth_status` or the auth diagnostic fallback to
     classify business auth state.

Do not use this script to decide whether the user is logged in, whether
membership is active, or whether MCP usage is throttled.

## User-facing output

Respond in the user's preferred language. Keep the summary short and include
only the concrete failing layer and next action. Do not paste full JSON, raw
local cache details, or token details.
