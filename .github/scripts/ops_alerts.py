#!/usr/bin/env python3
"""Format dependency, host, and uptime checks for a GitHub issue.

Exit 0 when there is nothing to alert, 2 when there is, 1 when the check
itself failed. The issue body is written to stdout.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

HIGH = {"high", "critical"}


def one_line(value: object, limit: int = 300) -> str:
    return " ".join(str(value).split())[:limit]


def npm_high_findings(doc: object) -> list[str]:
    if not isinstance(doc, dict):
        return []
    vulns = doc.get("vulnerabilities")
    if not isinstance(vulns, dict):
        return []
    lines: list[str] = []
    for name, info in vulns.items():
        if not isinstance(info, dict):
            continue
        severity = str(info.get("severity", "")).lower()
        if severity not in HIGH:
            continue
        title = ""
        via = info.get("via")
        if isinstance(via, list):
            for item in via:
                if isinstance(item, dict) and item.get("title"):
                    title = str(item["title"])
                    break
                if isinstance(item, str):
                    title = item
                    break
        suffix = f" — {one_line(title, 180)}" if title else ""
        lines.append(f"{severity}: {one_line(name, 80)}{suffix}")
    return lines


def moderate_count(doc: object) -> int:
    if not isinstance(doc, dict):
        return 0
    meta = doc.get("metadata")
    if not isinstance(meta, dict):
        return 0
    vulns = meta.get("vulnerabilities")
    if not isinstance(vulns, dict):
        return 0
    try:
        return int(vulns.get("moderate", 0))
    except (TypeError, ValueError):
        return 0


def _npm_failed(code: int, doc: object) -> bool:
    if code not in (0, 1) or not isinstance(doc, dict):
        return True
    return "error" in doc and "vulnerabilities" not in doc


def govuln_kind(code: int) -> str:
    if code == 0:
        return "clean"
    if code == 3:
        return "findings"
    return "error"


def summarize_audit(govuln_code: int, govuln_log: str, npm_code: int, npm_doc: object) -> tuple[int, str]:
    govuln = govuln_kind(govuln_code)
    findings = npm_high_findings(npm_doc)
    npm_failed = _npm_failed(npm_code, npm_doc)
    if govuln == "error" or npm_failed:
        body = "\n".join(
            [
                "## Dependency CVE audit failed",
                "",
                f"govulncheck exit {govuln_code}. npm audit exit {npm_code}.",
                "",
                "```",
                govuln_log[-4000:],
                "```",
                "",
            ]
        )
        return 1, body
    actionable = govuln == "findings" or bool(findings)
    lines = ["## Dependency CVE audit", ""]
    if findings:
        lines.append("npm high or critical:")
        lines.extend(f"- {item}" for item in findings)
    else:
        lines.append("No high or critical npm findings.")
    lines.append(f"Moderate npm findings (not alerting): {moderate_count(npm_doc)}.")
    lines.append("")
    if govuln == "findings":
        lines.append("govulncheck reported vulnerabilities:")
    else:
        lines.append("govulncheck reported no vulnerabilities.")
    lines.extend(["", "```", govuln_log[-8000:], "```", ""])
    return (2 if actionable else 0), "\n".join(lines)


def host_reasons(doc: object) -> list[str]:
    if not isinstance(doc, dict):
        return ["host status was not a JSON object"]
    reasons: list[str] = []
    count = doc.get("security_upgrade_count") or 0
    if isinstance(count, int) and count > 0:
        names = doc.get("security_upgrades") if isinstance(doc.get("security_upgrades"), list) else []
        shown = ", ".join(one_line(name, 80) for name in names[:40])
        suffix = f": {shown}" if shown else ""
        reasons.append(f"{count} security update(s) pending{suffix}")
    if doc.get("reboot_required") is True:
        reasons.append("reboot required")
    if doc.get("api_local_health") == "down":
        reasons.append("loopback API /healthz is down")
    if doc.get("docker_socket") == "world-accessible":
        reasons.append("Docker socket is world-accessible")
    unexpected = doc.get("unexpected_listeners")
    if isinstance(unexpected, list) and unexpected:
        rendered = ", ".join(one_line(item, 80) for item in unexpected[:30])
        reasons.append(f"unexpected listeners: {rendered}")
    warnings = doc.get("warnings")
    if isinstance(warnings, list):
        for warning in warnings[:8]:
            reasons.append(one_line(warning, 200))
    return reasons


def format_host_body(doc: object, reasons: list[str]) -> str:
    lines = ["## Host watch", ""]
    if reasons:
        lines.append("Actionable:")
        lines.extend(f"- {reason}" for reason in reasons)
    else:
        lines.append("Nothing actionable.")
    lines.append("")
    if isinstance(doc, dict):
        for key in (
            "collected_at",
            "os",
            "kernel",
            "nginx",
            "nginx_package",
            "openssh_package",
            "docker",
            "docker_socket",
            "api_local_health",
            "reboot_required",
            "unattended_upgrades",
            "security_upgrade_count",
            "other_upgrade_count",
        ):
            if key in doc:
                lines.append(f"- {key}: {one_line(doc[key])}")
    lines.append("")
    lines.append(
        "On main, run Host maintenance. `configure` enables unattended security upgrades. "
        "`apply` installs them now. Check reboot only when reboot_required is true."
    )
    return "\n".join(lines) + "\n"


def classify_uptime(root_code: int, health_code: int, health_body: str) -> tuple[str, str]:
    if root_code != 200:
        return "fail", f"GET / returned HTTP {root_code}."
    stripped = health_body.strip()
    if health_code == 200 and stripped == "ok":
        return "ok", "GET / returned 200 and GET /healthz returned ok."
    if health_code == 200 and stripped.startswith("<"):
        return (
            "ok",
            "GET / returned 200. GET /healthz returned HTML, so the live vhost does not proxy /healthz yet.",
        )
    sample = one_line(stripped, 80)
    return "fail", f"GET / returned 200 but GET /healthz returned HTTP {health_code} ({sample})."


def _read_json(path: Path) -> object:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        print(f"could not read {path.name}: {exc}", file=sys.stderr)
        return None


def _cmd_summarize(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(prog="summarize-audit")
    parser.add_argument("--govuln-exit", type=int, required=True)
    parser.add_argument("--govuln-log", type=Path, required=True)
    parser.add_argument("--npm-exit", type=int, required=True)
    parser.add_argument("--npm-audit", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        log = args.govuln_log.read_text(encoding="utf-8", errors="replace")
    except OSError as exc:
        print(f"could not read govuln log: {exc}", file=sys.stderr)
        return 1
    doc = _read_json(args.npm_audit)
    if doc is None:
        code, body = summarize_audit(args.govuln_exit, log, args.npm_exit, {"error": "unreadable"})
        sys.stdout.write(body)
        return 1 if code == 0 else code
    code, body = summarize_audit(args.govuln_exit, log, args.npm_exit, doc)
    sys.stdout.write(body)
    return code


def _cmd_host(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(prog="host-reasons")
    parser.add_argument("status_file", type=Path)
    args = parser.parse_args(argv)
    doc = _read_json(args.status_file)
    if doc is None:
        sys.stdout.write("Host status JSON could not be read.\n")
        return 1
    reasons = host_reasons(doc)
    sys.stdout.write(format_host_body(doc, reasons))
    return 2 if reasons else 0


def _cmd_uptime(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(prog="uptime")
    parser.add_argument("--root-code", type=int, required=True)
    parser.add_argument("--health-code", type=int, required=True)
    parser.add_argument("--health-body-file", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        body = args.health_body_file.read_text(encoding="utf-8", errors="replace")
    except OSError as exc:
        print(f"could not read health body: {exc}", file=sys.stderr)
        return 1
    status, detail = classify_uptime(args.root_code, args.health_code, body[:4000])
    sys.stdout.write(detail + "\n")
    return 0 if status == "ok" else 2


def main(argv: list[str] | None = None) -> int:
    args = list(sys.argv[1:] if argv is None else argv)
    if not args or args[0] not in {"summarize-audit", "host-reasons", "uptime"}:
        print("usage: ops_alerts.py summarize-audit|host-reasons|uptime ...", file=sys.stderr)
        return 1
    command, rest = args[0], args[1:]
    if command == "summarize-audit":
        return _cmd_summarize(rest)
    if command == "host-reasons":
        return _cmd_host(rest)
    if command == "uptime":
        return _cmd_uptime(rest)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
