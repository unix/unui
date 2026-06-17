# Codex Plugin Local Development Paths

This document only records the paths that matter when developing, installing, refreshing, and removing the plugin locally.

Use `<project_root>` for the local repository root and `<project_name>` for the plugin project name.

## Repository Paths

| Path | Purpose | Handling |
| --- | --- | --- |
| `<project_root>` | Plugin marketplace repository root | Run `make install`, `make refresh-cache`, and `make remove` from here |
| `<project_root>/.agents/plugins/marketplace.json` | Local marketplace definition | `make install` reads the marketplace name from this file |
| `<project_root>/plugins/<project_name>` | Plugin source directory | Codex resolves the marketplace entry to this directory |
| `<project_root>/plugins/<project_name>/.codex-plugin/plugin.json` | Plugin manifest | `make refresh-cache` updates the version cachebuster |
| `<project_root>/plugins/<project_name>/.mcp.json` | Plugin MCP server configuration | `make dev` and `make prod` update the MCP URL |
| `<project_root>/scripts/clear_codex_workspace_state.py` | Codex Desktop workspace state cleaner | Called by `make remove` |

## Codex Local State Paths

| Path | Purpose | Handling |
| --- | --- | --- |
| `$HOME/.codex/plugins/cache/<project_name>` | Marketplace-level plugin cache | Removed by `make remove` |
| `$HOME/.codex/plugins/cache/<project_name>/<project_name>` | Plugin-level plugin cache | Removed by `make remove` |
| `$HOME/.codex/.codex-global-state.json` | Codex Desktop workspace/project state | `make remove` removes the plugin repository path exactly |
| `$HOME/.codex/config.toml` | Codex CLI/project trust and marketplace/plugin configuration | Usually not modified by `make remove` |

## Desktop State Fields

`make remove` only removes the plugin repository path from these fields in `$HOME/.codex/.codex-global-state.json`:

- `electron-saved-workspace-roots`
- `project-order`
- `active-workspace-roots`
- `electron-persisted-atom-state.sidebar-collapsed-groups`
