import { marked } from "marked";

type AttrMap = Record<string, string | boolean>;

const LABEL_FIRST = /^(.*?)\s*\?([a-z-]+)\?(\*)?$/;
const LABEL_AFTER = /^\?([a-z-]+)\?(\*)?\s*(.*?)$/;

let registered = false;

function escapeHtml(input: string): string {
  return input
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function sanitizeId(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function parseAttrs(input: string | null | undefined): AttrMap {
  const attrs: AttrMap = {};
  if (!input) return attrs;

  const attrRegex = /([a-zA-Z][a-zA-Z0-9_-]*)(?:=("[^"]*"|[^\s]+))?/g;
  for (const match of input.matchAll(attrRegex)) {
    const key = match[1];
    const raw = match[2];
    if (!raw) {
      attrs[key] = true;
      continue;
    }
    const value =
      raw.startsWith('"') && raw.endsWith('"') ? raw.slice(1, -1) : raw;
    attrs[key] = value;
  }
  return attrs;
}

function attrString(attrs: AttrMap): string {
  const out: string[] = [];
  for (const [key, value] of Object.entries(attrs)) {
    if (value === false) continue;
    if (value === true) {
      out.push(` ${key}`);
    } else {
      out.push(` ${key}="${escapeHtml(String(value))}"`);
    }
  }
  return out.join("");
}

function getText(attrs: AttrMap, key: string, fallback = ""): string {
  const v = attrs[key];
  return typeof v === "string" ? v : fallback;
}

function parseSpec(
  text: string
): { label: string; type: string; required: boolean } | null {
  let match = text.match(LABEL_FIRST);
  if (match) {
    return {
      label: (match[1] ?? "").trim(),
      type: (match[2] ?? "").toLowerCase(),
      required: !!match[3],
    };
  }
  match = text.match(LABEL_AFTER);
  if (match) {
    return {
      label: (match[3] ?? "").trim(),
      type: (match[1] ?? "").toLowerCase(),
      required: !!match[2],
    };
  }
  return null;
}

function parseOptions(raw: string): Array<{ value: string; label: string }> {
  return raw
    .split("|")
    .map((s) => s.trim())
    .filter(Boolean)
    .map((entry) => {
      const idx = entry.indexOf(":");
      if (idx < 0) return { value: entry, label: entry };
      return {
        value: entry.slice(0, idx),
        label: entry.slice(idx + 1),
      };
    });
}

function renderField(
  label: string,
  controlHtml: string,
  attrs: AttrMap
): string {
  const labelClass = getText(attrs, "labelClass");
  const labelFor = getText(attrs, "for");
  const forAttr = labelFor ? ` for="${escapeHtml(labelFor)}"` : "";
  const classAttr = labelClass ? ` class="${escapeHtml(labelClass)}"` : "";
  const labelHtml = label
    ? `<label${classAttr}${forAttr}>${escapeHtml(label)}</label>`
    : "";
  return `${labelHtml}\n${controlHtml}`;
}

function renderControl(
  label: string,
  type: string,
  required: boolean,
  href: string,
  title?: string | null
): string | null {
  const attrs = parseAttrs(title);
  const baseName = href === "-" ? "" : href;
  const providedId = getText(attrs, "id");
  const id = providedId || sanitizeId(baseName || label || "field");

  if (type === "form") {
    const formAttrs: AttrMap = { ...attrs };
    if (!formAttrs.id && baseName) formAttrs.id = baseName;
    return `<form${attrString(formAttrs)}>`;
  }

  if (type === "form-end") return "</form>";

  if (type === "status") {
    const pAttrs: AttrMap = { ...attrs };
    if (!pAttrs.id && baseName) pAttrs.id = baseName;
    return `<p${attrString(pAttrs)}></p>`;
  }

  if (type === "slot") {
    const divAttrs: AttrMap = { ...attrs };
    if (!divAttrs.id && baseName) divAttrs.id = baseName;
    delete divAttrs.hint;
    delete divAttrs.labelClass;
    const hint = getText(attrs, "hint");
    const hintHtml = hint
      ? `<p class="rsvp-form__hint">${escapeHtml(hint)}</p>\n`
      : "";
    return `${hintHtml}<div${attrString(divAttrs)}></div>`;
  }

  if (type === "turnstile") {
    const divAttrs: AttrMap = { ...attrs };
    delete divAttrs.labelClass;
    if (!divAttrs.id) divAttrs.id = baseName ? sanitizeId(baseName) : "cf-turnstile";
    const extra = typeof divAttrs.class === "string" ? divAttrs.class.trim() : "";
    divAttrs.class = ["rsvp-form__turnstile-host", extra].filter(Boolean).join(" ");
    const control = `<div${attrString(divAttrs)}></div>`;
    return renderField(label, control, { ...attrs, for: "" });
  }

  if (type === "submit") {
    const btnAttrs: AttrMap = { ...attrs };
    delete btnAttrs.labelClass;
    return `<button type="submit"${attrString(btnAttrs)}>${escapeHtml(
      label || "Submit"
    )}</button>`;
  }

  if (type === "textarea") {
    const taAttrs: AttrMap = { ...attrs };
    if (!taAttrs.id) taAttrs.id = id;
    if (!taAttrs.name && baseName) taAttrs.name = baseName;
    if (required) taAttrs.required = true;
    delete taAttrs.labelClass;
    const control = `<textarea${attrString(taAttrs)}></textarea>`;
    return renderField(label, control, {
      ...attrs,
      for: String(taAttrs.id || ""),
    });
  }

  if (type === "select") {
    const selectAttrs: AttrMap = { ...attrs };
    const optionsRaw = getText(attrs, "options");
    if (!selectAttrs.id) selectAttrs.id = id;
    if (!selectAttrs.name && baseName) selectAttrs.name = baseName;
    if (required) selectAttrs.required = true;
    delete selectAttrs.options;
    delete selectAttrs.labelClass;

    const options = parseOptions(optionsRaw)
      .map((opt, idx) => {
        const isPlaceholder = idx === 0 && opt.value === "";
        const selected = isPlaceholder ? " selected" : "";
        const disabled = isPlaceholder ? " disabled" : "";
        return `<option value="${escapeHtml(
          opt.value
        )}"${selected}${disabled}>${escapeHtml(opt.label)}</option>`;
      })
      .join("\n");

    const control = `<select${attrString(selectAttrs)}>\n${options}\n</select>`;
    return renderField(label, control, {
      ...attrs,
      for: String(selectAttrs.id || ""),
    });
  }

  const inputType =
    type === "text" ||
    type === "email" ||
    type === "number" ||
    type === "hidden"
      ? type
      : "text";
  const inputAttrs: AttrMap = { ...attrs };
  if (!inputAttrs.id) inputAttrs.id = id;
  if (!inputAttrs.name && baseName) inputAttrs.name = baseName;
  if (required) inputAttrs.required = true;
  delete inputAttrs.options;
  delete inputAttrs.labelClass;

  const control = `<input type="${escapeHtml(inputType)}"${attrString(
    inputAttrs
  )} />`;
  return renderField(label, control, {
    ...attrs,
    for: String(inputAttrs.id || ""),
  });
}

export function registerFormTokenExtension(): void {
  if (registered) return;
  registered = true;

  marked.use({
    renderer: {
      link(token: any): string | false {
        const href = token?.href ?? "";
        const title = token?.title ?? undefined;
        const text = this.parser.parseInline(token.tokens ?? []);
        const spec = parseSpec(text);
        if (!spec) return false;

        const out = renderControl(
          spec.label,
          spec.type,
          spec.required,
          href,
          title
        );
        return out ?? false;
      },
    },
  });
}
