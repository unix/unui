import { dirname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

import { hasOption, packageNames, readJson, sleep } from "./lib.mjs";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "../..");
const config = await readJson(join(repositoryRoot, "npm/release.json"));
const args = process.argv.slice(2);
const apply = hasOption(args, "--apply");
const verify = hasOption(args, "--verify");
const trusted = config.trustedPublishing;

if (apply && verify) {
  throw new Error("Use either --apply or --verify, not both");
}

if (apply && !config.confirmations.packageNames) {
  throw new Error(
    "Confirm the npm package names in npm/release.json before configuring Trusted Publishing",
  );
}

const commandFor = (name) => [
  "npm",
  "trust",
  "github",
  name,
  "--repo",
  trusted.repository,
  "--file",
  trusted.workflow,
  "--env",
  trusted.environment,
  "--allow-publish",
  "--yes",
];

const verifyTrust = (name) => {
  const result = spawnSync("npm", ["trust", "list", name, "--json"], {
    encoding: "utf8",
  });

  if (result.status !== 0) {
    if (result.stderr) process.stderr.write(result.stderr);
    throw new Error(`Unable to inspect Trusted Publishing for ${name}`);
  }

  const output = JSON.parse(result.stdout);
  const entries = Array.isArray(output) ? output : [output];
  const match = entries.find(
    (entry) =>
      entry.type === "github" &&
      entry.file === trusted.workflow &&
      entry.repository === trusted.repository &&
      entry.environment === trusted.environment &&
      entry.permissions?.includes("createPackage"),
  );

  if (!match) {
    throw new Error(`Trusted Publishing configuration mismatch for ${name}`);
  }

  console.log(`Verified Trusted Publishing for ${name}`);
};

for (const name of packageNames(config)) {
  if (verify) {
    verifyTrust(name);
    continue;
  }

  const command = commandFor(name);

  if (!apply) {
    console.log(command.join(" "));
    continue;
  }

  const result = spawnSync(command[0], command.slice(1), {
    stdio: "inherit",
  });

  if (result.status !== 0) {
    throw new Error(`Unable to configure Trusted Publishing for ${name}`);
  }

  await sleep(2000);
}

if (!apply && !verify) {
  console.log("\nReview the commands, then rerun with --apply.");
}
