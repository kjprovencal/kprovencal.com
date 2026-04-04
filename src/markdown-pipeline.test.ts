import { marked } from "marked";
import { beforeAll, describe, expect, it } from "vitest";
import { configureMarked } from "./markdown-pipeline";
import { sanitizeSlug } from "./marked-tagged-table";

beforeAll(() => {
  configureMarked();
});

describe("sanitizeSlug", () => {
  it("normalizes unsafe characters", () => {
    expect(sanitizeSlug("My Table!")).toBe("my-table");
  });
});

describe("marked pipeline", () => {
  it("renders GFM headings", () => {
    const html = marked.parse("# Hello") as string;
    expect(html).toContain("<h1");
    expect(html).toContain("Hello");
  });

  it("renders emoji shortcodes", () => {
    const html = marked.parse("Hello :wave:") as string;
    expect(html).toContain("👋");
    expect(html).toContain('class="md-emoji"');
  });

  it("renders @table with data-md-table and optional tab label", () => {
    const md = `@table my-list Tab A

| H |
| - |
`;
    const html = marked.parse(md) as string;
    expect(html).toContain('data-md-table="my-list"');
    expect(html).toContain('data-md-tab-label="Tab A"');
    expect(html).toContain("md-tagged-table");
    expect(html).toContain("<tbody>");
  });

  it("keeps tokenizer state valid so a slot link after @table renders", () => {
    const md = `@table a A

| H |
| - |

[Dashboard ?slot?](admin-app)
`;
    const html = marked.parse(md) as string;
    expect(html).toContain('id="admin-app"');
  });

  it("renders a form token link as a form tag", () => {
    const md = `[RSVP ?form?](my-form "class=rsvp-form")`;
    const html = marked.parse(md) as string;
    expect(html).toContain('<form class="rsvp-form" id="my-form">');
  });
});
