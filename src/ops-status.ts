import { escapeHtml } from "./html-escape";

export type HostStatus = {
  configured: boolean;
  hint?: string;
  collected_at?: string;
  os?: string;
  kernel?: string;
  nginx?: string;
  nginx_package?: string;
  openssh_package?: string;
  docker?: string;
  docker_socket?: string;
  unattended_upgrades?: boolean;
  reboot_required?: boolean;
  security_upgrade_count?: number;
  security_upgrades?: string[];
  other_upgrade_count?: number;
  listeners?: string[];
  unexpected_listeners?: string[];
  api_local_health?: string;
  warnings?: string[];
  notes?: string[];
};

export type OpsSnapshot = {
  version?: string;
  started_at?: string;
  uptime_seconds?: number;
  requests_total?: number;
  responses_5xx?: number;
  log_level?: string;
  host?: HostStatus;
};

export function formatUptime(seconds: number | undefined): string {
  const total = Number.isFinite(seconds) ? Math.max(0, Math.trunc(seconds ?? 0)) : 0;
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total % 86400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h ${minutes}m`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

function row(label: string, value: string): string {
  return `<div class="admin-ops__item"><dt>${escapeHtml(label)}</dt><dd>${escapeHtml(value)}</dd></div>`;
}

function list(className: string, items: string[]): string {
  if (items.length === 0) return "";
  return `<ul class="${className}">${items
    .map((item) => `<li>${escapeHtml(item)}</li>`)
    .join("")}</ul>`;
}

/** HTML for the admin Ops tab. Every value is escaped. */
export function renderOpsHtml(snapshot: OpsSnapshot): string {
  const host = snapshot.host ?? { configured: false };
  const apiRows = [
    row("Build", snapshot.version || "dev"),
    row("Started", snapshot.started_at || ""),
    row("Uptime", formatUptime(snapshot.uptime_seconds)),
    row("Requests", String(snapshot.requests_total ?? 0)),
    row("5xx since start", String(snapshot.responses_5xx ?? 0)),
    row("Log level", snapshot.log_level || ""),
  ].join("");

  const alerts: string[] = [];
  if ((host.security_upgrade_count ?? 0) > 0) {
    alerts.push(`${host.security_upgrade_count} security update(s) pending.`);
  }
  if (host.reboot_required) {
    alerts.push("A reboot is required before the newest kernel is running.");
  }
  if (host.api_local_health === "down") {
    alerts.push("The API did not answer http://127.0.0.1:8080/healthz.");
  }
  const unexpected = (host.unexpected_listeners ?? []).filter(Boolean);
  if (unexpected.length > 0) {
    alerts.push(`Unexpected public listeners: ${unexpected.join(", ")}`);
  }
  if (host.docker_socket === "world-accessible") {
    alerts.push("The Docker socket is world-accessible.");
  }
  for (const warning of host.warnings ?? []) {
    if (warning) alerts.push(warning);
  }

  const hostBody = host.configured
    ? `${[
        row("Collected", host.collected_at || ""),
        row("OS", host.os || ""),
        row("Kernel", host.kernel || ""),
        row("nginx", host.nginx || ""),
        row("nginx package", host.nginx_package || ""),
        row("OpenSSH", host.openssh_package || ""),
        row("Docker", host.docker || ""),
        row("Docker socket", host.docker_socket || ""),
        row("Unattended upgrades", host.unattended_upgrades ? "Yes" : "No"),
        row("Reboot required", host.reboot_required ? "Yes" : "No"),
        row("Security updates", String(host.security_upgrade_count ?? 0)),
        row("Other updates", String(host.other_upgrade_count ?? 0)),
        row("API on loopback", host.api_local_health || ""),
      ].join("")}`
    : "";

  const hostHtml = host.configured
    ? `<dl class="admin-ops__grid">${hostBody}</dl>${list(
        "admin-ops__packages",
        (host.security_upgrades ?? []).filter(Boolean)
      )}${list("admin-ops__notes", (host.notes ?? []).filter(Boolean))}`
    : `<p class="admin-ops__hint">${escapeHtml(host.hint || "Host status is not configured.")}</p>`;

  return `
    <h2 class="admin-ops__title">API</h2>
    <dl class="admin-ops__grid">${apiRows}</dl>
    <h2 class="admin-ops__title">Host</h2>
    ${list("admin-ops__alerts", alerts)}
    ${hostHtml}
  `.trim();
}
