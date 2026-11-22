import { describe, expect, it } from "vitest";
import { galleryTokenFromPath } from "./gallery-path";

const token = "ab".repeat(16);

describe("galleryTokenFromPath", () => {
  it("reads a 32-character hex token", () => {
    expect(galleryTokenFromPath(`/gallery/${token}`)).toBe(token);
    expect(galleryTokenFromPath(`/gallery/${token.toUpperCase()}/`)).toBe(token);
  });

  it("ignores other site paths", () => {
    expect(galleryTokenFromPath("/")).toBeNull();
    expect(galleryTokenFromPath("/admin")).toBeNull();
    expect(galleryTokenFromPath("/gallery/short")).toBeNull();
    expect(galleryTokenFromPath("/gallery/")).toBeNull();
  });
});
