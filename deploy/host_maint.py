#!/usr/bin/env python3
"""Fixed host maintenance for the personal-site droplet.

Allowlisted subcommands only: status, configure, apply.
apply accepts --reboot. There is no free-form command.
"""

from __future__ import annotations

import argparse
import grp
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlsplit

STATUS_PATH = Path("/var/lib/personal-site/host-status.json")
SCRIPT_NAME = "personal-site-host-maint"
MAX_PACKAGES = 40
MAX_LISTENERS = 30
PUBLIC_PORTS = {22, 80, 443}
LOOPBACK_HOSTS = {"127.0.0.1", "::1"}
DEFAULT_HEALTH_URL = "http://127.0.0.1:8080/healthz"

PACKAGE_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9+.~:-]{0,120}$")
USER_RE = re.compile(r"^[a-z_][a-z0-9_-]{0,31}$")

KERNEL_NOTE = (
    "A zero security-upgrade count does not mean every kernel CVE is patched. "
    "CVE-2026-99288 (Linux TCP, remote denial of service) had no Ubuntu fix as of 20 Nov 2026. "
    "Unattended-upgrades installs a kernel when Ubuntu publishes one and does not reboot unless apply --reboot is used."
)
NGINX_NOTE = (
    "CVE-2026-123007 and CVE-2026-123633 apply when HTTP/2 is enabled, which certbot usually turns on. "
    "CVE-2026-118899 can hang a worker while it reads a TLS request body. "
    "The fix is the Ubuntu security package (USN-9159-1 / USN-9182-1 or newer), not a version string of 1.30.7."
)

UNATTENDED_CONF = """\
// Managed by personal-site-host-maint. Security pockets only. No automatic reboot.
Unattended-Upgrade::Allowed-Origins {
        "${distro_id}:${distro_codename}-security";
        "${distro_id}ESMApps:${distro_codename}-apps-security";
        "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Automatic-Reboot-WithUsers "false";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
"""

PERIODIC_CONF = """\
// Managed by personal-site-host-maint.
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
"""

SERVER_TOKENS_CONF = "server_tokens off;\n"

SERVICE_UNIT = """\
[Unit]
Description=Write personal-site host status
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/personal-site-host-maint status
Nice=10
NoNewPrivileges=yes

[Install]
WantedBy=multi-user.target
"""

TIMER_UNIT = """\
[Unit]
Description=Daily personal-site host status

[Timer]
OnCalendar=daily
Persistent=true
RandomizedDelaySec=30m
OnBootSec=5min

[Install]
WantedBy=timers.target
"""


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: ANN001
        return None


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog=SCRIPT_NAME)
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("status")
    sub.add_parser("configure")
    apply_parser = sub.add_parser("apply")
    apply_parser.add_argument("--reboot", action="store_true")
    return parser.parse_args(argv)


def classify_apt_simulation(text: str) -> tuple[list[str], list[str]]:
    security: list[str] = []
    other: list[str] = []
    for line in text.splitlines():
        parsed = _parse_inst_line(line)
        if parsed is None:
            continue
        name, origin = parsed
        if "security" in origin.lower():
            security.append(name)
        else:
            other.append(name)
    return security, other


def _parse_inst_line(line: str) -> tuple[str, str] | None:
    stripped = line.strip()
    if not stripped.startswith("Inst "):
        return None
    name, _, tail = stripped[5:].partition(" ")
    tail = tail.lstrip()
    if not name or not PACKAGE_RE.fullmatch(name):
        return None
    # apt prints "Inst name [old] (new origin)" or "Inst name (new origin)".
    if not (tail.startswith("[") or tail.startswith("(")):
        return None
    left = stripped.find("(")
    right = stripped.rfind(")")
    if left < 0 or right <= left:
        return None
    return name, stripped[left + 1 : right]


def parse_listeners(ss_text: str) -> list[tuple[str, int]]:
    found: list[tuple[str, int]] = []
    for line in ss_text.splitlines():
        parts = line.split()
        if len(parts) < 4:
            continue
        addr, port = split_local_address(parts[3])
        if port is None:
            continue
        found.append((addr, port))
    return found


def split_local_address(local: str) -> tuple[str, int | None]:
    host, sep, port_text = local.rpartition(":")
    if sep == "":
        return local, None
    if host.startswith("[") and host.endswith("]"):
        host = host[1:-1]
    try:
        port = int(port_text)
    except ValueError:
        return local, None
    if port < 0 or port > 65535:
        return local, None
    return host, port


def unexpected_listeners(listeners: list[tuple[str, int]]) -> list[str]:
    bad: list[str] = []
    for addr, port in listeners:
        if addr in LOOPBACK_HOSTS:
            continue
        if port in PUBLIC_PORTS:
            continue
        bad.append(f"{addr}:{port}")
        if len(bad) >= MAX_LISTENERS:
            break
    return bad


def socket_state_from_mode(mode: int | None, *, exists: bool, is_link: bool) -> str:
    if not exists:
        return "absent"
    if mode is None:
        return "unknown"
    if is_link:
        return "unknown"
    if mode & 0o007:
        return "world-accessible"
    return "present-not-world-accessible"


def classify_socket(path: Path) -> str:
    try:
        st = path.lstat()
    except FileNotFoundError:
        return "absent"
    except OSError:
        return "unknown"
    if stat.S_ISLNK(st.st_mode):
        try:
            st = path.stat()
        except OSError:
            return "unknown"
        return socket_state_from_mode(st.st_mode, exists=True, is_link=False)
    return socket_state_from_mode(st.st_mode, exists=True, is_link=False)


def allowed_health_url(url: str) -> bool:
    parts = urlsplit(url)
    if parts.scheme != "http":
        return False
    if parts.username or parts.password or parts.query or parts.fragment:
        return False
    if parts.hostname not in {"127.0.0.1", "localhost"}:
        return False
    if parts.path != "/healthz":
        return False
    return True


def render_sudoers(user: str) -> str:
    if user == "root" or not USER_RE.fullmatch(user):
        raise ValueError("refusing sudoers user")
    commands = ", ".join(
        [
            "/usr/local/sbin/personal-site-host-maint status",
            "/usr/local/sbin/personal-site-host-maint configure",
            "/usr/local/sbin/personal-site-host-maint apply",
            "/usr/local/sbin/personal-site-host-maint apply --reboot",
        ]
    )
    return (
        "# Managed by personal-site-host-maint. This script only.\n"
        f"{user} ALL=(root) NOPASSWD: {commands}\n"
    )


def build_payload(
    *,
    collected_at: str,
    os_name: str,
    kernel: str,
    nginx: str,
    nginx_package: str,
    openssh_package: str,
    docker: str,
    docker_socket: str,
    unattended_upgrades: bool,
    reboot_required: bool,
    security_upgrades: list[str],
    other_upgrade_count: int,
    listeners: list[str],
    unexpected_listeners: list[str],
    api_local_health: str,
    warnings: list[str],
) -> dict[str, object]:
    security = [name for name in security_upgrades if PACKAGE_RE.fullmatch(name)]
    socket_state = docker_socket if docker_socket in {
        "absent",
        "unknown",
        "world-accessible",
        "present-not-world-accessible",
    } else "unknown"
    health = api_local_health if api_local_health in {"ok", "down"} else "down"
    return {
        "configured": True,
        "collected_at": _clip(collected_at, 40),
        "os": _clip(os_name, 120),
        "kernel": _clip(kernel, 80),
        "nginx": _clip(nginx, 160),
        "nginx_package": _clip(nginx_package, 80),
        "openssh_package": _clip(openssh_package, 80),
        "docker": _clip(docker, 80) or "not-installed",
        "docker_socket": socket_state,
        "unattended_upgrades": bool(unattended_upgrades),
        "reboot_required": bool(reboot_required),
        "security_upgrade_count": min(len(security), 10000),
        "security_upgrades": security[:MAX_PACKAGES],
        "other_upgrade_count": max(0, min(other_upgrade_count, 10000)),
        "listeners": [_clip(item, 80) for item in listeners[:MAX_LISTENERS]],
        "unexpected_listeners": [_clip(item, 80) for item in unexpected_listeners[:MAX_LISTENERS]],
        "api_local_health": health,
        "warnings": [_clip(item, 200) for item in warnings[:8]],
        "notes": [KERNEL_NOTE, NGINX_NOTE],
    }


def _clip(value: str, limit: int) -> str:
    text = value.strip()
    if len(text) <= limit:
        return text
    return text[:limit]


def run(cmd: list[str], timeout: int = 180) -> subprocess.CompletedProcess[str]:
    env = os.environ.copy()
    env["DEBIAN_FRONTEND"] = "noninteractive"
    env["NEEDRESTART_MODE"] = "a"
    env["APT_LISTCHANGES_FRONTEND"] = "none"
    try:
        return subprocess.run(
            cmd,
            check=False,
            text=True,
            capture_output=True,
            timeout=timeout,
            env=env,
        )
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout if isinstance(exc.stdout, str) else ""
        stderr = exc.stderr if isinstance(exc.stderr, str) else "timed out"
        return subprocess.CompletedProcess(cmd, 124, stdout, stderr or "timed out")


def probe_local_api(url: str) -> str:
    if not allowed_health_url(url):
        return "down"
    opener = urllib.request.build_opener(_NoRedirect)
    try:
        with opener.open(url, timeout=3) as resp:
            body = resp.read(32)
            status = getattr(resp, "status", 200)
            if status == 200 and body == b"ok":
                return "ok"
    except (urllib.error.URLError, TimeoutError, OSError):
        return "down"
    return "down"


def dpkg_version(package: str) -> str:
    if not PACKAGE_RE.fullmatch(package):
        return ""
    result = run(["dpkg-query", "-W", "-f", "${Version}", package], timeout=30)
    if result.returncode != 0:
        return ""
    return result.stdout.strip()


def package_installed(package: str) -> bool:
    if not PACKAGE_RE.fullmatch(package):
        return False
    result = run(["dpkg-query", "-W", "-f", "${Status}", package], timeout=30)
    return result.returncode == 0 and "install ok installed" in result.stdout


def collect_status() -> dict[str, object]:
    warnings: list[str] = []
    updated = run(["apt-get", "update", "-qq"], timeout=300)
    if updated.returncode != 0:
        warnings.append("apt-get update failed")
    simulated = run(["apt-get", "-s", "upgrade"], timeout=180)
    if simulated.returncode != 0:
        warnings.append("apt-get simulation failed")
        security, other = [], []
    else:
        security, other = classify_apt_simulation(simulated.stdout)

    nginx_bin = shutil.which("nginx")
    nginx_text = ""
    if nginx_bin is None:
        warnings.append("nginx binary not found")
    else:
        version = run([nginx_bin, "-v"], timeout=15)
        nginx_text = (version.stderr or version.stdout).strip()

    openssh = dpkg_version("openssh-server")
    if not openssh:
        warnings.append("openssh-server package not installed")

    if not _unattended_enabled():
        warnings.append("unattended-upgrades is not enabled")

    health_url = os.environ.get("API_HEALTH_URL", DEFAULT_HEALTH_URL).strip() or DEFAULT_HEALTH_URL
    if not allowed_health_url(health_url):
        warnings.append("API health URL is not a loopback /healthz address")
        health = "down"
    else:
        health = probe_local_api(health_url)

    ss = run(["ss", "-H", "-ltn"], timeout=15)
    listeners: list[str] = []
    unexpected: list[str] = []
    if ss.returncode != 0:
        warnings.append("ss failed")
    else:
        parsed = parse_listeners(ss.stdout)
        listeners = [f"{addr}:{port}" for addr, port in parsed]
        unexpected = unexpected_listeners(parsed)

    pretty = _read_pretty_name()
    return build_payload(
        collected_at=datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        os_name=pretty,
        kernel=os.uname().release,
        nginx=nginx_text,
        nginx_package=dpkg_version("nginx"),
        openssh_package=openssh,
        docker=_docker_package(),
        docker_socket=classify_socket(Path("/var/run/docker.sock")),
        unattended_upgrades=_unattended_enabled(),
        reboot_required=Path("/var/run/reboot-required").exists(),
        security_upgrades=security,
        other_upgrade_count=len(other),
        listeners=listeners,
        unexpected_listeners=unexpected,
        api_local_health=health,
        warnings=warnings,
    )


def _unattended_enabled() -> bool:
    if not package_installed("unattended-upgrades"):
        return False
    result = run(["systemctl", "is-enabled", "apt-daily-upgrade.timer"], timeout=20)
    return result.returncode == 0 and result.stdout.strip() in {"enabled", "static"}


def _docker_package() -> str:
    for package in ("docker-ce", "docker.io"):
        version = dpkg_version(package)
        if version:
            return version
    return "not-installed"


def _read_pretty_name() -> str:
    path = Path("/etc/os-release")
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return ""
    for line in text.splitlines():
        if line.startswith("PRETTY_NAME="):
            return line.split("=", 1)[1].strip().strip('"')
    return ""


def write_text(path: Path, data: str, mode: int) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(path.name + ".tmp")
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, mode)
    try:
        os.write(fd, data.encode("utf-8"))
    finally:
        os.close(fd)
    os.chmod(tmp, mode)
    os.replace(tmp, path)
    os.chmod(path, mode)


def write_status(payload: dict[str, object], path: Path) -> None:
    write_text(path, json.dumps(payload, indent=2) + "\n", 0o640)
    try:
        gid = grp.getgrnam("www-data").gr_gid
        os.chown(path, 0, gid)
        os.chown(path.parent, 0, gid)
        os.chmod(path.parent, 0o750)
    except KeyError:
        print("group www-data does not exist; status file stays root-owned", file=sys.stderr)
    except PermissionError:
        if os.geteuid() == 0:
            raise


def install_server_tokens() -> str | None:
    if shutil.which("nginx") is None:
        return None
    path = Path("/etc/nginx/conf.d/personal-site-server-tokens.conf")
    write_text(path, SERVER_TOKENS_CONF, 0o644)
    test = run(["nginx", "-t"], timeout=30)
    if test.returncode != 0:
        path.unlink(missing_ok=True)
        run(["nginx", "-t"], timeout=30)
        detail = (test.stderr or test.stdout or "nginx -t failed").strip().splitlines()
        tail = detail[-1] if detail else "nginx -t failed"
        return "nginx -t failed; removed server_tokens drop-in: " + tail[:180]
    reload = run(["systemctl", "reload", "nginx"], timeout=30)
    if reload.returncode != 0:
        return "nginx reload failed after server_tokens change"
    return None


def maint_user() -> str:
    """Deploy user to grant in sudoers. sudo sets SUDO_USER; HOST_MAINT_USER overrides it."""
    for key in ("HOST_MAINT_USER", "SUDO_USER"):
        user = os.environ.get(key, "").strip()
        if user and user != "root" and USER_RE.fullmatch(user):
            return user
    return ""


def install_sudoers(user: str) -> str | None:
    if user == "" or user == "root":
        return None
    if not USER_RE.fullmatch(user):
        return "HOST_MAINT_USER is not a safe username; sudoers not installed"
    if shutil.which("visudo") is None:
        return "visudo is not installed; sudoers not installed"
    text = render_sudoers(user)
    path = Path("/etc/sudoers.d/personal-site-host-maint")
    tmp = path.with_name(path.name + ".tmp")
    write_text(tmp, text, 0o440)
    check = run(["visudo", "-c", "-f", str(tmp)], timeout=20)
    if check.returncode != 0:
        tmp.unlink(missing_ok=True)
        return "visudo rejected the host-maint sudoers drop-in; not installed"
    os.replace(tmp, path)
    os.chmod(path, 0o440)
    return None


def cmd_status() -> int:
    payload = collect_status()
    try:
        write_status(payload, STATUS_PATH)
    except OSError as exc:
        print(f"could not write status file: {exc}", file=sys.stderr)
        return 1
    json.dump(payload, sys.stdout)
    sys.stdout.write("\n")
    return 0


def cmd_configure() -> int:
    updated = run(["apt-get", "update", "-qq"], timeout=300)
    if updated.returncode != 0:
        print("apt-get update failed", file=sys.stderr)
        print((updated.stderr or "")[-1000:], file=sys.stderr)
        return 1
    installed = run(["apt-get", "install", "-y", "unattended-upgrades"], timeout=600)
    if installed.returncode != 0:
        print("could not install unattended-upgrades", file=sys.stderr)
        print((installed.stderr or "")[-1000:], file=sys.stderr)
        return 1
    write_text(Path("/etc/apt/apt.conf.d/52personal-site-unattended"), UNATTENDED_CONF, 0o644)
    write_text(Path("/etc/apt/apt.conf.d/52personal-site-periodic"), PERIODIC_CONF, 0o644)
    for timer in ("apt-daily.timer", "apt-daily-upgrade.timer"):
        enabled = run(["systemctl", "enable", "--now", timer], timeout=30)
        if enabled.returncode != 0:
            print(f"could not enable {timer}", file=sys.stderr)
            print((enabled.stderr or "")[-500:], file=sys.stderr)
            return 1
    write_text(Path("/etc/systemd/system/personal-site-host-status.service"), SERVICE_UNIT, 0o644)
    write_text(Path("/etc/systemd/system/personal-site-host-status.timer"), TIMER_UNIT, 0o644)
    reloaded = run(["systemctl", "daemon-reload"], timeout=30)
    if reloaded.returncode != 0:
        print("systemctl daemon-reload failed", file=sys.stderr)
        return 1
    timer = run(["systemctl", "enable", "--now", "personal-site-host-status.timer"], timeout=30)
    if timer.returncode != 0:
        print("could not enable personal-site-host-status.timer", file=sys.stderr)
        return 1
    token_warning = install_server_tokens()
    if token_warning:
        print(token_warning, file=sys.stderr)
        if token_warning.startswith("nginx reload failed"):
            return 1
    user = maint_user()
    sudo_warning = install_sudoers(user)
    if sudo_warning:
        print(sudo_warning, file=sys.stderr)
    return cmd_status()


def cmd_apply(reboot: bool) -> int:
    if not package_installed("unattended-upgrades"):
        print("unattended-upgrades is not installed; run configure first", file=sys.stderr)
        return 1
    upgraded = run(["unattended-upgrade", "-v"], timeout=1800)
    if upgraded.stdout:
        print(upgraded.stdout[-4000:], file=sys.stderr)
    if upgraded.returncode != 0:
        print((upgraded.stderr or "")[-2000:], file=sys.stderr)
        return 1
    if shutil.which("nginx"):
        test = run(["nginx", "-t"], timeout=30)
        if test.returncode != 0:
            print("nginx -t failed; did not reload", file=sys.stderr)
            return 1
        reload = run(["systemctl", "reload", "nginx"], timeout=30)
        if reload.returncode != 0:
            print("nginx reload failed", file=sys.stderr)
            return 1
    code = cmd_status()
    if code != 0:
        return code
    if not reboot:
        return 0
    scheduled = run(
        ["systemd-run", "--on-active=8s", "systemctl", "reboot"],
        timeout=20,
    )
    if scheduled.returncode != 0:
        print("could not schedule reboot", file=sys.stderr)
        print((scheduled.stderr or "")[-500:], file=sys.stderr)
        return 1
    print("reboot scheduled in about 8 seconds", file=sys.stderr)
    return 0


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if os.geteuid() != 0:
        print(f"{SCRIPT_NAME} must run as root", file=sys.stderr)
        return 1
    if args.command == "status":
        return cmd_status()
    if args.command == "configure":
        return cmd_configure()
    if args.command == "apply":
        return cmd_apply(reboot=bool(args.reboot))
    print("unknown command", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
