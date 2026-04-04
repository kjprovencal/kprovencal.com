import { describe, expect, it } from "vitest";
import { publicApiUrl } from "./api-base";

describe("publicApiUrl", () => {
  it("returns path only when base is empty", () => {
    expect(publicApiUrl("/api/x", "")).toBe("/api/x");
  });

  it("normalizes leading slash on path", () => {
    expect(publicApiUrl("api/x", "")).toBe("/api/x");
  });

  it("joins base without trailing slash", () => {
    expect(publicApiUrl("/api/x", "https://api.example.com")).toBe(
      "https://api.example.com/api/x"
    );
  });

  it("strips trailing slash from base", () => {
    expect(publicApiUrl("/api/x", "https://api.example.com/")).toBe(
      "https://api.example.com/api/x"
    );
  });
});
