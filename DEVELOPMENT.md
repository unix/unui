# CLI development

The unUI CLI is a standalone Go application. Run the commands in this document
from the repository root.

## Requirements

- Go 1.25.8 or newer
- [Task](https://taskfile.dev/)
- [GoReleaser](https://goreleaser.com/) 2.17.0 or newer for release checks
  and snapshot builds
- Node.js 22.14.0 or newer for npm release tooling
- npm 12.0.1 or newer for configuring Trusted Publishing

## Testing

Tests use Go's built-in test runner. Task provides the standard project command:

```sh
task test
```

Run the complete local quality check before submitting changes:

```sh
task
```

This formats the Go source with `goimports`, runs Staticcheck, and executes all
Go tests.

## Running locally

Use `go run` to execute the CLI directly from source:

```sh
go run ./cmd/unui --help
```

To test against a local unUI API, pass its URL when logging in:

```sh
go run ./cmd/unui auth login --registry http://127.0.0.1:3001
```

The selected registry is saved for subsequent commands.

## Deployment

The CLI is distributed as a native binary; it is not deployed as a long-running
service. Build a binary for the current operating system and architecture with:

```sh
task build
```

Task uses `go build` and writes the result to `dist/unui` or `dist/unui.exe` on
Windows. Inspect the version and VCS metadata embedded in the binary with:

```sh
task build-info
```

## Version information

The source code does not contain the current release version. `unui version`
uses Go build information with this precedence:

1. Version, commit, and commit date injected by GoReleaser.
2. Module and VCS information embedded automatically by Go 1.24 or newer.
3. The `dev` fallback when neither source is available.

A valid semantic Git tag such as `v0.1.0` becomes the release version. Builds
from later commits use a Go pseudo-version, and builds from a modified worktree
are marked dirty. The CLI never invokes Git at runtime.

The JSON output includes the full version, commit, commit date, dirty state, and
Go toolchain version:

```sh
go run ./cmd/unui version --json
```

## Release preparation

Validate the GoReleaser configuration:

```sh
task release-check
```

Build the complete release matrix locally without uploading or publishing
anything:

```sh
task release-snapshot
```

GoReleaser builds macOS, Linux, and Windows archives, injects deterministic
commit metadata, and generates `checksums.txt`.

## Tag-triggered releases

The release workflow runs only when a tag whose name starts with `v` is pushed
to GitHub. Pushes to `main`, pull request merges, and local tags do not publish
anything.

Create an annotated semantic version tag on the commit to release:

```sh
git tag -a v0.1.0 -m "Release v0.1.0"
```

Review the tag locally, then push that tag when the release should begin:

```sh
git show v0.1.0
git push origin v0.1.0
```

Pushing the tag causes GitHub Actions to run GoReleaser, create the GitHub
Release, upload the archives and checksum file, then publish the matching npm
packages through Trusted Publishing. Do not reuse, move, or overwrite a
published version tag.

See [NPM_PUBLISHING.md](./NPM_PUBLISHING.md) for the one-time npm bootstrap,
Trusted Publishing configuration, package verification, and recovery steps.
