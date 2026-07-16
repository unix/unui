import { spawnSync } from "node:child_process";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { hasOption, packageNames, readJson, sleep } from "./lib.mjs";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "../..");
const config = await readJson(join(repositoryRoot, "npm/release.json"));
const args = process.argv.slice(2);
const apply = hasOption(args, "--apply");
const confirmedOIDC = hasOption(args, "--confirmed-oidc");

if (apply && !confirmedOIDC) {
  throw new Error(
    "Run a successful Trusted Publishing release before applying this change, then pass --confirmed-oidc",
  );
}

if (apply && !config.confirmations.packageNames) {
  throw new Error(
    "Confirm the npm package names in npm/release.json before restricting publishing access",
  );
}

const commandFor = (name) => [
  "npm",
  "access",
  "set",
  "mfa=publish",
  name,
  "--registry",
  config.registry,
];

for (const name of packageNames(config)) {
  const command = commandFor(name);

  if (!apply) {
    console.log(command.join(" "));
    continue;
  }

  const result = spawnSync(command[0], command.slice(1), {
    stdio: "inherit",
  });

  if (result.status !== 0) {
    throw new Error(`Unable to restrict token publishing for ${name}`);
  }

  await sleep(2000);
}

if (!apply) {
  console.log(
    "\nApply only after a successful OIDC release: rerun with --apply --confirmed-oidc.",
  );
}
