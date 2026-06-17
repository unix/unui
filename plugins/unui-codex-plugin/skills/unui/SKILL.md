---
name: unui
description: Use when working on the UNUI project, including implementation, review, refactors, test planning, repository-specific maintenance, and UI design or shadcn/Tailwind block work that should use UNUI style evidence.
---

# UNUI

Use this skill for work in the UNUI codebase and for UI work that should be grounded in UNUI style evidence.

## Workflow

1. Inspect the current workspace before changing behavior; do not assume a fixed local repository path.
2. Follow the repository's existing architecture, naming, and UI patterns.
3. Keep edits scoped to the requested feature or fix.
4. Prefer focused verification over broad build commands unless the user asks for a build.
5. Summarize changed files, validation, and any follow-up risks when handing work back.

## Project Notes

- For TypeScript and React changes, follow the user's Codex coding style preferences.
- Avoid starting dev servers unless needed for frontend verification and the user has not asked to skip it.
- Do not rely on maintainer-only local path aliases in public guidance.

## UI Style Evidence

Before designing a page or authoring a new block, fetch a style evidence pack from the bundled MCP server by calling `unui_evidence_pack`.

`unui_evidence_pack` is an MCP tool exposed through the UNUI API MCP endpoint. It is not a standalone REST route, so do not look for a matching controller action or HTTP path with that exact name.

If `unui_evidence_pack` is not visible as a callable tool or a tool call fails,
do not assume the tool is missing and do not immediately fall back to local
files. First call the public `unui_auth_status` MCP tool to diagnose login,
membership, block, and throttle state.

If `unui_auth_status` is not visible or cannot be called at all, use the
bundled auth diagnostic fallback:

```bash
python3 <plugin-root>/scripts/mcp_diagnose_auth.py --format json
```

Resolve `<plugin-root>` to the installed or source plugin directory that contains this skill. If the auth diagnostic reports `auth_status_unavailable`, run the health diagnostic:

```bash
python3 <plugin-root>/scripts/mcp_diagnose_health.py --format json
```

Use the diagnostic and auth results this way:

1. If `unui_auth_status` reports `simulatedMCP: true`, continue to
   `unui_evidence_pack`.
2. If it reports not logged in, blocked account, inactive membership, or active
   usage throttle, stop the requested UNUI workflow and give the repair
   guidance from the auth status result. Prefer Codex-assisted prompts when the
   result includes one, and present CLI commands only as manual alternatives.
3. If the auth diagnostic reports not logged in, blocked account, inactive
   membership, or active usage throttle, stop the requested UNUI workflow and
   give the repair guidance from the diagnostic result. Prefer
   `suggestedPrompt` over `cliCommand` when both are present.
4. If the auth diagnostic cannot reach `unui_auth_status`, use the health
   diagnostic only to explain setup or connectivity state, then stop. Do not
   continue with local fallback unless the user explicitly asks for a fallback.

Use the pack this way:

1. Parse `result.content[0].text` as JSON before reading the pack.
2. Treat returned references as style evidence, not templates.
3. Use `styleSignals` as the primary evidence: inspect `componentPatterns`, `composition`, `layout`, `spacing`, `surface`, `typography`, `colorSystem`, `interaction`, `responsive`, `copyShape`, and `tokenEvidence`.
4. Use `styleBrief` only as a quick orientation summary.
5. Do not request, expect, reconstruct, or copy original HTML.
6. Do not copy a single reference block or reproduce exact element order, original copy, or section assembly.
7. Author a new block that follows the observed density, surface treatment, spacing, typography, color, and interaction patterns.

Call the tool with:

- `task`: the concrete UI task or user request.
- `category`: a known category when obvious, such as `settings`, `dashboard`, `pricing`, `login`, or `hero`.
- `limit`: usually `4` to `6`; use `2` to `3` for narrow tasks and `6` to `8` for page-level or complex work.

Use the MCP server URL supplied by the plugin configuration. Local debug builds may point at localhost; published builds should point at the production MCP endpoint.

If local fallback is explicitly requested after a failure, do not invent style
summaries. Inspect existing nearby blocks or source files directly and preserve
the same evidence-first behavior.
