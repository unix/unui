import { spawnSync } from "node:child_process";
import { readFile, stat } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import {
  hasOption,
  optionValue,
  readJson,
  supportedPlatforms,
} from "./lib.mjs";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "../..");
const args = process.argv.slice(2);
const outputDirectory = resolve(
  repositoryRoot,
  optionValue(args, "--directory", "dist/npm"),
);
const config = await readJson(join(repositoryRoot, "npm/release.json"));
const release = await readJson(join(outputDirectory, "release-manifest.json"));

if (hasOption(args, "--for-publish")) {
  if (!config.confirmations.packageNames) {
    throw new Error(
      "Confirm the npm package names in npm/release.json before publishing",
    );
  }

  if (!config.confirmations.license) {
    throw new Error(
      "Confirm the npm license in npm/release.json before publishing",
    );
  }
}

const verifyPackage = async (entry, expectedFiles) => {
  const directory = join(outputDirectory, entry.directory);
  const manifest = await readJson(join(directory, "package.json"));

  if (manifest.name !== entry.name) {
    throw new Error(
      `Manifest name mismatch in ${entry.directory}: ${manifest.name}`,
    );
  }

  if (manifest.version !== release.version) {
    throw new Error(
      `Manifest version mismatch in ${entry.directory}: ${manifest.version}`,
    );
  }

  if (manifest.license !== config.license) {
    throw new Error(
      `Manifest license mismatch in ${entry.directory}: ${manifest.license}`,
    );
  }

  if (manifest.scripts) {
    throw new Error(`${entry.name} must not contain npm lifecycle scripts`);
  }

  const result = spawnSync(
    "npm",
    ["pack", "--dry-run", "--json", "--ignore-scripts"],
    {
      cwd: directory,
      encoding: "utf8",
    },
  );

  if (result.error) {
    throw new Error(
      `Unable to run npm pack for ${entry.name}: ${result.error.message}`,
    );
  }

  if (result.status !== 0) {
    throw new Error(`npm pack failed for ${entry.name}: ${result.stderr}`);
  }

  const packOutput = JSON.parse(result.stdout);
  const packResult = Array.isArray(packOutput)
    ? packOutput[0]
    : (packOutput[entry.name] ?? Object.values(packOutput)[0]);
  const files = new Set(packResult.files.map((file) => file.path));

  for (const file of expectedFiles) {
    if (!files.has(file)) {
      throw new Error(`${entry.name} npm tarball does not contain ${file}`);
    }
  }
};

for (const platform of release.platforms) {
  const binaryPath = join(
    outputDirectory,
    platform.directory,
    "bin",
    platform.binary,
  );
  const binary = await stat(binaryPath);

  if ((binary.mode & 0o111) === 0) {
    throw new Error(`${binaryPath} is not executable`);
  }

  await verifyPackage(platform, ["LICENSE", `bin/${platform.binary}`]);
}

await verifyPackage(release.root, [
  "LICENSE",
  "bin/installation.cjs",
  "bin/unui.cjs",
  "platforms.json",
]);

const rootManifest = await readJson(
  join(outputDirectory, release.root.directory, "package.json"),
);
const optionalNames = Object.keys(
  rootManifest.optionalDependencies ?? {},
).sort();
const expectedNames = release.platforms.map((platform) => platform.name).sort();

if (JSON.stringify(optionalNames) !== JSON.stringify(expectedNames)) {
  throw new Error(
    "Root package optionalDependencies do not match platform packages",
  );
}

for (const platform of release.platforms) {
  if (rootManifest.optionalDependencies[platform.name] !== release.version) {
    throw new Error(
      `${platform.name} must use exact version ${release.version}`,
    );
  }
}

for (const platform of supportedPlatforms) {
  const packageName = release.platforms.find(
    (entry) => entry.os === platform.os && entry.cpu === platform.cpu,
  );

  if (!packageName) {
    throw new Error(`Missing package for ${platform.os}/${platform.cpu}`);
  }
}

const wrapper = await readFile(
  join(outputDirectory, release.root.directory, "bin/unui.cjs"),
  "utf8",
);

if (wrapper.includes("postinstall")) {
  throw new Error("Root wrapper must not rely on postinstall");
}

console.log(`Verified ${release.platforms.length + 1} npm packages`);
