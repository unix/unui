#!/usr/bin/env python3
"""Diagnose UNUI MCP setup and service health without business auth checks."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from dataclasses import dataclass
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen


DEFAULT_MCP_URL = "http://localhost:3001/v1/mcp"
DEFAULT_SERVER_NAME = "unui-mcp"
DEFAULT_TIMEOUT_SECONDS = 5
EXPECTED_TOOLS = ["unui_auth_status", "unui_evidence_pack"]


@dataclass(frozen=True)
class CommandResult:
    ok: bool
    stdout: str
    stderr: str
    returncode: int | None


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Check whether the UNUI MCP service is installed and reachable."
    )
    parser.add_argument(
        "--server",
        default=DEFAULT_SERVER_NAME,
        help=f"Codex MCP server name. Defaults to {DEFAULT_SERVER_NAME}.",
    )
    parser.add_argument(
        "--mcp-url",
        default=DEFAULT_MCP_URL,
        help=f"UNUI MCP URL. Defaults to {DEFAULT_MCP_URL}.",
    )
    parser.add_argument(
        "--timeout",
        default=DEFAULT_TIMEOUT_SECONDS,
        type=float,
        help=f"Command and HTTP timeout in seconds. Defaults to {DEFAULT_TIMEOUT_SECONDS}.",
    )
    parser.add_argument(
        "--format",
        choices=["human", "json", "debug-json"],
        default="human",
        help="Output format. Defaults to human.",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    report = diagnose(args.server, args.mcp_url, args.timeout)
    print_report(report, args.format)


def diagnose(server_name: str, mcp_url: str, timeout: float) -> dict[str, Any]:
    codex_list = load_codex_mcp_list(timeout)
    codex_get = load_codex_mcp_get(server_name, timeout)
    server_entry = find_server(codex_list.get("servers"), server_name)
    server_config = codex_get.get("server")
    enabled_tools = server_enabled_tools(server_config)
    metadata_url = protected_resource_metadata_url(mcp_url)
    metadata = request_json("GET", metadata_url, timeout)
    summary = summarize(
        codex_get=codex_get,
        codex_list=codex_list,
        enabled_tools=enabled_tools,
        mcp_url=mcp_url,
        metadata=metadata,
        server_config=server_config,
        server_entry=server_entry,
        server_name=server_name,
    )

    return {
        "checks": {
            "expectedToolsConfigured": summary["expectedToolsConfigured"],
            "serverConfigured": summary["serverConfigured"],
            "serverEnabled": summary["serverEnabled"],
            "serviceReachable": summary["serviceReachable"],
        },
        "expectedTools": EXPECTED_TOOLS,
        "mcpUrl": mcp_url,
        "metadata": metadata,
        "server": {
            "configured": summary["serverConfigured"],
            "enabled": summary["serverEnabled"],
            "enabledTools": enabled_tools,
            "missingTools": summary["missingTools"],
            "name": server_name,
            "transportUrl": transport_url(server_config),
        },
        "summary": summary,
        "debug": {
            "codexGet": codex_get,
            "codexList": codex_list,
            "metadataUrl": metadata_url,
        },
    }


def load_codex_mcp_list(timeout: float) -> dict[str, Any]:
    result = run_command(["codex", "mcp", "list", "--json"], timeout)
    if not result.ok:
        return {
            "ok": False,
            "error": result.stderr or "Unable to run codex mcp list.",
            "returncode": result.returncode,
        }

    try:
        servers = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        return {
            "ok": False,
            "error": f"Unable to parse codex mcp list JSON: {error}",
        }

    return {
        "ok": True,
        "servers": servers,
    }


def load_codex_mcp_get(server_name: str, timeout: float) -> dict[str, Any]:
    result = run_command(["codex", "mcp", "get", server_name, "--json"], timeout)
    if not result.ok:
        return {
            "ok": False,
            "error": result.stderr or f"Unable to inspect MCP server {server_name}.",
            "returncode": result.returncode,
        }

    try:
        payload = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        return {
            "ok": False,
            "error": f"Unable to parse codex mcp get JSON: {error}",
        }

    return {
        "ok": True,
        "server": payload,
    }


def run_command(command: list[str], timeout: float) -> CommandResult:
    try:
        completed = subprocess.run(
            command,
            capture_output=True,
            check=False,
            text=True,
            timeout=timeout,
        )
    except FileNotFoundError as error:
        return CommandResult(False, "", str(error), None)
    except subprocess.TimeoutExpired:
        return CommandResult(False, "", f"Command timed out: {' '.join(command)}", None)

    return CommandResult(
        completed.returncode == 0,
        completed.stdout,
        completed.stderr,
        completed.returncode,
    )


def find_server(servers: Any, server_name: str) -> dict[str, Any] | None:
    if not isinstance(servers, list):
        return None

    for server in servers:
        if isinstance(server, dict) and server.get("name") == server_name:
            return server

    return None


def server_enabled_tools(server: Any) -> list[str]:
    if not isinstance(server, dict):
        return []
    enabled_tools = server.get("enabled_tools")
    if not isinstance(enabled_tools, list):
        return []

    return [tool for tool in enabled_tools if isinstance(tool, str)]


def transport_url(server: Any) -> str | None:
    if not isinstance(server, dict):
        return None
    transport = server.get("transport")
    if not isinstance(transport, dict):
        return None
    url = transport.get("url")
    return url if isinstance(url, str) else None


def protected_resource_metadata_url(mcp_url: str) -> str:
    parsed = urlparse(mcp_url)
    origin = f"{parsed.scheme}://{parsed.netloc}"
    return f"{origin}/.well-known/oauth-protected-resource{parsed.path}"


def request_json(method: str, url: str, timeout: float) -> dict[str, Any]:
    request = Request(
        url,
        headers={"Accept": "application/json"},
        method=method,
    )

    return open_request(request, timeout)


def open_request(request: Request, timeout: float) -> dict[str, Any]:
    try:
        with urlopen(request, timeout=timeout) as response:
            raw_body = response.read().decode("utf-8")
            return {
                "body": parse_body(raw_body),
                "ok": True,
                "status": response.status,
            }
    except HTTPError as error:
        raw_body = error.read().decode("utf-8")
        return {
            "body": parse_body(raw_body),
            "ok": False,
            "status": error.code,
        }
    except URLError as error:
        return {
            "error": str(error.reason),
            "ok": False,
            "status": None,
        }


def parse_body(raw_body: str) -> Any:
    if not raw_body:
        return None

    try:
        return json.loads(raw_body)
    except json.JSONDecodeError:
        return raw_body


def metadata_resource(metadata: dict[str, Any]) -> str | None:
    body = metadata.get("body")
    if not isinstance(body, dict):
        return None
    resource = body.get("resource")
    return resource if isinstance(resource, str) else None


def summarize(
    codex_get: dict[str, Any],
    codex_list: dict[str, Any],
    enabled_tools: list[str],
    mcp_url: str,
    metadata: dict[str, Any],
    server_config: Any,
    server_entry: dict[str, Any] | None,
    server_name: str,
) -> dict[str, Any]:
    server_configured = server_entry is not None
    server_enabled = bool(server_entry and server_entry.get("enabled", False))
    missing_tools = [tool for tool in EXPECTED_TOOLS if tool not in enabled_tools]
    expected_tools_configured = not missing_tools
    transport = transport_url(server_config)
    service_reachable = metadata.get("status") == 200
    resource = metadata_resource(metadata)
    service_metadata_valid = service_reachable and resource == mcp_url

    if not codex_list.get("ok"):
        return health_summary(
            "codex_cli_unavailable",
            "Make sure the Codex CLI is available, then run this health check again.",
            False,
            expected_tools_configured,
            missing_tools,
            server_configured,
            server_enabled,
            service_reachable,
        )
    if not server_configured:
        return health_summary(
            "server_missing",
            f"Install or enable the MCP server named {server_name}.",
            False,
            expected_tools_configured,
            missing_tools,
            server_configured,
            server_enabled,
            service_reachable,
        )
    if not server_enabled:
        return health_summary(
            "server_disabled",
            f"Enable the MCP server named {server_name}.",
            False,
            expected_tools_configured,
            missing_tools,
            server_configured,
            server_enabled,
            service_reachable,
        )
    if not codex_get.get("ok"):
        return health_summary(
            "server_inspection_failed",
            f"Inspect `codex mcp get {server_name} --json`.",
            False,
            expected_tools_configured,
            missing_tools,
            server_configured,
            server_enabled,
            service_reachable,
        )
    if transport != mcp_url:
        return health_summary(
            "server_url_mismatch",
            "Point the UNUI Codex plugin MCP configuration at the intended MCP service.",
            False,
            expected_tools_configured,
            missing_tools,
            server_configured,
            server_enabled,
            service_reachable,
        )
    if missing_tools:
        return health_summary(
            "tool_config_missing",
            "Refresh or reinstall the UNUI Codex plugin, then open a new Codex thread.",
            False,
            expected_tools_configured,
            missing_tools,
            server_configured,
            server_enabled,
            service_reachable,
        )
    if not service_reachable:
        return health_summary(
            "service_unreachable",
            "Start or repair the UNUI API MCP service, then run this health check again.",
            False,
            expected_tools_configured,
            missing_tools,
            server_configured,
            server_enabled,
            service_reachable,
        )
    if not service_metadata_valid:
        return health_summary(
            "service_metadata_invalid",
            "Make sure the configured MCP URL points at the UNUI MCP service.",
            False,
            expected_tools_configured,
            missing_tools,
            server_configured,
            server_enabled,
            service_reachable,
        )

    return health_summary(
        "ready",
        "Run the UNUI MCP auth diagnosis if access still needs to be classified.",
        True,
        expected_tools_configured,
        missing_tools,
        server_configured,
        server_enabled,
        service_reachable,
    )


def health_summary(
    status: str,
    next_action: str,
    healthy: bool,
    expected_tools_configured: bool,
    missing_tools: list[str],
    server_configured: bool,
    server_enabled: bool,
    service_reachable: bool,
) -> dict[str, Any]:
    return {
        "expectedToolsConfigured": expected_tools_configured,
        "healthy": healthy,
        "missingTools": missing_tools,
        "nextAction": next_action,
        "serverConfigured": server_configured,
        "serverEnabled": server_enabled,
        "serviceReachable": service_reachable,
        "status": status,
    }


def status_label(ok: bool) -> str:
    return "ready" if ok else "needs attention"


def print_report(report: dict[str, Any], output_format: str) -> None:
    if output_format == "json":
        print(json.dumps(compact_report(report), indent=2, sort_keys=True))
        return
    if output_format == "debug-json":
        print(json.dumps(report, indent=2, sort_keys=True))
        return

    summary = report["summary"]
    print(f"UNUI MCP health: {summary['status']}")
    server_ready = summary["serverConfigured"] and summary["serverEnabled"]
    print(f"MCP server: {status_label(server_ready)}")
    print(f"MCP service: {status_label(summary['serviceReachable'])}")
    print(f"Plugin tools: {status_label(summary['expectedToolsConfigured'])}")
    print(f"Next: {summary['nextAction']}")


def compact_report(report: dict[str, Any]) -> dict[str, Any]:
    summary = report["summary"]

    return {
        "checks": report["checks"],
        "missingTools": summary["missingTools"],
        "summary": {
            "healthy": summary["healthy"],
            "nextAction": summary["nextAction"],
            "status": summary["status"],
        },
    }


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
