const normalizePath = (value) =>
  String(value ?? "")
    .replaceAll("\\", "/")
    .toLowerCase();

const managerFromUserAgent = (userAgent) => {
  const value = String(userAgent ?? "").toLowerCase();

  if (value.startsWith("pnpm/")) return "pnpm";
  if (value.startsWith("yarn/")) return "yarn";
  if (value.startsWith("bun/")) return "bun";
  if (value.startsWith("npm/")) return "npm";

  return "unknown";
};

const detectNPMInstallation = ({ invokedPath, packageRoot, userAgent }) => {
  const invoked = normalizePath(invokedPath);
  const root = normalizePath(packageRoot);
  const paths = `${invoked}\n${root}`;

  if (paths.includes("/.npm/_npx/")) {
    return { global: false, manager: "npm", temporary: true };
  }

  if (paths.includes("/.bun/install/cache/")) {
    return { global: false, manager: "bun", temporary: true };
  }

  if (paths.includes("/pnpm/dlx/")) {
    return { global: false, manager: "pnpm", temporary: true };
  }

  if (
    paths.includes("/.bun/install/global/") ||
    invoked.includes("/.bun/bin/")
  ) {
    return { global: true, manager: "bun", temporary: false };
  }

  if (paths.includes("/yarn/global/") || invoked.includes("/.yarn/bin/")) {
    return { global: true, manager: "yarn", temporary: false };
  }

  if (paths.includes("/pnpm/global/")) {
    return { global: true, manager: "pnpm", temporary: false };
  }

  if (
    paths.includes("/lib/node_modules/") ||
    paths.includes("/npm/node_modules/")
  ) {
    return { global: true, manager: "npm", temporary: false };
  }

  const manager = root.includes("/.pnpm/")
    ? "pnpm"
    : managerFromUserAgent(userAgent);

  return { global: false, manager, temporary: false };
};

module.exports = {
  detectNPMInstallation,
};
