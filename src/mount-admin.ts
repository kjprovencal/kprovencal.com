import { publicApiUrl } from "./api-base";
import { escapeHtml } from "./html-escape";
import {
  MD_TABLE_ATTR,
  MD_TABLE_CLASS,
  MD_TAB_LABEL_ATTR,
} from "./marked-tagged-table";

type WeddingRSVP = {
  id: string;
  created_at: string;
  name: string;
  email: string;
  guest_count: number;
  meals: string[];
  notes: string;
};

type ContactRow = {
  id: string;
  created_at: string;
  name: string;
  email: string;
  message: string;
};

type ClientErrorRow = {
  id: string;
  created_at: string;
  kind: string;
  message: string;
  page: string;
  source?: string;
  line?: number;
  column?: number;
  stack?: string;
  user_agent?: string;
};

type AdminTableSpec = {
  table: HTMLTableElement;
  slug: string;
  label: string;
};

type ListHandler = {
  path: string;
  render: (data: unknown, colspan: number) => string;
};

function adminFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(publicApiUrl(path), {
    ...init,
    credentials: "include",
    cache: init?.cache ?? "no-store",
  });
}

function formatWhen(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}

function ensureTbody(table: HTMLTableElement): HTMLTableSectionElement {
  let tbody = table.querySelector("tbody");
  if (!tbody) {
    tbody = table.appendChild(document.createElement("tbody"));
  }
  return tbody;
}

function theadColCount(table: HTMLTableElement): number {
  const n = table.querySelectorAll("thead th").length;
  return n > 0 ? n : 1;
}

function mealListHtml(meals: string[]): string {
  return `<ul class="admin-meals">${meals
    .map((m) => `<li>${escapeHtml(m)}</li>`)
    .join("")}</ul>`;
}

function weddingRowsHtml(rows: WeddingRSVP[], colspan: number): string {
  if (rows.length === 0) {
    return `<tr><td colspan="${colspan}" class="admin-empty">No RSVPs yet.</td></tr>`;
  }
  return rows
    .map(
      (r) =>
        `<tr><td>${escapeHtml(formatWhen(r.created_at))}</td><td>${escapeHtml(
          r.name
        )}</td><td>${escapeHtml(r.email)}</td><td>${
          r.guest_count
        }</td><td>${mealListHtml(
          r.meals ?? []
        )}</td><td class="admin-cell-notes">${escapeHtml(
          r.notes || "—"
        )}</td></tr>`
    )
    .join("");
}

function clientErrorRowsHtml(rows: ClientErrorRow[], colspan: number): string {
  if (rows.length === 0) {
    return `<tr><td colspan="${colspan}" class="admin-empty">No frontend errors yet.</td></tr>`;
  }
  return rows
    .map((r) => {
      const loc =
        r.source && r.line
          ? `${r.source}:${r.line}${r.column ? `:${r.column}` : ""}`
          : r.source ?? "—";
      const detail = [r.message, r.stack ? `\n${r.stack}` : ""].join("");
      return `<tr><td>${escapeHtml(formatWhen(r.created_at))}</td><td>${escapeHtml(
        r.kind
      )}</td><td>${escapeHtml(r.page || "—")}</td><td class="admin-cell-notes">${escapeHtml(
        loc
      )}</td><td class="admin-cell-notes">${escapeHtml(detail)}</td></tr>`;
    })
    .join("");
}

function contactRowsHtml(rows: ContactRow[], colspan: number): string {
  if (rows.length === 0) {
    return `<tr><td colspan="${colspan}" class="admin-empty">No contact messages yet.</td></tr>`;
  }
  return rows
    .map(
      (r) =>
        `<tr><td>${escapeHtml(formatWhen(r.created_at))}</td><td>${escapeHtml(
          r.name
        )}</td><td>${escapeHtml(
          r.email
        )}</td><td class="admin-cell-notes">${escapeHtml(r.message)}</td></tr>`
    )
    .join("");
}

function formatGenericCell(v: unknown): string {
  if (v === null || v === undefined) return "—";
  if (typeof v === "boolean") return v ? "yes" : "no";
  if (typeof v === "number") return escapeHtml(String(v));
  if (typeof v === "string") return escapeHtml(v);
  if (Array.isArray(v)) {
    if (v.every((x) => typeof x === "string")) {
      return mealListHtml(v as string[]);
    }
    return escapeHtml(JSON.stringify(v));
  }
  if (typeof v === "object") {
    return escapeHtml(JSON.stringify(v));
  }
  return escapeHtml(String(v));
}

function genericArrayRowsHtml(data: unknown, colspan: number): string {
  if (!Array.isArray(data)) {
    return `<tr><td colspan="${colspan}" class="admin-empty">Invalid response (expected a JSON array).</td></tr>`;
  }
  if (data.length === 0) {
    return `<tr><td colspan="${colspan}" class="admin-empty">No rows yet.</td></tr>`;
  }
  const first = data[0];
  if (first === null || typeof first !== "object" || Array.isArray(first)) {
    return `<tr><td colspan="${colspan}" class="admin-empty">Unsupported row shape.</td></tr>`;
  }
  const keys = Object.keys(first as object);
  if (keys.length === 0) {
    return `<tr><td colspan="${colspan}" class="admin-empty">Empty objects.</td></tr>`;
  }
  return data
    .map((row) => {
      const o = row as Record<string, unknown>;
      return `<tr>${keys
        .map((k) => `<td class="admin-cell-notes">${formatGenericCell(o[k])}</td>`)
        .join("")}</tr>`;
    })
    .join("");
}

const LIST_REGISTRY: Record<string, ListHandler> = {
  rsvps: {
    path: "/admin/rsvps",
    render: (data, colspan) =>
      weddingRowsHtml(data as WeddingRSVP[], colspan),
  },
  contacts: {
    path: "/admin/contacts",
    render: (data, colspan) =>
      contactRowsHtml(data as ContactRow[], colspan),
  },
  "client-errors": {
    path: "/admin/client-errors",
    render: (data, colspan) =>
      clientErrorRowsHtml(data as ClientErrorRow[], colspan),
  },
};

/** Slug on `@table` line → registry key (API path segment after `/admin/`). */
const SLUG_ALIASES: Record<string, string> = {
  rsvp: "rsvps",
  "wedding-rsvp": "rsvps",
  "wedding-rsvps": "rsvps",
};

function resolveListHandler(slug: string): ListHandler {
  const key = SLUG_ALIASES[slug] ?? slug;
  if (LIST_REGISTRY[key]) {
    return LIST_REGISTRY[key];
  }
  return {
    path: `/admin/${key}`,
    render: (data, colspan) => genericArrayRowsHtml(data, colspan),
  };
}

function slugToDefaultLabel(slug: string): string {
  return slug
    .split(/[-_]/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase())
    .join(" ");
}

const TAGGED_TABLE_SELECTOR = `table.${MD_TABLE_CLASS}[${MD_TABLE_ATTR}]`;

/**
 * Detaches tagged markdown tables from `#content` in **document order**.
 */
function collectAdminTables(): AdminTableSpec[] | null {
  const content = document.getElementById("content");
  if (!content) return null;

  const tables = Array.from(
    content.querySelectorAll<HTMLTableElement>(TAGGED_TABLE_SELECTOR)
  );

  for (const t of tables) {
    t.classList.add("admin-table");
    t.remove();
  }

  if (tables.length === 0) return null;

  return tables.map((table) => {
    const slug = table.getAttribute(MD_TABLE_ATTR)?.trim().toLowerCase() ?? "";
    const labelAttr = table.getAttribute(MD_TAB_LABEL_ATTR)?.trim();
    const label =
      (labelAttr && labelAttr.length > 0 ? labelAttr : null) ??
      (slugToDefaultLabel(slug) || "List");
    return { table, slug, label };
  });
}

const ADMIN_SHELL = `
<div id="login-panel" class="admin-login-panel">
  <form id="login-form" class="admin-login-form" autocomplete="on">
    <h1 class="admin-login-form__title">Admin sign-in</h1>
    <p class="admin-login-form__hint">Use the password configured for this site’s API.</p>
    <label class="admin-login-form__label">Password
      <input type="password" name="password" id="admin-password" class="admin-login-form__input" required autocomplete="current-password" />
    </label>
    <button type="submit" class="admin-login-form__submit">Sign in</button>
    <p id="login-error" class="admin-error" role="alert" hidden></p>
  </form>
</div>
<div id="dashboard" class="admin-dashboard" hidden>
  <div class="admin-dashboard__toolbar">
    <h1 class="admin-dashboard__title">Submissions</h1>
    <button type="button" id="logout-btn" class="admin-dashboard__logout">Sign out</button>
  </div>
  <p id="admin-status" class="admin-status" aria-live="polite"></p>
  <div id="admin-tablist" class="admin-tabs" role="tablist" aria-label="Submission lists"></div>
  <div id="admin-panel-mount"></div>
</div>
`.trim();

type AdminLogsResponse = {
  enabled: boolean;
  path?: string;
  lines?: string[];
  hint?: string;
  since?: string;
  until?: string;
  truncated?: boolean;
  matched?: number;
  log_level?: string;
};

function logRangeISO(value: string): string {
  if (!value) return "";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? "" : d.toISOString();
}

function buildAdminLogsUrl(root: HTMLElement): string {
  const params = new URLSearchParams({ limit: "200" });
  const sinceEl = root.querySelector("#admin-logs-since");
  const untilEl = root.querySelector("#admin-logs-until");
  if (sinceEl instanceof HTMLInputElement && sinceEl.value) {
    const since = logRangeISO(sinceEl.value);
    if (since) params.set("since", since);
  }
  if (untilEl instanceof HTMLInputElement && untilEl.value) {
    const until = logRangeISO(untilEl.value);
    if (until) params.set("until", until);
  }
  return `/admin/logs?${params.toString()}`;
}

async function loadAdminLogs(
  root: HTMLElement,
  statusEl: HTMLElement | null
): Promise<void> {
  const pre = root.querySelector("#admin-logs-view");
  const hint = root.querySelector("#admin-logs-hint");
  if (!(pre instanceof HTMLElement)) return;

  pre.textContent = "Loading…";
  if (hint instanceof HTMLElement) {
    hint.hidden = true;
    hint.textContent = "";
  }

  try {
    const res = await adminFetch(buildAdminLogsUrl(root));
    if (!res.ok) {
      pre.textContent = "Could not load logs.";
      return;
    }
    const data = (await res.json()) as AdminLogsResponse;
    if (!data.enabled) {
      pre.textContent = "";
      if (hint instanceof HTMLElement) {
        hint.textContent =
          data.hint ??
          "Server logging to a file is not configured (set LOG_PATH on the API).";
        hint.hidden = false;
      }
      return;
    }
    const lines = data.lines ?? [];
    pre.textContent =
      lines.length > 0 ? lines.join("\n") : "(No log lines in this range.)";
    if (hint instanceof HTMLElement) {
      const parts: string[] = [];
      if (data.log_level) {
        const lv = data.log_level.toUpperCase();
        const levelNote =
          lv === "DEBUG"
            ? "all slog output"
            : lv === "INFO"
              ? "HTTP requests, warnings, errors"
              : lv === "WARN"
                ? "warnings and errors only"
                : lv === "ERROR"
                  ? "errors only (no request lines)"
                  : "see LOG_LEVEL";
        parts.push(`level ${data.log_level} (${levelNote})`);
      }
      if (data.path) parts.push(data.path);
      if (data.since || data.until) {
        const range = [data.since ?? "…", data.until ?? "…"].join(" → ");
        parts.push(`range ${range}`);
      } else {
        parts.push("latest tail");
      }
      if (typeof data.matched === "number") {
        parts.push(`${data.matched} matched`);
      }
      if (data.truncated) {
        parts.push("file partially scanned (see API docs)");
      }
      hint.textContent = parts.join(" · ");
      hint.hidden = false;
    }
    if (statusEl) statusEl.textContent = "";
  } catch {
    pre.textContent = "Network error while loading logs.";
  }
}

function buildTabsAndPanels(
  root: HTMLElement,
  specs: AdminTableSpec[],
  signal: AbortSignal,
  options?: { includeLogsTab?: boolean }
): void {
  const tablist = root.querySelector("#admin-tablist");
  const panelMount = root.querySelector("#admin-panel-mount");
  if (!tablist || !panelMount) return;

  tablist.replaceChildren();
  panelMount.replaceChildren();

  specs.forEach((spec, i) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className =
      "admin-tab" + (i === 0 ? " admin-tab--active" : "");
    btn.setAttribute("role", "tab");
    btn.id = `tab-btn-${i}`;
    btn.setAttribute("aria-selected", i === 0 ? "true" : "false");
    btn.setAttribute("aria-controls", `tab-panel-${i}`);
    btn.dataset.tab = String(i);
    btn.textContent = spec.label;
    tablist.appendChild(btn);

    const section = document.createElement("section");
    section.id = `tab-panel-${i}`;
    section.className =
      "admin-tab-panel" + (i === 0 ? " admin-tab-panel--active" : "");
    section.setAttribute("role", "tabpanel");
    section.setAttribute("aria-labelledby", `tab-btn-${i}`);
    if (i !== 0) section.hidden = true;

    const wrap = document.createElement("div");
    wrap.className = "admin-table-wrap";
    wrap.dataset.adminSlug = spec.slug;
    wrap.appendChild(spec.table);
    section.appendChild(wrap);
    panelMount.appendChild(section);
  });

  if (options?.includeLogsTab) {
    const i = specs.length;
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "admin-tab";
    btn.setAttribute("role", "tab");
    btn.id = `tab-btn-${i}`;
    btn.setAttribute("aria-selected", "false");
    btn.setAttribute("aria-controls", `tab-panel-${i}`);
    btn.dataset.tab = String(i);
    btn.textContent = "Logs";
    tablist.appendChild(btn);

    const section = document.createElement("section");
    section.id = `tab-panel-${i}`;
    section.className = "admin-tab-panel";
    section.setAttribute("role", "tabpanel");
    section.setAttribute("aria-labelledby", `tab-btn-${i}`);
    section.setAttribute("data-admin-logs-panel", "");
    section.hidden = true;

    const toolbar = document.createElement("div");
    toolbar.className = "admin-logs-toolbar";

    const sinceLabel = document.createElement("label");
    sinceLabel.className = "admin-logs-range-label";
    sinceLabel.htmlFor = "admin-logs-since";
    sinceLabel.textContent = "From";
    const sinceInput = document.createElement("input");
    sinceInput.type = "datetime-local";
    sinceInput.id = "admin-logs-since";
    sinceInput.className = "admin-logs-range-input";
    sinceLabel.appendChild(sinceInput);
    toolbar.appendChild(sinceLabel);

    const untilLabel = document.createElement("label");
    untilLabel.className = "admin-logs-range-label";
    untilLabel.htmlFor = "admin-logs-until";
    untilLabel.textContent = "Until";
    const untilInput = document.createElement("input");
    untilInput.type = "datetime-local";
    untilInput.id = "admin-logs-until";
    untilInput.className = "admin-logs-range-input";
    untilLabel.appendChild(untilInput);
    toolbar.appendChild(untilLabel);

    const refresh = document.createElement("button");
    refresh.type = "button";
    refresh.id = "admin-logs-refresh";
    refresh.className = "admin-logs-refresh";
    refresh.textContent = "Refresh";
    toolbar.appendChild(refresh);

    const clearRange = document.createElement("button");
    clearRange.type = "button";
    clearRange.id = "admin-logs-clear-range";
    clearRange.className = "admin-logs-refresh";
    clearRange.textContent = "Clear range";
    toolbar.appendChild(clearRange);

    const hint = document.createElement("p");
    hint.id = "admin-logs-hint";
    hint.className = "admin-logs-hint";
    hint.hidden = true;

    const pre = document.createElement("pre");
    pre.id = "admin-logs-view";
    pre.className = "admin-logs";

    section.appendChild(toolbar);
    section.appendChild(hint);
    section.appendChild(pre);
    panelMount.appendChild(section);

    const reloadLogs = () => {
      const statusEl = root.querySelector("#admin-status");
      void loadAdminLogs(
        root,
        statusEl instanceof HTMLElement ? statusEl : null
      );
    };

    refresh.addEventListener("click", reloadLogs, { signal });
    clearRange.addEventListener(
      "click",
      () => {
        sinceInput.value = "";
        untilInput.value = "";
        reloadLogs();
      },
      { signal }
    );
  }

  const statusEl = root.querySelector("#admin-status");
  setupTabs(
    root,
    signal,
    (panel) => {
      if (panel.hasAttribute("data-admin-logs-panel")) {
        void loadAdminLogs(
          root,
          statusEl instanceof HTMLElement ? statusEl : null
        );
      }
    }
  );
}

function setupTabs(
  root: HTMLElement,
  signal: AbortSignal,
  onTabShown?: (panel: HTMLElement) => void
): void {
  root.querySelectorAll<HTMLButtonElement>("#admin-tablist .admin-tab").forEach((btn) => {
    btn.addEventListener(
      "click",
      () => {
        const idx = btn.dataset.tab;
        if (idx === undefined) return;

        root.querySelectorAll<HTMLButtonElement>("#admin-tablist .admin-tab").forEach((t) => {
          const active = t === btn;
          t.classList.toggle("admin-tab--active", active);
          t.setAttribute("aria-selected", active ? "true" : "false");
        });

        root.querySelectorAll<HTMLElement>("#admin-panel-mount .admin-tab-panel").forEach((panel) => {
          const on = panel.id === `tab-panel-${idx}`;
          panel.classList.toggle("admin-tab-panel--active", on);
          panel.toggleAttribute("hidden", !on);
          if (on) onTabShown?.(panel);
        });
      },
      { signal }
    );
  });
}

export function mountAdmin(): () => void {
  const adminRoot = document.getElementById("admin-app");
  if (!(adminRoot instanceof HTMLElement)) return () => {};

  const specs = collectAdminTables();

  adminRoot.innerHTML = ADMIN_SHELL;
  const ac = new AbortController();
  const { signal } = ac;

  const loginPanel = adminRoot.querySelector("#login-panel");
  const dashboard = adminRoot.querySelector("#dashboard");
  const loginForm = adminRoot.querySelector("#login-form") as HTMLFormElement | null;
  const loginError = adminRoot.querySelector("#login-error") as HTMLElement | null;
  const adminStatus = adminRoot.querySelector("#admin-status");

  if (specs && specs.length > 0) {
    buildTabsAndPanels(adminRoot, specs, signal, { includeLogsTab: true });
  } else if (adminStatus) {
    adminStatus.textContent =
      "Add one or more @table blocks to content/admin.md (see docs/form-markdown.md).";
  }

  const logoutBtn = adminRoot.querySelector("#logout-btn");

  function showLogin(): void {
    loginPanel?.removeAttribute("hidden");
    dashboard?.setAttribute("hidden", "");
  }

  function showDashboard(): void {
    loginPanel?.setAttribute("hidden", "");
    dashboard?.removeAttribute("hidden");
  }

  async function checkSession(): Promise<boolean> {
    const res = await adminFetch("/admin/session");
    return res.ok;
  }

  function tbodyInWrap(wrap: Element): HTMLTableSectionElement | null {
    const table = wrap.querySelector("table");
    if (!table) return null;
    return ensureTbody(table);
  }

  async function loadSubmissions(): Promise<void> {
    const root = document.getElementById("admin-app");
    if (!root) return;
    const wraps = root.querySelectorAll<HTMLDivElement>(
      "#admin-panel-mount .admin-table-wrap[data-admin-slug]"
    );
    if (!wraps.length || !adminStatus) return;

    adminStatus.textContent = "Loading…";

    for (const w of wraps) {
      const tb = tbodyInWrap(w);
      if (tb) tb.innerHTML = "";
    }

    const slugList = [...wraps].map((w) => w.dataset.adminSlug ?? "");

    try {
      const handlers = slugList.map((slug) => resolveListHandler(slug));
      const responses = await Promise.all(
        handlers.map((h) => adminFetch(h.path))
      );

      if (signal.aborted) return;

      if (responses.some((r) => r.status === 401)) {
        showLogin();
        adminStatus.textContent = "";
        return;
      }

      const failed = responses.filter((r) => !r.ok);
      if (failed.length > 0) {
        adminStatus.textContent = "Could not load one or more lists.";
        return;
      }

      const payloads = await Promise.all(responses.map((r) => r.json()));

      wraps.forEach((wrap, i) => {
        const slug = slugList[i];
        const handler = handlers[i];
        const table = wrap.querySelector("table");
        if (!table) return;
        const tbody = ensureTbody(table);
        const colspan = theadColCount(table);
        tbody.innerHTML = handler.render(payloads[i], colspan);
      });

      adminStatus.textContent = "";
    } catch {
      if (!signal.aborted) {
        adminStatus.textContent = "Network error while loading submissions.";
      }
    }
  }

  loginForm?.addEventListener(
    "submit",
    (e) => {
      e.preventDefault();
      void (async () => {
        const fd = new FormData(loginForm);
        const password = String(fd.get("password") ?? "");
        if (!loginError) return;
        loginError.hidden = true;
        loginError.textContent = "";

        try {
          const res = await adminFetch("/admin/login", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: new URLSearchParams({ password }).toString(),
          });

          if (signal.aborted) return;

          if (!res.ok) {
            if (res.status === 429) {
              loginError.textContent =
                "Too many sign-in attempts. Please wait a minute and try again.";
              loginError.hidden = false;
              return;
            }
            const data = (await res.json().catch(() => null)) as {
              error?: string;
            } | null;
            loginError.textContent =
              data?.error === "invalid credentials"
                ? "Incorrect password."
                : "Could not sign in.";
            loginError.hidden = false;
            return;
          }

          showDashboard();
          await loadSubmissions();
        } catch {
          if (!signal.aborted) {
            loginError.textContent = "Network error.";
            loginError.hidden = false;
          }
        }
      })();
    },
    { signal }
  );

  logoutBtn?.addEventListener(
    "click",
    () => {
      void (async () => {
        try {
          await adminFetch("/admin/logout", { method: "POST" });
        } catch {
          /* still sign out locally */
        }
        if (signal.aborted) return;
        showLogin();
        loginForm?.reset();
      })();
    },
    { signal }
  );

  void (async () => {
    if (signal.aborted) return;
    if (await checkSession()) {
      if (signal.aborted) return;
      showDashboard();
      await loadSubmissions();
    } else {
      showLogin();
    }
  })();

  return () => ac.abort();
}
