#!/usr/bin/env python3
"""Validate repo-local Codex plugin marketplace wiring."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path, PurePosixPath
from typing import Any


ALLOWED_AUTHENTICATION = {"ON_INSTALL", "ON_USE"}
ALLOWED_INSTALLATION = {"AVAILABLE", "INSTALLED_BY_DEFAULT", "NOT_AVAILABLE"}
PLUGIN_NAME_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
TODO_MARKER = "[TODO:"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate a Codex plugin marketplace repository."
    )
    parser.add_argument(
        "--repo-root",
        default=".",
        type=Path,
        help="Repository root. Defaults to the current directory.",
    )
    parser.add_argument(
        "--marketplace",
        default=".agents/plugins/marketplace.json",
        type=Path,
        help="Path to the repo-local marketplace JSON, relative to repo root by default.",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    repo_root = args.repo_root.resolve()
    marketplace_path = args.marketplace
    if not marketplace_path.is_absolute():
        marketplace_path = repo_root / marketplace_path

    errors = validate_repo(repo_root, marketplace_path.resolve())
    if errors:
        print("Plugin repo validation failed:")
        for error in errors:
            print(f"- {error}")
        raise SystemExit(1)

    print(f"Plugin repo validation passed: {display_path(repo_root, marketplace_path)}")


def validate_repo(repo_root: Path, marketplace_path: Path) -> list[str]:
    errors: list[str] = []
    marketplace = load_json_object(marketplace_path, "marketplace", errors)
    if marketplace is None:
        return errors

    reject_unknown_fields(marketplace, {"name", "interface", "plugins"}, "marketplace", errors)
    reject_todo_markers(marketplace, "marketplace", errors)
    require_non_empty_string(marketplace, "name", "marketplace", errors)
    validate_interface(marketplace.get("interface"), errors)

    plugins = marketplace.get("plugins")
    if not isinstance(plugins, list) or not plugins:
        errors.append("marketplace.plugins must be a non-empty array")
        return errors

    seen_plugin_names: set[str] = set()
    for index, entry in enumerate(plugins):
        validate_plugin_entry(repo_root, entry, index, seen_plugin_names, errors)

    return errors


def validate_plugin_entry(
    repo_root: Path,
    entry: Any,
    index: int,
    seen_plugin_names: set[str],
    errors: list[str],
) -> None:
    prefix = f"marketplace.plugins[{index}]"
    if not isinstance(entry, dict):
        errors.append(f"{prefix} must be an object")
        return

    reject_unknown_fields(entry, {"name", "source", "policy", "category"}, prefix, errors)
    reject_todo_markers(entry, prefix, errors)
    plugin_name = require_non_empty_string(entry, "name", prefix, errors)
    if plugin_name is not None:
        if PLUGIN_NAME_RE.fullmatch(plugin_name) is None:
            errors.append(f"{prefix}.name must be lower-case kebab-case")
        if plugin_name in seen_plugin_names:
            errors.append(f"{prefix}.name duplicates plugin `{plugin_name}`")
        seen_plugin_names.add(plugin_name)

    raw_source_path = validate_source(entry.get("source"), prefix, errors)
    plugin_dir = validate_source_path(repo_root, raw_source_path, prefix, errors)
    if plugin_name is not None and plugin_dir is not None:
        if plugin_dir.name != plugin_name:
            errors.append(
                f"{prefix}.source.path directory `{plugin_dir.name}` must match entry name "
                f"`{plugin_name}`"
            )
        validate_plugin_manifest(repo_root, plugin_dir, plugin_name, prefix, errors)

    validate_policy(entry.get("policy"), prefix, errors)
    require_non_empty_string(entry, "category", prefix, errors)


def validate_source(source: Any, prefix: str, errors: list[str]) -> str | None:
    source_prefix = f"{prefix}.source"
    if not isinstance(source, dict):
        errors.append(f"{source_prefix} must be an object")
        return None

    reject_unknown_fields(source, {"source", "path"}, source_prefix, errors)
    source_type = require_non_empty_string(source, "source", source_prefix, errors)
    if source_type is not None and source_type != "local":
        errors.append(f"{source_prefix}.source must be `local`")

    return require_non_empty_string(source, "path", source_prefix, errors)


def validate_source_path(
    repo_root: Path,
    raw_source_path: str | None,
    prefix: str,
    errors: list[str],
) -> Path | None:
    if raw_source_path is None:
        return None

    source_prefix = f"{prefix}.source.path"
    source_path = PurePosixPath(raw_source_path.replace("\\", "/"))
    if (
        source_path.is_absolute()
        or any(part in {"", ".", ".."} for part in source_path.parts)
        or not raw_source_path.startswith("./plugins/")
    ):
        errors.append(f"{source_prefix} must be a relative `./plugins/<plugin-name>` path")
        return None

    plugin_dir = (repo_root / source_path.as_posix()).resolve()
    if not is_relative_to(plugin_dir, repo_root):
        errors.append(f"{source_prefix} must stay inside the repository")
        return None
    if not plugin_dir.is_dir():
        errors.append(f"{source_prefix} points to missing directory `{raw_source_path}`")
        return None

    return plugin_dir


def validate_plugin_manifest(
    repo_root: Path,
    plugin_dir: Path,
    plugin_name: str,
    prefix: str,
    errors: list[str],
) -> None:
    manifest_path = plugin_dir / ".codex-plugin" / "plugin.json"
    manifest = load_json_object(manifest_path, f"{prefix} plugin.json", errors)
    if manifest is None:
        return

    manifest_name = manifest.get("name")
    if manifest_name != plugin_name:
        errors.append(
            f"{display_path(repo_root, manifest_path)} field `name` must match marketplace "
            f"entry `{plugin_name}`"
        )

    check_companion_path(
        manifest=manifest,
        field="skills",
        expected_path=plugin_dir / "skills",
        expected_value="./skills/",
        repo_root=repo_root,
        manifest_path=manifest_path,
        errors=errors,
    )
    check_companion_path(
        manifest=manifest,
        field="mcpServers",
        expected_path=plugin_dir / ".mcp.json",
        expected_value="./.mcp.json",
        repo_root=repo_root,
        manifest_path=manifest_path,
        errors=errors,
    )
    check_companion_path(
        manifest=manifest,
        field="apps",
        expected_path=plugin_dir / ".app.json",
        expected_value="./.app.json",
        repo_root=repo_root,
        manifest_path=manifest_path,
        errors=errors,
    )


def validate_policy(policy: Any, prefix: str, errors: list[str]) -> None:
    policy_prefix = f"{prefix}.policy"
    if not isinstance(policy, dict):
        errors.append(f"{policy_prefix} must be an object")
        return

    reject_unknown_fields(policy, {"installation", "authentication", "products"}, policy_prefix, errors)
    installation = require_non_empty_string(policy, "installation", policy_prefix, errors)
    if installation is not None and installation not in ALLOWED_INSTALLATION:
        allowed = ", ".join(sorted(ALLOWED_INSTALLATION))
        errors.append(f"{policy_prefix}.installation must be one of: {allowed}")

    authentication = require_non_empty_string(policy, "authentication", policy_prefix, errors)
    if authentication is not None and authentication not in ALLOWED_AUTHENTICATION:
        allowed = ", ".join(sorted(ALLOWED_AUTHENTICATION))
        errors.append(f"{policy_prefix}.authentication must be one of: {allowed}")

    products = policy.get("products")
    if products is not None and (
        not isinstance(products, list)
        or not all(isinstance(value, str) and value.strip() for value in products)
    ):
        errors.append(f"{policy_prefix}.products must be an array of non-empty strings")


def validate_interface(interface: Any, errors: list[str]) -> None:
    if interface is None:
        return
    if not isinstance(interface, dict):
        errors.append("marketplace.interface must be an object")
        return

    reject_unknown_fields(interface, {"displayName"}, "marketplace.interface", errors)
    display_name = interface.get("displayName")
    if display_name is not None and (
        not isinstance(display_name, str) or not display_name.strip()
    ):
        errors.append("marketplace.interface.displayName must be a non-empty string")


def check_companion_path(
    *,
    manifest: dict[str, Any],
    field: str,
    expected_path: Path,
    expected_value: str,
    repo_root: Path,
    manifest_path: Path,
    errors: list[str],
) -> None:
    value = manifest.get(field)
    if value is None:
        return
    if value != expected_value:
        errors.append(
            f"{display_path(repo_root, manifest_path)} field `{field}` must be "
            f"`{expected_value}` when present"
        )
        return
    if not expected_path.exists():
        errors.append(
            f"{display_path(repo_root, manifest_path)} field `{field}` points to missing "
            f"`{display_path(repo_root, expected_path)}`"
        )


def load_json_object(path: Path, label: str, errors: list[str]) -> dict[str, Any] | None:
    if not path.is_file():
        errors.append(f"{label} is missing at `{path}`")
        return None
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except OSError:
        errors.append(f"{label} could not be read")
        return None
    except json.JSONDecodeError as error:
        errors.append(f"{label} must be valid JSON: {error}")
        return None
    if not isinstance(payload, dict):
        errors.append(f"{label} must contain a JSON object")
        return None
    return payload


def reject_todo_markers(value: Any, path: str, errors: list[str]) -> None:
    if isinstance(value, str):
        if TODO_MARKER in value:
            errors.append(f"{path} still contains a `[TODO: ...]` placeholder")
        return
    if isinstance(value, list):
        for index, item in enumerate(value):
            reject_todo_markers(item, f"{path}[{index}]", errors)
        return
    if isinstance(value, dict):
        for key, item in value.items():
            reject_todo_markers(item, f"{path}.{key}", errors)


def reject_unknown_fields(
    payload: dict[str, Any],
    allowed_keys: set[str],
    prefix: str,
    errors: list[str],
) -> None:
    for key in sorted(set(payload) - allowed_keys):
        errors.append(f"{prefix}.{key} is not accepted")


def require_non_empty_string(
    payload: dict[str, Any],
    key: str,
    prefix: str,
    errors: list[str],
) -> str | None:
    value = payload.get(key)
    if not isinstance(value, str) or not value.strip():
        errors.append(f"{prefix}.{key} must be a non-empty string")
        return None
    return value


def is_relative_to(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
    except ValueError:
        return False
    return True


def display_path(repo_root: Path, path: Path) -> str:
    try:
        return path.resolve().relative_to(repo_root).as_posix()
    except ValueError:
        return path.as_posix()


if __name__ == "__main__":
    main()
