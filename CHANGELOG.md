# Changelog

Notable changes to the unUI CLI are recorded here. Maintain this file when
preparing a release; GoReleaser generates GitHub Release notes from commits
between version tags.

## Unreleased

- Avoid changing persisted configuration, credential, and lock file modes
  during normal CLI access.
- Isolate saved device credentials by configured API environment and migrate
  existing flat credentials when they are next updated.
- Keep configured API endpoints out of routine command output and diagnostics;
  retain explicit access through `config get --registry`.
- Use conventional, default-safe `[y/N]` confirmations for logout and
  uninstall.
- Add safe local CLI link tasks for development.
