# unUI CLI

The unUI CLI turns natural-language interface requests into evidence-backed
design guidance that developers and coding agents can use to build better user
interfaces. Learn more at the official [unUI website](https://unui.cc/).

## Quick start

After installing the CLI, log in once and install the bundled skill for your
coding agent:

```sh
unui auth login
unui update-skill --client codex
```

The login command opens a browser authorization page. After approval,
`unui update-skill` installs the skill included in the CLI source and release.
One device can stay authorized per account; a successful login automatically
signs out the previous device.

Use `$unui` in Codex or `/unui` in Claude Code and Cursor for later interface
tasks. The skill calls `unui ask` and applies the returned Evidence Pack.

## CI and agents

Add `--json` when CI or a coding agent needs machine-readable output:

```sh
unui auth show --json
unui update-skill --client codex --json
unui ask "Build a dense SaaS billing settings page" --json
```

JSON mode writes one versioned document to stdout:

```json
{
  "schemaVersion": "1",
  "ok": true,
  "exitCode": 0,
  "data": {},
  "error": null
}
```

If you are an agent setting up unUI, visit <https://unui.cc/setup.txt> to learn
the complete setup workflow.

## Version

Inspect the CLI version and the source revision used to build it:

```sh
unui version
unui version --json
```

Release binaries report their semantic version, commit, commit date, dirty
state, and Go toolchain version. Development builds use Go's embedded module and
VCS metadata and fall back to `dev` when that information is unavailable.
