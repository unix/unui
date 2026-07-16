import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";

import {
  defaultDistTag,
  normalizeVersion,
  packageNames,
  platformKey,
  platformPackageName,
  supportedPlatforms,
} from "./lib.mjs";

const require = createRequire(import.meta.url);
const { detectNPMInstallation } = require("../bin/installation.cjs");

const config = {
  rootPackage: "@unix/unui",
  platformPackagePrefix: "@unix/unui",
};

test("normalizes release tags", () => {
  assert.equal(normalizeVersion("v1.2.3"), "1.2.3");
  assert.equal(normalizeVersion("1.2.3-rc.1"), "1.2.3-rc.1");
  assert.throws(() => normalizeVersion("release-1.2.3"));
});

test("selects npm distribution tags", () => {
  assert.equal(defaultDistTag("1.2.3"), "latest");
  assert.equal(defaultDistTag("1.2.3-rc.1"), "next");
});

test("maps Go targets to Node platform keys", () => {
  assert.equal(platformKey(supportedPlatforms[0]), "darwin-x64");
  assert.equal(platformKey(supportedPlatforms[5]), "win32-arm64");
});

test("maps platforms to @unix package names", () => {
  assert.equal(
    platformPackageName(config, supportedPlatforms[0]),
    "@unix/unui-darwin-x64",
  );
  assert.equal(
    platformPackageName(config, supportedPlatforms[5]),
    "@unix/unui-win32-arm64",
  );
});

test("lists platform packages before the root package", () => {
  const names = packageNames(config);

  assert.equal(names.length, 7);
  assert.equal(names.at(-1), "@unix/unui");
});

test("detects global npm package managers from launcher paths", () => {
  const cases = [
    {
      expected: { global: true, manager: "npm", temporary: false },
      packageRoot: "/usr/local/lib/node_modules/@unix/unui",
    },
    {
      expected: { global: true, manager: "pnpm", temporary: false },
      packageRoot:
        "/Users/me/Library/pnpm/global/5/.pnpm/@unix+unui@1.0.0/node_modules/@unix/unui",
    },
    {
      expected: { global: true, manager: "yarn", temporary: false },
      packageRoot: "/Users/me/.config/yarn/global/node_modules/@unix/unui",
    },
    {
      expected: { global: true, manager: "bun", temporary: false },
      packageRoot: "/Users/me/.bun/install/global/node_modules/@unix/unui",
    },
  ];

  for (const fixture of cases) {
    assert.deepEqual(
      detectNPMInstallation({
        invokedPath: `${fixture.packageRoot}/bin/unui.cjs`,
        packageRoot: fixture.packageRoot,
      }),
      fixture.expected,
    );
  }
});

test("detects temporary and local npm package runs", () => {
  assert.deepEqual(
    detectNPMInstallation({
      invokedPath: "/Users/me/.npm/_npx/123/node_modules/.bin/unui",
      packageRoot: "/Users/me/.npm/_npx/123/node_modules/@unix/unui",
    }),
    { global: false, manager: "npm", temporary: true },
  );
  assert.deepEqual(
    detectNPMInstallation({
      invokedPath: "/project/node_modules/.bin/unui",
      packageRoot:
        "/project/node_modules/.pnpm/@unix+unui@1.0.0/node_modules/@unix/unui",
    }),
    { global: false, manager: "pnpm", temporary: false },
  );
});
