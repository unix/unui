import { spawnSync } from "node:child_process";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { hasOption, optionValue, readJson, sleep } from "./lib.mjs";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "../..");
const args = process.argv.slice(2);
const outputDirectory = resolve(
  repositoryRoot,
  optionValue(args, "--directory", "dist/npm"),
);
const config = await readJson(join(repositoryRoot, "npm/release.json"));
const release = await readJson(join(outputDirectory, "release-manifest.json"));
const distTag = optionValue(args, "--tag", release.distTag);
const trustedPublishing = hasOption(args, "--trusted-publishing");
const publishRequested = trustedPublishing || hasOption(args, "--publish");

if (!publishRequested) {
  throw new Error(
    "Publishing requires --publish for an interactive release or --trusted-publishing in GitHub Actions",
  );
}

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

if (trustedPublishing) {
  if (process.env.NPM_TOKEN || process.env.NODE_AUTH_TOKEN) {
    throw new Error(
      "Trusted Publishing must not run with NPM_TOKEN or NODE_AUTH_TOKEN",
    );
  }

  if (
    !process.env.ACTIONS_ID_TOKEN_REQUEST_URL ||
    !process.env.ACTIONS_ID_TOKEN_REQUEST_TOKEN
  ) {
    throw new Error(
      "GitHub Actions did not expose an OIDC token request; verify id-token: write",
    );
  }
}

const packageMetadataUrl = (name) =>
  `${release.registry.replace(/\/$/, "")}/${encodeURIComponent(name)}`;

const distTagVersion = (name) => {
  if (trustedPublishing) return null;

  const result = spawnSync(
    "npm",
    ["dist-tag", "ls", name, "--registry", release.registry],
    {
      encoding: "utf8",
    },
  );

  if (result.status !== 0) return null;

  const line = result.stdout
    .split("\n")
    .find((value) => value.startsWith(`${distTag}:`));

  return line?.slice(distTag.length + 1).trim() ?? null;
};

const packageVersionExists = async (name, version) => {
  const response = await fetch(packageMetadataUrl(name), {
    headers: {
      accept: "application/vnd.npm.install-v1+json",
    },
  });

  if (response.status === 404) {
    return distTagVersion(name) === version;
  }

  if (!response.ok) {
    throw new Error(
      `Unable to inspect ${name}: npm registry returned ${response.status}`,
    );
  }

  const metadata = await response.json();
  return Object.hasOwn(metadata.versions ?? {}, version);
};

const waitForPackageVersion = async (name) => {
  for (let attempt = 1; attempt <= 6; attempt += 1) {
    if (await packageVersionExists(name, release.version)) return;
    if (attempt < 6) await sleep(2000);
  }

  console.warn(
    `${name}@${release.version} was accepted but is not publicly visible yet`,
  );
};

const publishPackage = async (entry) => {
  if (await packageVersionExists(entry.name, release.version)) {
    console.log(`Skipping existing ${entry.name}@${release.version}`);
    return;
  }

  const directory = join(outputDirectory, entry.directory);
  const publish = spawnSync(
    "npm",
    [
      "publish",
      directory,
      "--access",
      "public",
      "--tag",
      distTag,
      "--registry",
      release.registry,
      "--ignore-scripts",
    ],
    {
      stdio: "inherit",
    },
  );

  if (publish.error) {
    throw new Error(
      `Unable to run npm publish for ${entry.name}: ${publish.error.message}`,
    );
  }

  if (publish.status !== 0) {
    throw new Error(`npm publish failed for ${entry.name}`);
  }

  await waitForPackageVersion(entry.name);
};

for (const platform of release.platforms) {
  await publishPackage(platform);
}

await publishPackage(release.root);
console.log(
  `Published 7 npm packages for ${release.version} with tag ${distTag}`,
);
