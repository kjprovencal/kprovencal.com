#!/usr/bin/env python3
"""Unit tests for the allowlisted host maintenance script."""

from __future__ import annotations

import contextlib
import io
import json
import os
import stat
import unittest

import host_maint


class ParseTests(unittest.TestCase):
    def test_rejects_unknown_command(self) -> None:
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            host_maint.parse_args(["bash", "-c", "true"])

    def test_status_rejects_reboot_flag(self) -> None:
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            host_maint.parse_args(["status", "--reboot"])

    def test_apply_reboot_flag(self) -> None:
        args = host_maint.parse_args(["apply", "--reboot"])
        self.assertEqual(args.command, "apply")
        self.assertTrue(args.reboot)

    def test_main_refuses_when_not_root(self) -> None:
        if os.geteuid() == 0:
            self.skipTest("root would run apt")
        with contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(host_maint.main(["status"]), 1)


class AptTests(unittest.TestCase):
    def test_security_origin_is_split_from_updates(self) -> None:
        text = "\n".join(
            [
                "Inst nginx [1.24.0-2ubuntu7.3] (1.24.0-2ubuntu7.21 Ubuntu:24.04/noble-security [amd64])",
                "Inst bash [5.2] (5.2-2 Ubuntu:24.04/noble-updates [amd64])",
                "Inst not a package (1 local [amd64])",
                "Conf nginx (1.24.0-2ubuntu7.21 Ubuntu:24.04/noble-security [amd64])",
            ]
        )
        security, other = host_maint.classify_apt_simulation(text)
        self.assertEqual(security, ["nginx"])
        self.assertEqual(other, ["bash"])

    def test_package_names_are_capped_in_the_payload(self) -> None:
        names = [f"libpkg{i}" for i in range(50)]
        payload = host_maint.build_payload(
            collected_at="2026-11-22T00:00:00Z",
            os_name="Ubuntu",
            kernel="6.8.0",
            nginx="nginx/1.24.0",
            nginx_package="1.24.0-2ubuntu7.21",
            openssh_package="1:9.6p1",
            docker="not-installed",
            docker_socket="absent",
            unattended_upgrades=True,
            reboot_required=False,
            security_upgrades=names,
            other_upgrade_count=3,
            listeners=["0.0.0.0:22"],
            unexpected_listeners=[],
            api_local_health="ok",
            warnings=[],
        )
        upgrades = payload["security_upgrades"]
        self.assertIsInstance(upgrades, list)
        assert isinstance(upgrades, list)
        self.assertEqual(len(upgrades), host_maint.MAX_PACKAGES)
        self.assertEqual(payload["security_upgrade_count"], 50)
        self.assertNotIn("env", payload)
        self.assertIn("CVE-2026-99288", payload["notes"][0])


class ListenerTests(unittest.TestCase):
    def test_ss_and_unexpected_ports(self) -> None:
        text = "\n".join(
            [
                "LISTEN 0 128 0.0.0.0:22 0.0.0.0:*",
                "LISTEN 0 511 127.0.0.1:8080 0.0.0.0:*",
                "LISTEN 0 511 *:443 *:*",
                "LISTEN 0 128 [::]:22 [::]:*",
                "LISTEN 0 511 0.0.0.0:8080 0.0.0.0:*",
            ]
        )
        parsed = host_maint.parse_listeners(text)
        self.assertEqual(
            parsed,
            [
                ("0.0.0.0", 22),
                ("127.0.0.1", 8080),
                ("*", 443),
                ("::", 22),
                ("0.0.0.0", 8080),
            ],
        )
        self.assertEqual(host_maint.unexpected_listeners(parsed), ["0.0.0.0:8080"])


class SocketTests(unittest.TestCase):
    def test_world_bits(self) -> None:
        self.assertEqual(
            host_maint.socket_state_from_mode(0o660, exists=True, is_link=False),
            "present-not-world-accessible",
        )
        self.assertEqual(
            host_maint.socket_state_from_mode(0o666, exists=True, is_link=False),
            "world-accessible",
        )
        self.assertEqual(
            host_maint.socket_state_from_mode(None, exists=False, is_link=False),
            "absent",
        )
        self.assertEqual(
            host_maint.socket_state_from_mode(stat.S_IFLNK | 0o777, exists=True, is_link=True),
            "unknown",
        )


class HealthUrlTests(unittest.TestCase):
    def test_only_loopback_healthz(self) -> None:
        self.assertTrue(host_maint.allowed_health_url("http://127.0.0.1:8080/healthz"))
        self.assertTrue(host_maint.allowed_health_url("http://localhost:8080/healthz"))
        self.assertFalse(host_maint.allowed_health_url("http://169.254.169.254/latest/meta-data"))
        self.assertFalse(host_maint.allowed_health_url("https://127.0.0.1:8080/healthz"))
        self.assertFalse(host_maint.allowed_health_url("http://127.0.0.1:8080/healthz?x=1"))
        self.assertFalse(host_maint.allowed_health_url("http://user:pass@127.0.0.1/healthz"))


class SudoersTests(unittest.TestCase):
    def test_maint_user_ignores_root_and_junk(self) -> None:
        previous_host = os.environ.get("HOST_MAINT_USER")
        previous_sudo = os.environ.get("SUDO_USER")
        try:
            os.environ["HOST_MAINT_USER"] = "root"
            os.environ["SUDO_USER"] = "github-deploy"
            self.assertEqual(host_maint.maint_user(), "github-deploy")
            os.environ["HOST_MAINT_USER"] = "bad user"
            os.environ["SUDO_USER"] = "ok;rm"
            self.assertEqual(host_maint.maint_user(), "")
        finally:
            if previous_host is None:
                os.environ.pop("HOST_MAINT_USER", None)
            else:
                os.environ["HOST_MAINT_USER"] = previous_host
            if previous_sudo is None:
                os.environ.pop("SUDO_USER", None)
            else:
                os.environ["SUDO_USER"] = previous_sudo

    def test_render_is_limited_to_the_script(self) -> None:
        text = host_maint.render_sudoers("github-deploy")
        self.assertIn("github-deploy ALL=(root) NOPASSWD: ", text)
        self.assertIn("/usr/local/sbin/personal-site-host-maint status", text)
        self.assertNotIn("NOPASSWD: ALL", text)
        with self.assertRaises(ValueError):
            host_maint.render_sudoers("root")
        with self.assertRaises(ValueError):
            host_maint.render_sudoers("github deploy")
        with self.assertRaises(ValueError):
            host_maint.render_sudoers("ok;rm")


class PayloadTests(unittest.TestCase):
    def test_round_trip_json_has_no_secret_keys(self) -> None:
        payload = host_maint.build_payload(
            collected_at="2026-11-22T00:00:00Z",
            os_name="Ubuntu 24.04",
            kernel="<script>",
            nginx="nginx/1.24.0",
            nginx_package="1.24.0-2ubuntu7.21",
            openssh_package="",
            docker="not-installed",
            docker_socket="nope",
            unattended_upgrades=False,
            reboot_required=True,
            security_upgrades=["nginx", "bad name"],
            other_upgrade_count=1,
            listeners=["127.0.0.1:8080"],
            unexpected_listeners=[],
            api_local_health="ok",
            warnings=["unattended-upgrades is not enabled"],
        )
        encoded = json.dumps(payload)
        self.assertNotIn("SESSION_SECRET", encoded)
        self.assertNotIn("BEGIN OPENSSH", encoded)
        self.assertEqual(payload["docker_socket"], "unknown")
        self.assertEqual(payload["security_upgrades"], ["nginx"])
        self.assertEqual(payload["kernel"], "<script>")


if __name__ == "__main__":
    unittest.main()
