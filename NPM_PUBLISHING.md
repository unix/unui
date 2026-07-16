# npm publishing

The npm distribution contains one public entry package and six scoped platform
packages:

```text
@unix/unui
@unix/unui-darwin-x64
@unix/unui-darwin-arm64
@unix/unui-linux-x64
@unix/unui-linux-arm64
@unix/unui-win32-x64
@unix/unui-win32-arm64
```

None of the packages use install lifecycle scripts, Git dependencies, or remote
tarball dependencies.

## Package layout

The entry package exposes the `unui` command and selects a platform package
through exact-version `optionalDependencies`. Platform packages contain the
same binaries as the corresponding GitHub Release archives.

All names and publication metadata are configured in `npm/release.json`.

## Local verification

Build a GoReleaser snapshot first:

```sh
task release-snapshot
```

Create and inspect npm packages without publishing:

```sh
task npm-package-check VERSION=0.2.0-rc.1
```

Generated files are written to `dist/npm`. The packaging script verifies every
release archive against `checksums.txt` before extracting its binary.

## Initial v0.1.0 publication

Trusted Publishing can only be configured after each package exists. The
initial v0.1.0 packages are published from a local interactive npm session with
2FA.

Update npm and sign in:

```sh
npm install --global npm@12.0.1
npm login
npm whoami
```

Download the existing GitHub Release and build the npm packages:

```sh
mkdir -p dist/release-v0.1.0
gh release download v0.1.0 \
  --repo unix/unui \
  --dir dist/release-v0.1.0 \
  --clobber
node npm/scripts/build-packages.mjs \
  --version v0.1.0 \
  --assets dist/release-v0.1.0
node npm/scripts/verify-packages.mjs --for-publish
```

Publish the six platform packages first and the entry package last:

```sh
node npm/scripts/publish-packages.mjs \
  --directory dist/npm \
  --publish \
  --tag latest
```

The script skips versions already present in the registry, so it can safely
continue after a partial failure.

## Trusted Publishing

The npm account must have 2FA enabled. Trusted Publishing is prepared for:

```text
Repository: unix/unui
Workflow: release.yml
Environment: npm
Allowed action: npm publish
```

The GitHub repository must contain an environment named `npm`. Restrict its
deployment branch and tag policy to tags matching `v*`. The release workflow
does not use an npm token or an environment secret.

Print the seven `npm trust` commands:

```sh
node npm/scripts/configure-trusted-publishing.mjs
```

After reviewing them, apply the configuration:

```sh
node npm/scripts/configure-trusted-publishing.mjs --apply
```

The first package prompts for 2FA. npm can temporarily remember the successful
verification while the remaining packages are configured.

Verify that all seven packages have the expected Trusted Publisher:

```sh
task npm-trust-verify
```

Add required reviewers to the GitHub environment if releases should require a
second human approval after a version tag is pushed.

## Verify OIDC with a prerelease

Create a prerelease only after the npm release files and workflow are committed
to the release branch:

```sh
git tag -a v0.2.0-rc.2 -m "Release v0.2.0-rc.2"
git push origin v0.2.0-rc.2
```

Confirm that all seven packages exist at `0.2.0-rc.2`, use the `next` dist-tag,
and show npm provenance linked to `unix/unui`.

After the OIDC release succeeds, preview the commands that set every package's
publishing access to:

```text
Require two-factor authentication and disallow tokens
```

```sh
task npm-token-restrictions
```

Apply the restriction with an interactive npm session:

```sh
node npm/scripts/restrict-token-publishing.mjs \
  --apply \
  --confirmed-oidc
```

Do not add `NPM_TOKEN` or `NODE_AUTH_TOKEN` to the GitHub repository. The
workflow rejects those variables and requires the GitHub OIDC request
environment.

Only then create the stable tag:

```sh
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

Stable versions publish with the `latest` dist-tag. Prerelease versions publish
with `next`.

## Recovery

If npm publishing fails after the GitHub Release succeeds, use **Re-run failed
jobs** in GitHub Actions. The npm job downloads the existing GitHub Release
assets, verifies their checksums, skips versions already published, and
continues with the missing packages.
