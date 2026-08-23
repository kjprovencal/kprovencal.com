import { publicApiUrl } from "./api-base";
import { escapeAttr, escapeHtml } from "./html-escape";
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

type RsvpAbandonRow = {
  id: string;
  created_at: string;
  name: string;
  email: string;
  guest_count: number;
  meals?: string[];
  notes: string;
  reason: string;
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

/** Must stay in sync with `allowedWeddingMealLabels` in `api/public.go`. */
const RSVP_MEAL_LABELS = [
  "Chicken Alfredo",
  "Scampi",
  "Verdura al Napoleon",
] as const;

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

type RsvpStats = {
  submissions: number;
  guests: number;
  meals: Map<string, number>;
};

function parseMealLabelFromLine(line: string): string | null {
  const idx = line.indexOf(": ");
  if (idx < 0) return null;
  const label = line.slice(idx + 2).trim();
  return label || null;
}

function computeRsvpStats(rows: WeddingRSVP[]): RsvpStats {
  const meals = new Map<string, number>();
  let guests = 0;

  for (const row of rows) {
    guests += row.guest_count;
    for (const line of row.meals ?? []) {
      const label = parseMealLabelFromLine(line);
      if (!label) continue;
      meals.set(label, (meals.get(label) ?? 0) + 1);
    }
  }

  return { submissions: rows.length, guests, meals };
}

function sortMealStatEntries(
  entries: Array<[string, number]>
): Array<[string, number]> {
  return entries.sort((a, b) => {
    const ai = RSVP_MEAL_LABELS.indexOf(
      a[0] as (typeof RSVP_MEAL_LABELS)[number]
    );
    const bi = RSVP_MEAL_LABELS.indexOf(
      b[0] as (typeof RSVP_MEAL_LABELS)[number]
    );
    if (ai >= 0 && bi >= 0) return ai - bi;
    if (ai >= 0) return -1;
    if (bi >= 0) return 1;
    return a[0].localeCompare(b[0]);
  });
}

function renderRsvpStatsHtml(stats: RsvpStats): string {
  const mealEntries = sortMealStatEntries([...stats.meals.entries()]);
  const mealsHtml =
    mealEntries.length === 0
      ? '<p class="admin-rsvp-stats__meals-empty">No meal choices yet.</p>'
      : `<ul class="admin-rsvp-stats__meals">${mealEntries
          .map(
            ([label, count]) =>
              `<li><span class="admin-rsvp-stats__meal-label">${escapeHtml(
                label
              )}</span> <span class="admin-rsvp-stats__meal-count">${count}</span></li>`
          )
          .join("")}</ul>`;

  return `
    <h2 class="admin-rsvp-stats__title">RSVP summary</h2>
    <dl class="admin-rsvp-stats__grid">
      <div class="admin-rsvp-stats__item">
        <dt>Submissions</dt>
        <dd>${stats.submissions}</dd>
      </div>
      <div class="admin-rsvp-stats__item">
        <dt>Guests</dt>
        <dd>${stats.guests}</dd>
      </div>
    </dl>
    <div class="admin-rsvp-stats__meals-wrap">
      <h3 class="admin-rsvp-stats__meals-title">Meals</h3>
      ${mealsHtml}
    </div>
  `.trim();
}

function setRsvpStatsLoading(el: HTMLElement): void {
  el.hidden = false;
  el.innerHTML =
    '<p class="admin-rsvp-stats__loading">Loading RSVP summary…</p>';
}

function setRsvpStatsError(el: HTMLElement): void {
  el.hidden = false;
  el.innerHTML =
    '<p class="admin-rsvp-stats__error">Could not load RSVP summary.</p>';
}

function updateRsvpStats(el: HTMLElement, rows: WeddingRSVP[]): void {
  el.hidden = false;
  el.innerHTML = renderRsvpStatsHtml(computeRsvpStats(rows));
}

function weddingRowsHtml(rows: WeddingRSVP[], colspan: number): string {
  if (rows.length === 0) {
    return `<tr><td colspan="${colspan}" class="admin-empty">No RSVPs yet.</td></tr>`;
  }
  return rows
    .map(
      (r) =>
        `<tr data-rsvp-id="${escapeAttr(r.id)}"><td>${escapeHtml(
          formatWhen(r.created_at)
        )}</td><td>${escapeHtml(r.name)}</td><td>${escapeHtml(
          r.email
        )}</td><td>${r.guest_count}</td><td>${mealListHtml(
          r.meals ?? []
        )}</td><td class="admin-cell-notes">${escapeHtml(
          r.notes || "—"
        )}</td><td class="admin-cell-actions"><button type="button" class="admin-row-action" data-rsvp-action="edit" data-rsvp-id="${escapeAttr(
          r.id
        )}">Edit</button> <button type="button" class="admin-row-action admin-row-action--danger" data-rsvp-action="remove" data-rsvp-id="${escapeAttr(
          r.id
        )}">Remove</button></td></tr>`
    )
    .join("");
}

function rsvpAbandonRowsHtml(rows: RsvpAbandonRow[], colspan: number): string {
  if (!Array.isArray(rows)) {
    return `<tr><td colspan="${colspan}" class="admin-empty">Invalid response (expected a JSON array).</td></tr>`;
  }
  if (rows.length === 0) {
    return `<tr><td colspan="${colspan}" class="admin-empty">No incomplete RSVPs yet.</td></tr>`;
  }
  return rows
    .map((r) => {
      const guests =
        r.guest_count < 0 ? "—" : escapeHtml(String(r.guest_count));
      return `<tr><td>${escapeHtml(formatWhen(r.created_at))}</td><td>${escapeHtml(
        r.reason || "—"
      )}</td><td>${escapeHtml(r.name || "—")}</td><td>${escapeHtml(
        r.email || "—"
      )}</td><td>${guests}</td><td>${mealListHtml(
        r.meals ?? []
      )}</td><td class="admin-cell-notes">${escapeHtml(r.notes || "—")}</td></tr>`;
    })
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
  "rsvp-abandons": {
    path: "/admin/rsvp-abandons",
    render: (data, colspan) =>
      rsvpAbandonRowsHtml(data as RsvpAbandonRow[], colspan),
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
  <div id="admin-rsvp-stats" class="admin-rsvp-stats" hidden aria-live="polite"></div>
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

function mealLabelFromStoredLine(line: string, guestIndex: number): string {
  const prefix = `Guest ${guestIndex}: `;
  if (line.startsWith(prefix)) {
    return line.slice(prefix.length).trim();
  }
  return "";
}

function mealOptionsHtml(selectedLabel: string): string {
  const opts = RSVP_MEAL_LABELS.map((label) => {
    const sel = label === selectedLabel ? " selected" : "";
    return `<option value="${escapeAttr(label)}"${sel}>${escapeHtml(label)}</option>`;
  }).join("");
  return `<option value="" disabled${selectedLabel ? "" : " selected"}>Choose…</option>${opts}`;
}

function buildMealFieldsHtml(guestCount: number, meals: string[]): string {
  if (guestCount <= 0) {
    return `<p class="admin-muted">No meal choices when guest count is 0.</p>`;
  }
  const fields: string[] = [];
  for (let i = 1; i <= guestCount; i++) {
    const selected = mealLabelFromStoredLine(meals[i - 1] ?? "", i);
    fields.push(`
      <label class="admin-edit-form__label">Guest ${i} meal
        <select name="meal_${i}" class="admin-edit-form__input" required>
          ${mealOptionsHtml(selected)}
        </select>
      </label>`);
  }
  return fields.join("");
}

function ensureRsvpEditDialog(root: HTMLElement): HTMLDialogElement {
  const existing = root.querySelector("#admin-rsvp-edit");
  if (existing instanceof HTMLDialogElement) return existing;

  const dialog = document.createElement("dialog");
  dialog.id = "admin-rsvp-edit";
  dialog.className = "admin-edit-dialog";
  dialog.innerHTML = `
    <form method="dialog" id="admin-rsvp-edit-form" class="admin-edit-form">
      <h2 class="admin-edit-form__title">Edit RSVP</h2>
      <input type="hidden" name="id" id="admin-rsvp-edit-id" />
      <label class="admin-edit-form__label">Name
        <input type="text" name="name" id="admin-rsvp-edit-name" class="admin-edit-form__input" required maxlength="120" />
      </label>
      <label class="admin-edit-form__label">Email
        <input type="email" name="email" id="admin-rsvp-edit-email" class="admin-edit-form__input" required maxlength="254" />
      </label>
      <label class="admin-edit-form__label">Guests
        <input type="number" name="guest_count" id="admin-rsvp-edit-guests" class="admin-edit-form__input" required min="0" max="8" />
      </label>
      <div id="admin-rsvp-edit-meals" class="admin-edit-form__meals"></div>
      <label class="admin-edit-form__label">Notes
        <textarea name="notes" id="admin-rsvp-edit-notes" class="admin-edit-form__input admin-edit-form__textarea" maxlength="2000" rows="3"></textarea>
      </label>
      <p id="admin-rsvp-edit-error" class="admin-error" role="alert" hidden></p>
      <div class="admin-edit-form__actions">
        <button type="submit" value="save" class="admin-edit-form__save">Save</button>
        <button type="submit" value="cancel" class="admin-edit-form__cancel" formnovalidate>Cancel</button>
      </div>
    </form>
  `;
  root.appendChild(dialog);
  return dialog;
}

function openRsvpEditDialog(
  root: HTMLElement,
  row: WeddingRSVP,
  signal: AbortSignal
): Promise<WeddingRSVP | null> {
  const dialog = ensureRsvpEditDialog(root);
  const form = dialog.querySelector("#admin-rsvp-edit-form");
  const idInput = dialog.querySelector("#admin-rsvp-edit-id");
  const nameInput = dialog.querySelector("#admin-rsvp-edit-name");
  const emailInput = dialog.querySelector("#admin-rsvp-edit-email");
  const guestsInput = dialog.querySelector("#admin-rsvp-edit-guests");
  const notesInput = dialog.querySelector("#admin-rsvp-edit-notes");
  const mealsMount = dialog.querySelector("#admin-rsvp-edit-meals");
  const errorEl = dialog.querySelector("#admin-rsvp-edit-error");

  if (
    !(form instanceof HTMLFormElement) ||
    !(idInput instanceof HTMLInputElement) ||
    !(nameInput instanceof HTMLInputElement) ||
    !(emailInput instanceof HTMLInputElement) ||
    !(guestsInput instanceof HTMLInputElement) ||
    !(notesInput instanceof HTMLTextAreaElement) ||
    !(mealsMount instanceof HTMLElement) ||
    !(errorEl instanceof HTMLElement)
  ) {
    return Promise.resolve(null);
  }

  idInput.value = row.id;
  nameInput.value = row.name;
  emailInput.value = row.email;
  guestsInput.value = String(row.guest_count);
  notesInput.value = row.notes ?? "";
  mealsMount.innerHTML = buildMealFieldsHtml(row.guest_count, row.meals ?? []);
  errorEl.hidden = true;
  errorEl.textContent = "";

  const refreshMeals = () => {
    const n = Number(guestsInput.value);
    const guestCount = Number.isFinite(n) ? Math.max(0, Math.min(8, Math.trunc(n))) : 0;
    const prevMeals: string[] = [];
    for (let i = 1; i <= 8; i++) {
      const sel = form.querySelector(`select[name="meal_${i}"]`);
      if (sel instanceof HTMLSelectElement && sel.value) {
        prevMeals.push(`Guest ${i}: ${sel.value}`);
      }
    }
    mealsMount.innerHTML = buildMealFieldsHtml(guestCount, prevMeals);
  };

  return new Promise((resolve) => {
    const onGuestChange = () => refreshMeals();
    guestsInput.addEventListener("change", onGuestChange, { signal });
    guestsInput.addEventListener("input", onGuestChange, { signal });

    const finish = (value: WeddingRSVP | null) => {
      dialog.removeEventListener("close", onClose);
      resolve(value);
    };

    const onClose = () => {
      if (dialog.returnValue !== "save") {
        finish(null);
        return;
      }
      const guests = Number(guestsInput.value);
      const guestCount = Number.isFinite(guests)
        ? Math.max(0, Math.min(8, Math.trunc(guests)))
        : 0;
      const meals: string[] = [];
      for (let i = 1; i <= guestCount; i++) {
        const sel = form.querySelector(`select[name="meal_${i}"]`);
        if (!(sel instanceof HTMLSelectElement) || !sel.value) {
          errorEl.textContent = `Choose a meal for guest ${i}.`;
          errorEl.hidden = false;
          dialog.showModal();
          return;
        }
        meals.push(`Guest ${i}: ${sel.value}`);
      }
      finish({
        id: idInput.value,
        created_at: row.created_at,
        name: nameInput.value.trim(),
        email: emailInput.value.trim(),
        guest_count: guestCount,
        meals,
        notes: notesInput.value.trim(),
      });
    };

    dialog.addEventListener("close", onClose, { signal });
    if (!dialog.open) dialog.showModal();
  });
}

export function mountAdmin(): () => void {
  const adminApp = document.getElementById("admin-app");
  if (!(adminApp instanceof HTMLElement)) return () => {};
  const adminRoot: HTMLElement = adminApp;

  const specs = collectAdminTables();

  adminRoot.innerHTML = ADMIN_SHELL;
  const ac = new AbortController();
  const { signal } = ac;

  const loginPanel = adminRoot.querySelector("#login-panel");
  const dashboard = adminRoot.querySelector("#dashboard");
  const loginForm = adminRoot.querySelector("#login-form") as HTMLFormElement | null;
  const loginError = adminRoot.querySelector("#login-error") as HTMLElement | null;
  const adminStatus = adminRoot.querySelector("#admin-status");

  const rsvpById = new Map<string, WeddingRSVP>();
  const showRsvpStats =
    specs?.some((s) => (SLUG_ALIASES[s.slug] ?? s.slug) === "rsvps") ?? false;
  const rsvpStatsEl = adminRoot.querySelector("#admin-rsvp-stats");

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
    if (
      showRsvpStats &&
      rsvpStatsEl instanceof HTMLElement
    ) {
      setRsvpStatsLoading(rsvpStatsEl);
    }

    for (const w of wraps) {
      const tb = tbodyInWrap(w);
      if (tb) tb.innerHTML = "";
    }

    const slugList = [...wraps].map((w) => w.dataset.adminSlug ?? "");

    try {
      const handlers = slugList.map((slug) => resolveListHandler(slug));
      const outcomes = await Promise.all(
        [...wraps].map(async (wrap, i) => {
          const slug = slugList[i];
          const handler = handlers[i];
          const table = wrap.querySelector("table");
          const colspan = table ? theadColCount(table) : 1;
          try {
            const res = await adminFetch(handler.path);
            if (res.status === 401) {
              return { kind: "unauthorized" as const };
            }
            if (!res.ok) {
              return {
                kind: "error" as const,
                wrap,
                colspan,
                message: `Could not load ${slug} (HTTP ${res.status}).`,
              };
            }
            let data: unknown;
            try {
              data = await res.json();
            } catch {
              return {
                kind: "error" as const,
                wrap,
                colspan,
                message: `Could not load ${slug} (invalid JSON — is this admin API path proxied to Go?).`,
              };
            }
            return { kind: "ok" as const, wrap, handler, colspan, data, slug };
          } catch {
            return {
              kind: "error" as const,
              wrap,
              colspan,
              message: `Could not load ${slug} (network error).`,
            };
          }
        })
      );

      if (signal.aborted) return;

      if (outcomes.some((o: (typeof outcomes)[number]) => o.kind === "unauthorized")) {
        showLogin();
        adminStatus.textContent = "";
        if (rsvpStatsEl instanceof HTMLElement) {
          rsvpStatsEl.hidden = true;
        }
        return;
      }

      let failed = 0;
      let rsvpStatsRows: WeddingRSVP[] | undefined;
      let rsvpStatsFailed = false;

      for (const outcome of outcomes) {
        if (outcome.kind === "error") {
          failed++;
          const slug = outcome.wrap.dataset.adminSlug ?? "";
          if ((SLUG_ALIASES[slug] ?? slug) === "rsvps") {
            rsvpStatsFailed = true;
          }
          const tbody = tbodyInWrap(outcome.wrap);
          if (tbody) {
            tbody.innerHTML = `<tr><td colspan="${outcome.colspan}" class="admin-empty">${escapeHtml(
              outcome.message
            )}</td></tr>`;
          }
          continue;
        }
        if (outcome.kind !== "ok") continue;
        const table = outcome.wrap.querySelector("table");
        if (!table) continue;
        const tbody = ensureTbody(table);
        tbody.innerHTML = outcome.handler.render(outcome.data, outcome.colspan);

        const listKey =
          SLUG_ALIASES[outcome.slug] ?? outcome.slug;
        if (listKey === "rsvps" && Array.isArray(outcome.data)) {
          rsvpStatsRows = outcome.data as WeddingRSVP[];
          rsvpById.clear();
          for (const row of rsvpStatsRows) {
            if (row && typeof row.id === "string") {
              rsvpById.set(row.id, row);
            }
          }
        }
      }

      if (showRsvpStats && rsvpStatsEl instanceof HTMLElement) {
        if (rsvpStatsFailed) {
          setRsvpStatsError(rsvpStatsEl);
        } else if (rsvpStatsRows) {
          updateRsvpStats(rsvpStatsEl, rsvpStatsRows);
        }
      }

      adminStatus.textContent =
        failed > 0 ? "Some lists could not be loaded." : "";
    } catch {
      if (!signal.aborted) {
        adminStatus.textContent = "Unexpected error while loading submissions.";
      }
    }
  }

  async function removeRsvp(id: string): Promise<void> {
    const row = rsvpById.get(id);
    const label = row?.name ? `RSVP for ${row.name}` : "this RSVP";
    if (!window.confirm(`Remove ${label}? This cannot be undone.`)) return;

    if (adminStatus) adminStatus.textContent = "Removing…";
    try {
      const res = await adminFetch(`/admin/rsvps/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
      if (signal.aborted) return;
      if (res.status === 401) {
        showLogin();
        return;
      }
      if (!res.ok) {
        if (adminStatus) {
          adminStatus.textContent = `Could not remove RSVP (HTTP ${res.status}).`;
        }
        return;
      }
      await loadSubmissions();
    } catch {
      if (!signal.aborted && adminStatus) {
        adminStatus.textContent = "Network error while removing RSVP.";
      }
    }
  }

  async function editRsvp(id: string): Promise<void> {
    const existing = rsvpById.get(id);
    if (!existing) {
      if (adminStatus) adminStatus.textContent = "RSVP not found in the current list.";
      return;
    }
    const edited = await openRsvpEditDialog(adminRoot, existing, signal);
    if (!edited || signal.aborted) return;

    if (adminStatus) adminStatus.textContent = "Saving…";
    try {
      const res = await adminFetch(`/admin/rsvps/${encodeURIComponent(id)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: edited.name,
          email: edited.email,
          guest_count: edited.guest_count,
          meals: edited.meals,
          notes: edited.notes,
        }),
      });
      if (signal.aborted) return;
      if (res.status === 401) {
        showLogin();
        return;
      }
      if (!res.ok) {
        const data = (await res.json().catch(() => null)) as {
          error?: string;
        } | null;
        if (adminStatus) {
          adminStatus.textContent =
            data?.error ?? `Could not save RSVP (HTTP ${res.status}).`;
        }
        return;
      }
      await loadSubmissions();
    } catch {
      if (!signal.aborted && adminStatus) {
        adminStatus.textContent = "Network error while saving RSVP.";
      }
    }
  }

  adminRoot.addEventListener(
    "click",
    (e) => {
      const t = e.target;
      if (!(t instanceof Element)) return;
      const btn = t.closest<HTMLElement>("[data-rsvp-action]");
      if (!btn || !adminRoot.contains(btn)) return;
      const action = btn.dataset.rsvpAction;
      const id = btn.dataset.rsvpId;
      if (!id) return;
      if (action === "remove") {
        void removeRsvp(id);
      } else if (action === "edit") {
        void editRsvp(id);
      }
    },
    { signal }
  );

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
