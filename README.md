# unUI CLI

The unUI CLI turns natural-language interface requests into evidence-backed
design guidance that developers and coding agents can use to build better user
interfaces. Learn more at the official [unUI website](https://unui.cc/).

## Install

On macOS or Linux, download and run the official installer:

```sh
curl -fsSL https://unui.cc/install.sh | sh
```

The installer skips an already current verified installation. Use
`sh -s -- --force` after the pipe to reinstall the same version.

Alternatively, install the cross-platform npm package:

```sh
npm install --global @unix/unui
```

## Quick start

After installing the CLI, log in once and install the bundled skill for your
coding agent:

```sh
unui auth login
unui skill update --client codex
```

The login command opens a browser authorization page. After approval,
`unui skill update` creates or replaces the skill included in the CLI source
and release.
One device can stay authorized per account; a successful login automatically
signs out the previous device.

Use `$unui` in Codex or `/unui` in Claude Code and Cursor for later interface
tasks. The skill calls `unui ask` and applies the returned Evidence Pack.

Print the available bundled skill text without installing it:

```sh
unui skill show
```

## CI and agents

Add `--json` when CI or a coding agent needs machine-readable output:

```sh
unui auth show --json
unui skill update --client codex --json
unui skill show --json
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

## Update and uninstall

Check the latest GitHub Release and show the matching update command when a
newer version is available:

```sh
unui update
unui update --json
```

If the installed version is current, unUI reports that it is already up to
date. When an update is available, npm installations print the matching
package-manager command and script installations print the official installer
command. Run the printed update command after `unui update` exits.

`unui uninstall` prints the package-manager removal command for npm
installations. Verified `install.sh` installations can remove themselves after
confirmation, while Windows installations print a PowerShell removal command.
