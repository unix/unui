#!/usr/bin/env python3
"""Diagnose UNUI MCP business auth state through the public auth status tool."""

from __future__ import annotations

import argparse
import json
import sys
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


DEFAULT_MCP_URL = "http://api.unui.cc/v1/mcp"
DEFAULT_TIMEOUT_SECONDS = 5
AUTH_STATUS_TOOL = "unui_auth_status"
LOGIN_COMMAND = "codex mcp login unui-mcp --scopes style:read"
RELOGIN_COMMAND = (
    "codex mcp logout unui-mcp && codex mcp login unui-mcp --scopes style:read"
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Check UNUI MCP login, membership, block, and throttle status."
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
        help=f"HTTP timeout in seconds. Defaults to {DEFAULT_TIMEOUT_SECONDS}.",
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
    report = diagnose(args.mcp_url, args.timeout)
    print_report(report, args.format)


def diagnose(mcp_url: str, timeout: float) -> dict[str, Any]:
    response = call_auth_status(mcp_url, timeout)
    auth_status, extraction_status = extract_auth_status(response)
    summary = summarize(auth_status, extraction_status)

    return {
        "authStatus": auth_status,
        "mcpUrl": mcp_url,
        "response": response,
        "summary": summary,
    }


def call_auth_status(mcp_url: str, timeout: float) -> dict[str, Any]:
    body = json.dumps(
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tools/call",
            "params": {
                "arguments": {},
                "name": AUTH_STATUS_TOOL,
            },
        }
    ).encode("utf-8")
    request = Request(
        mcp_url,
        data=body,
        headers={
            "Accept": "application/json, text/event-stream",
            "Content-Type": "application/json",
        },
        method="POST",
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
        return parse_event_stream_body(raw_body)


def parse_event_stream_body(raw_body: str) -> Any:
    for line in raw_body.splitlines():
        if not line.startswith("data:"):
            continue
        data = line.removeprefix("data:").strip()
        if not data or data == "[DONE]":
            continue
        try:
            return json.loads(data)
        except json.JSONDecodeError:
            return raw_body

    return raw_body


def extract_auth_status(response: dict[str, Any]) -> tuple[dict[str, Any] | None, str]:
    if response.get("status") != 200:
        return None, "unavailable"

    body = response.get("body")
    if not isinstance(body, dict):
        return None, "invalid_response"

    result = body.get("result")
    if not isinstance(result, dict):
        return None, "invalid_response"

    content = result.get("content")
    if not isinstance(content, list) or not content:
        return None, "empty_content"

    first_item = content[0]
    if not isinstance(first_item, dict):
        return None, "invalid_response"

    text = first_item.get("text")
    if not isinstance(text, str):
        return None, "invalid_response"

    try:
        auth_status = json.loads(text)
    except json.JSONDecodeError:
        return None, "invalid_payload"

    if not isinstance(auth_status, dict):
        return None, "invalid_payload"

    return auth_status, "ok"


def summarize(
    auth_status: dict[str, Any] | None,
    extraction_status: str,
) -> dict[str, Any]:
    if extraction_status == "unavailable":
        return summary(
            "auth_status_unavailable",
            "UNUI MCP auth diagnosis is unavailable. Check MCP health and repair the UNUI MCP service before retrying.",
            False,
        )
    if extraction_status == "empty_content":
        return summary(
            "auth_status_empty",
            "Wait a minute, then run the UNUI MCP auth diagnosis again.",
            False,
        )
    if extraction_status != "ok" or auth_status is None:
        return summary(
            "auth_status_invalid",
            "Repair the UNUI MCP auth status response, then run the diagnosis again.",
            False,
        )

    problems = auth_status.get("problems")
    problem_list = problems if isinstance(problems, list) else []
    recommended_action = auth_status.get("recommendedAction")
    next_action = (
        recommended_action
        if isinstance(recommended_action, str) and recommended_action
        else None
    )

    usage_throttle = auth_status.get("usageThrottle")
    throttle_active = (
        isinstance(usage_throttle, dict) and usage_throttle.get("active") is True
    )
    if throttle_active or "usage_throttled" in problem_list:
        return summary(
            "usage_throttled",
            next_action or "Wait until usageThrottle.throttleUntil.",
            True,
            auth_status,
        )
    if "account_blocked" in problem_list:
        return summary(
            "account_blocked",
            next_action or "Switch accounts or contact unUI support.",
            True,
            auth_status,
        )
    if "membership_inactive" in problem_list:
        return summary(
            "membership_inactive",
            next_action or "Renew or switch to an eligible account, then re-login to UNUI MCP.",
            True,
            auth_status,
            cli_command=RELOGIN_COMMAND,
            suggested_prompt="login unui",
        )
    if "not_logged_in" in problem_list:
        return summary(
            "not_logged_in",
            "Ask Codex to start UNUI login, or run the CLI command manually.",
            True,
            auth_status,
            cli_command=next_action or LOGIN_COMMAND,
            suggested_prompt="login unui",
        )
    if auth_status.get("simulatedMCP") is True:
        return summary(
            "authorized",
            "UNUI MCP auth is ready.",
            True,
            auth_status,
        )

    return summary(
        "auth_status_unknown",
        "Review the UNUI MCP auth status payload before continuing.",
        True,
        auth_status,
    )


def summary(
    status: str,
    next_action: str,
    available: bool,
    auth_status: dict[str, Any] | None = None,
    cli_command: str | None = None,
    suggested_prompt: str | None = None,
) -> dict[str, Any]:
    return {
        "authStatusAvailable": available,
        "cliCommand": cli_command,
        "nextAction": next_action,
        "status": status,
        "suggestedPrompt": suggested_prompt,
        **compact_auth_fields(auth_status),
    }


def compact_auth_fields(auth_status: dict[str, Any] | None) -> dict[str, Any]:
    if not auth_status:
        return {
            "auth": None,
            "membership": None,
            "problems": [],
            "usageThrottle": None,
        }

    return {
        "auth": auth_status.get("auth")
        if isinstance(auth_status.get("auth"), dict)
        else None,
        "membership": auth_status.get("membership")
        if isinstance(auth_status.get("membership"), dict)
        else None,
        "problems": auth_status.get("problems")
        if isinstance(auth_status.get("problems"), list)
        else [],
        "usageThrottle": auth_status.get("usageThrottle")
        if isinstance(auth_status.get("usageThrottle"), dict)
        else None,
    }


def print_report(report: dict[str, Any], output_format: str) -> None:
    if output_format == "json":
        print(json.dumps(compact_report(report), indent=2, sort_keys=True))
        return
    if output_format == "debug-json":
        print(json.dumps(report, indent=2, sort_keys=True))
        return

    summary = report["summary"]
    print(f"UNUI MCP auth: {summary['status']}")
    print(f"Next: {summary['nextAction']}")
    if summary.get("suggestedPrompt"):
        print(f"Suggested prompt: {summary['suggestedPrompt']}")
    if summary.get("cliCommand"):
        print(f"Manual command: {summary['cliCommand']}")


def compact_report(report: dict[str, Any]) -> dict[str, Any]:
    summary = report["summary"]

    return {
        "auth": summary["auth"],
        "membership": summary["membership"],
        "problems": summary["problems"],
        "summary": {
            "authStatusAvailable": summary["authStatusAvailable"],
            "cliCommand": summary["cliCommand"],
            "nextAction": summary["nextAction"],
            "status": summary["status"],
            "suggestedPrompt": summary["suggestedPrompt"],
        },
        "usageThrottle": summary["usageThrottle"],
    }


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
