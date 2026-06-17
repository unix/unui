#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import tempfile
from pathlib import Path
from typing import Any


def normalized_path(value: str) -> str:
    return os.path.normcase(os.path.realpath(os.path.expanduser(value)))


def is_target_path(value: Any, targets: set[str]) -> bool:
    return isinstance(value, str) and normalized_path(value) in targets


def remove_paths_from_list(state: dict[str, Any], key: str, targets: set[str]) -> int:
    values = state.get(key)
    if not isinstance(values, list):
        return 0

    kept_values = [value for value in values if not is_target_path(value, targets)]
    removed_count = len(values) - len(kept_values)
    if removed_count:
        state[key] = kept_values

    return removed_count


def remove_paths_from_sidebar_groups(state: dict[str, Any], targets: set[str]) -> int:
    persisted_atom_state = state.get("electron-persisted-atom-state")
    if not isinstance(persisted_atom_state, dict):
        return 0

    sidebar_groups = persisted_atom_state.get("sidebar-collapsed-groups")
    if not isinstance(sidebar_groups, dict):
        return 0

    removed_count = 0
    for key in list(sidebar_groups):
        if is_target_path(key, targets):
            del sidebar_groups[key]
            removed_count += 1

    return removed_count


def write_json_atomic(path: Path, value: dict[str, Any]) -> None:
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    encoded = f"{encoded}\n"
    with tempfile.NamedTemporaryFile(
        "w",
        encoding="utf-8",
        dir=path.parent,
        delete=False,
    ) as tmp_file:
        tmp_file.write(encoded)
        tmp_path = Path(tmp_file.name)

    tmp_path.replace(path)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Remove exact workspace roots from Codex Desktop global state.",
    )
    parser.add_argument(
        "--state",
        default="~/.codex/.codex-global-state.json",
        help="Path to Codex Desktop global state JSON.",
    )
    parser.add_argument(
        "--path",
        action="append",
        required=True,
        help="Workspace path to remove. May be passed multiple times.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    state_path = Path(args.state).expanduser()
    targets = {normalized_path(path) for path in args.path}

    if not state_path.exists():
        print(f"Codex global state not found: {state_path}")
        return 0

    with state_path.open("r", encoding="utf-8") as state_file:
        state = json.load(state_file)

    if not isinstance(state, dict):
        raise TypeError(f"Expected a JSON object in {state_path}")

    removed_count = 0
    removed_count += remove_paths_from_list(
        state,
        "electron-saved-workspace-roots",
        targets,
    )
    removed_count += remove_paths_from_list(state, "project-order", targets)
    removed_count += remove_paths_from_list(state, "active-workspace-roots", targets)
    removed_count += remove_paths_from_sidebar_groups(state, targets)

    if removed_count:
        write_json_atomic(state_path, state)

    print(f"Removed {removed_count} Codex Desktop workspace state entries.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
