import { describe, expect, it } from "vitest";
import { formatUptime, renderOpsHtml } from "./ops-status";

describe("formatUptime", () => {
  it("formats minutes, hours, and days", () => {
    expect(formatUptime(0)).toBe("0m");
    expect(formatUptime(125)).toBe("2m");
    expect(formatUptime(3600 + 120)).toBe("1h 2m");
    expect(formatUptime(86400 + 3600)).toBe("1d 1h 0m");
    expect(formatUptime(undefined)).toBe("0m");
  });
});

describe("renderOpsHtml", () => {
  it("escapes host fields and shows security alerts", () => {
    const html = renderOpsHtml({
      version: "abc",
      started_at: "2026-11-22T00:00:00Z",
      uptime_seconds: 90,
      requests_total: 4,
      responses_5xx: 1,
      log_level: "INFO",
      host: {
        configured: true,
        kernel: `<script>alert("x")</script>`,
        security_upgrade_count: 1,
        security_upgrades: ["nginx"],
        reboot_required: true,
        api_local_health: "down",
        unexpected_listeners: ["0.0.0.0:8080"],
        docker_socket: "world-accessible",
        warnings: ["unattended-upgrades is not enabled"],
        notes: ["CVE-2026-99288"],
      },
    });

    expect(html).toContain("abc");
    expect(html).toContain("1m");
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
    expect(html).toContain("1 security update(s) pending.");
    expect(html).toContain("nginx");
    expect(html).toContain("world-accessible");
    expect(html).toContain("0.0.0.0:8080");
  });

  it("shows the not-configured hint without a host file", () => {
    const html = renderOpsHtml({
      host: { configured: false, hint: `Run configure & check <b>` },
    });
    expect(html).toContain("Run configure &amp; check &lt;b&gt;");
    expect(html).not.toContain("<b>");
  });
});
