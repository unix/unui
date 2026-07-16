import { readFile, writeFile } from "node:fs/promises";

export const supportedPlatforms = [
  {
    goos: "darwin",
    goarch: "amd64",
    os: "darwin",
    cpu: "x64",
    binary: "unui",
    archiveExtension: "tar.gz",
  },
  {
    goos: "darwin",
    goarch: "arm64",
    os: "darwin",
    cpu: "arm64",
    binary: "unui",
    archiveExtension: "tar.gz",
  },
  {
    goos: "linux",
    goarch: "amd64",
    os: "linux",
    cpu: "x64",
    binary: "unui",
    archiveExtension: "tar.gz",
  },
  {
    goos: "linux",
    goarch: "arm64",
    os: "linux",
    cpu: "arm64",
    binary: "unui",
    archiveExtension: "tar.gz",
  },
  {
    goos: "windows",
    goarch: "amd64",
    os: "win32",
    cpu: "x64",
    binary: "unui.exe",
    archiveExtension: "zip",
  },
  {
    goos: "windows",
    goarch: "arm64",
    os: "win32",
    cpu: "arm64",
    binary: "unui.exe",
    archiveExtension: "zip",
  },
];

const semverPattern =
  /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

export const normalizeVersion = (value) => {
  const version = value.startsWith("v") ? value.slice(1) : value;

  if (!semverPattern.test(version)) {
    throw new Error(
      `Expected a semantic version or v-prefixed tag, received ${value}`,
    );
  }

  return version;
};

export const defaultDistTag = (version) =>
  version.includes("-") ? "next" : "latest";

export const platformKey = (platform) => `${platform.os}-${platform.cpu}`;

export const platformPackageName = (config, platform) =>
  `${config.platformPackagePrefix}-${platformKey(platform)}`;

export const packageNames = (config) => [
  ...supportedPlatforms.map((platform) =>
    platformPackageName(config, platform),
  ),
  config.rootPackage,
];

export const repositoryFields = (config) => ({
  repository: {
    type: "git",
    url: `git+${config.repository}`,
  },
  homepage: config.homepage,
  bugs: {
    url: config.bugs,
  },
});

export const readJson = async (path) =>
  JSON.parse(await readFile(path, "utf8"));

export const writeJson = async (path, value) => {
  await writeFile(path, `${JSON.stringify(value, null, 2)}\n`);
};

export const optionValue = (args, name, fallback) => {
  const index = args.indexOf(name);
  if (index === -1) return fallback;

  const value = args[index + 1];
  if (!value || value.startsWith("--")) {
    throw new Error(`${name} requires a value`);
  }

  return value;
};

export const hasOption = (args, name) => args.includes(name);

export const isSetupNodeAuthPlaceholder = (value) =>
  /^X+(?:-X+)+$/.test(String(value ?? "").trim());

export const sleep = (milliseconds) =>
  new Promise((resolve) => setTimeout(resolve, milliseconds));
