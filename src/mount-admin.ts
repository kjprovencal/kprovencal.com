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

function buildTabsAndPanels(
  root: HTMLElement,
  specs: AdminTableSpec[],
  signal: AbortSignal
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

  setupTabs(root, signal);
}

function setupTabs(host: HTMLElement, signal: AbortSignal): void {
  host.querySelectorAll<HTMLButtonElement>("#admin-tablist .admin-tab").forEach((btn) => {
    btn.addEventListener(
      "click",
      () => {
        const idx = btn.dataset.tab;
        if (idx === undefined) return;

        host.querySelectorAll<HTMLButtonElement>("#admin-tablist .admin-tab").forEach((t) => {
          const active = t === btn;
          t.classList.toggle("admin-tab--active", active);
          t.setAttribute("aria-selected", active ? "true" : "false");
        });

        host.querySelectorAll<HTMLElement>("#admin-panel-mount .admin-tab-panel").forEach((panel) => {
          const on = panel.id === `tab-panel-${idx}`;
          panel.classList.toggle("admin-tab-panel--active", on);
          panel.toggleAttribute("hidden", !on);
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
    buildTabsAndPanels(adminRoot, specs, signal);
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
    const host = document.getElementById("admin-app");
    if (!host) return;
    const wraps = host.querySelectorAll<HTMLDivElement>(
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
