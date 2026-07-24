# Changelog

Notable changes to the unUI CLI are recorded here. Maintain this file when
preparing a release; GoReleaser generates GitHub Release notes from commits
between version tags.

## Unreleased

## v0.3.1 - 2026-07-25

- Map design evidence timeouts and oversized queries to stable, actionable CLI
  errors.
- Keep unUI design evidence queries concise, focused, and within API limits
  while retaining the complete task locally.

## v0.3.0 - 2026-07-20

- Avoid changing persisted configuration, credential, and lock file modes
  during normal CLI access.
- Isolate saved device credentials by configured API environment and migrate
  existing flat credentials when they are next updated.
- Keep configured API endpoints out of routine command output and diagnostics;
  retain explicit access through `config get --registry`.
- Use conventional, default-safe `[y/N]` confirmations for logout and
  uninstall.
- Add safe local CLI link tasks for development.
