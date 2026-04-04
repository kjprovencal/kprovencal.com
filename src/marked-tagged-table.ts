import { Lexer, marked } from "marked";
import type { RendererThis, TokenizerThis, Tokens } from "marked";

/**
 * Named GFM tables for stable DOM targeting after `marked.parse`.
 *
 * ```markdown
 * @table my-slug Optional tab label
 *
 * | Col A | Col B |
 * | ----- | ----- |
 * ```
 *
 * Renders `<table class="md-tagged-table" id="md-table-my-slug" data-md-table="my-slug">…</table>`
 * with an empty `<tbody></tbody>` when there are no body rows (fill rows from JS).
 * If a tab label is given (after the slug on the same line), it is stored in
 * `data-md-tab-label` for the admin dashboard; quotes on the label are optional.
 */

export const MD_TABLE_ATTR = "data-md-table";
export const MD_TABLE_CLASS = "md-tagged-table";
/** Admin UI: optional human-readable tab title (otherwise derived from the slug). */
export const MD_TAB_LABEL_ATTR = "data-md-tab-label";

export type TaggedTableToken = Omit<Tokens.Table, "type"> & {
  type: "taggedTable";
  slug: string;
  tabLabel?: string;
};

const TAG_LINE =
  /^\s* {0,3}@table\s+([a-z][a-z0-9_-]*)(?:\s+([^\n]+?))?\s*(?:\n+|\s*$)/i;

/** Same idea as marked’s gfm delimiter check */
const TABLE_DELIM = /[:|]/;

let registered = false;

export function sanitizeSlug(slug: string): string {
  return (
    slug
      .toLowerCase()
      .replace(/[^a-z0-9_-]+/g, "-")
      .replace(/^-+|-+$/g, "") || "table"
  );
}

function parseTabLabel(raw: string | undefined): string | undefined {
  if (raw === undefined) return undefined;
  let s = raw.trim();
  if (!s) return undefined;
  if (
    (s.startsWith('"') && s.endsWith('"')) ||
    (s.startsWith("'") && s.endsWith("'"))
  ) {
    s = s.slice(1, -1).trim();
  }
  return s || undefined;
}

function escapeAttr(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

/**
 * Parses a GFM table immediately after `@table …` (optional blank lines).
 * `consumed` must match `afterTag`’s prefix so `raw` advances the lexer correctly.
 */
function extractTableLines(afterTag: string): {
  tableMd: string;
  consumed: string;
} | null {
  const lines = afterTag.split(/\r?\n/);
  let i = 0;

  while (i < lines.length && lines[i].trim() === "") {
    i++;
  }
  if (i >= lines.length) return null;

  const h = i;
  if (!lines[h].includes("|")) return null;
  i = h + 1;

  while (i < lines.length && lines[i].trim() === "") {
    i++;
  }
  if (i >= lines.length) return null;

  const d = i;
  if (!TABLE_DELIM.test(lines[d])) return null;
  i = d + 1;

  const bodyStart = i;
  while (i < lines.length) {
    const line = lines[i];
    if (line.trim() === "" || !line.includes("|")) break;
    i++;
  }
  const bodyEnd = i;

  const headerLine = lines[h];
  const delimLine = lines[d];
  const bodyLines = lines.slice(bodyStart, bodyEnd);
  const tableMd = [headerLine, delimLine, ...bodyLines].join("\n");

  const consumedLines = lines.slice(0, bodyEnd);
  const consumed =
    consumedLines.join("\n") + (consumedLines.length > 0 ? "\n" : "");

  return { tableMd, consumed };
}

export function taggedTableSelector(slug: string): string {
  return `table.${MD_TABLE_CLASS}[${MD_TABLE_ATTR}="${CSS.escape(slug)}"]`;
}

export function registerTaggedTableExtension(): void {
  if (registered) return;
  registered = true;

  marked.use({
    extensions: [
      {
        name: "taggedTable",
        level: "block",
        start(src: string): number | undefined {
          const idx = src.indexOf("@table");
          return idx >= 0 ? idx : undefined;
        },
        tokenizer(this: TokenizerThis, src: string): TaggedTableToken | undefined {
          const m = TAG_LINE.exec(src);
          if (!m) return undefined;

          const slug = m[1].toLowerCase();
          const tabLabel = parseTabLabel(m[2]);
          const afterTag = src.slice(m[0].length);
          const extracted = extractTableLines(afterTag);
          if (!extracted) return undefined;

          const { tableMd, consumed } = extracted;
          const outerLexer = this.lexer as unknown as {
            tokenizer: { lexer: unknown };
          };
          const sharedTokenizer = outerLexer.tokenizer;
          const savedLexerRef = sharedTokenizer.lexer;
          let lexed: ReturnType<typeof Lexer.lex>;
          try {
            lexed = Lexer.lex(tableMd + "\n", this.lexer.options);
          } finally {
            sharedTokenizer.lexer = savedLexerRef;
          }
          const tableTok = lexed.find((t): t is Tokens.Table => t.type === "table");
          if (!tableTok) return undefined;

          const fullRaw = m[0] + consumed;

          return {
            type: "taggedTable",
            raw: fullRaw,
            slug,
            tabLabel,
            align: tableTok.align,
            header: tableTok.header,
            rows: tableTok.rows,
          };
        },
        renderer(this: RendererThis, token: Tokens.Generic): string {
          const t = token as TaggedTableToken;
          const { parser } = this;
          let cell = "";
          let headerRow = "";
          for (let j = 0; j < t.header.length; j++) {
            cell += parser.renderer.tablecell(t.header[j]);
          }
          headerRow += parser.renderer.tablerow({ text: cell });

          let body = "";
          for (let j = 0; j < t.rows.length; j++) {
            const row = t.rows[j];
            cell = "";
            for (let k = 0; k < row.length; k++) {
              cell += parser.renderer.tablecell(row[k]);
            }
            body += parser.renderer.tablerow({ text: cell });
          }

          const safeSlug = escapeAttr(t.slug);
          const id = `md-table-${sanitizeSlug(t.slug)}`;
          const tbody = body ? `<tbody>${body}</tbody>` : "<tbody></tbody>";
          const labelAttr =
            t.tabLabel !== undefined
              ? ` ${MD_TAB_LABEL_ATTR}="${escapeAttr(t.tabLabel)}"`
              : "";

          return `<table class="${MD_TABLE_CLASS}" id="${id}" ${MD_TABLE_ATTR}="${safeSlug}"${labelAttr}>
<thead>
${headerRow}</thead>
${tbody}
</table>
`;
        },
      },
    ],
  });
}
