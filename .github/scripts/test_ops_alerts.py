#!/usr/bin/env python3
"""Tests for GitHub issue bodies produced by the ops workflows."""

from __future__ import annotations

import contextlib
import io
import json
import tempfile
import unittest
from pathlib import Path

import ops_alerts


class AuditTests(unittest.TestCase):
    def test_high_npm_and_clean_govuln_is_actionable(self) -> None:
        code, body = ops_alerts.summarize_audit(
            0,
            "No vulnerabilities found.",
            1,
            {
                "vulnerabilities": {
                    "marked": {"severity": "high", "via": [{"title": "bad parse <x>"}]}
                },
                "metadata": {"vulnerabilities": {"moderate": 2, "high": 1}},
            },
        )
        self.assertEqual(code, 2)
        self.assertIn("high: marked", body)
        self.assertIn("Moderate npm findings (not alerting): 2.", body)

    def test_moderate_only_is_quiet(self) -> None:
        code, _body = ops_alerts.summarize_audit(
            0,
            "No vulnerabilities found.",
            1,
            {
                "vulnerabilities": {"vite": {"severity": "moderate", "via": ["vite"]}},
                "metadata": {"vulnerabilities": {"moderate": 1}},
            },
        )
        self.assertEqual(code, 0)

    def test_tool_failure(self) -> None:
        code, body = ops_alerts.summarize_audit(1, "boom", 1, {"error": "nope"})
        self.assertEqual(code, 1)
        self.assertIn("failed", body)


class HostTests(unittest.TestCase):
    def test_pending_security_update(self) -> None:
        reasons = ops_alerts.host_reasons(
            {
                "security_upgrade_count": 1,
                "security_upgrades": ["nginx"],
                "api_local_health": "ok",
                "reboot_required": False,
                "warnings": [],
            }
        )
        self.assertEqual(reasons, ["1 security update(s) pending: nginx"])

    def test_clear_host_has_no_reasons(self) -> None:
        self.assertEqual(
            ops_alerts.host_reasons(
                {
                    "security_upgrade_count": 0,
                    "api_local_health": "ok",
                    "reboot_required": False,
                    "docker_socket": "absent",
                    "unexpected_listeners": [],
                    "warnings": [],
                }
            ),
            [],
        )

    def test_cli_exit_codes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "status.json"
            path.write_text(json.dumps({"api_local_health": "down", "warnings": []}), encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(ops_alerts.main(["host-reasons", str(path)]), 2)
            path.write_text("{", encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(ops_alerts.main(["host-reasons", str(path)]), 1)


class UptimeTests(unittest.TestCase):
    def test_html_healthz_is_not_an_outage(self) -> None:
        status, detail = ops_alerts.classify_uptime(200, 200, "<!DOCTYPE html>")
        self.assertEqual(status, "ok")
        self.assertIn("does not proxy", detail)

    def test_plain_ok(self) -> None:
        status, _detail = ops_alerts.classify_uptime(200, 200, "ok")
        self.assertEqual(status, "ok")

    def test_bad_gateway_fails(self) -> None:
        status, detail = ops_alerts.classify_uptime(200, 502, "<html>bad gateway")
        self.assertEqual(status, "fail")
        self.assertIn("502", detail)

    def test_root_down_fails(self) -> None:
        status, _detail = ops_alerts.classify_uptime(503, 200, "ok")
        self.assertEqual(status, "fail")


if __name__ == "__main__":
    unittest.main()
