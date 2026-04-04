import { describe, expect, it } from "vitest";
import { escapeAttr, escapeHtml } from "./html-escape";

describe("escapeHtml", () => {
  it("escapes HTML special characters", () => {
    expect(escapeHtml(`<>&"`)).toBe("&lt;&gt;&amp;&quot;");
  });
});

describe("escapeAttr", () => {
  it("escapes for attribute context including apostrophe", () => {
    expect(escapeAttr(`a'b`)).toBe("a&#39;b");
  });
});
