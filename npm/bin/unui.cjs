#!/usr/bin/env node

const { spawnSync } = require("node:child_process");
const { dirname, join, resolve } = require("node:path");

const { detectNPMInstallation } = require("./installation.cjs");
const platforms = require("../platforms.json");

const platformKey = `${process.platform}-${process.arch}`;
const packageName = platforms[platformKey];

if (!packageName) {
  console.error(
    `unUI does not provide an npm binary for ${process.platform}/${process.arch}.`,
  );
  process.exit(1);
}

let packageManifest;

try {
  packageManifest = require.resolve(`${packageName}/package.json`);
} catch {
  console.error(
    [
      `The optional package ${packageName} is required for this platform but was not installed.`,
      "Reinstall without --omit=optional and verify that your package manager did not remove optional dependencies.",
    ].join("\n"),
  );
  process.exit(1);
}

const binaryName = process.platform === "win32" ? "unui.exe" : "unui";
const binaryPath = join(dirname(packageManifest), "bin", binaryName);
const installation = detectNPMInstallation({
  invokedPath: process.argv[1],
  packageRoot: resolve(__dirname, ".."),
  userAgent: process.env.npm_config_user_agent,
});
const result = spawnSync(binaryPath, process.argv.slice(2), {
  env: {
    ...process.env,
    UNUI_INTERNAL_INSTALL_SOURCE: "npm",
    UNUI_INTERNAL_NPM_CLIENT: installation.manager,
    UNUI_INTERNAL_NPM_GLOBAL: installation.global ? "1" : "0",
    UNUI_INTERNAL_NPM_TEMPORARY: installation.temporary ? "1" : "0",
  },
  stdio: "inherit",
  windowsHide: false,
});

if (result.error) {
  console.error(`Unable to start the unUI binary: ${result.error.message}`);
  process.exit(1);
}

if (result.signal) {
  process.kill(process.pid, result.signal);
}

process.exit(result.status ?? 1);
