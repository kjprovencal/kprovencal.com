import { describe, expect, it } from "vitest";
import {
  decideResize,
  fitWithin,
  MAX_ORIGINAL_BYTES,
  uploadContentType,
  WEB_JPEG_QUALITY,
  WEB_MAX_EDGE,
} from "./gallery-image";

describe("decideResize", () => {
  it("accepts a jpeg under the size cap", () => {
    expect(
      decideResize({ type: "image/jpeg", name: "a.jpg", size: 1000 })
    ).toEqual({
      kind: "resize",
      maxEdge: WEB_MAX_EDGE,
      quality: WEB_JPEG_QUALITY,
    });
  });

  it("accepts HEIC from the filename when the browser leaves type empty", () => {
    expect(decideResize({ type: "", name: "IMG.HEIC", size: 10 })).toMatchObject(
      { kind: "resize" }
    );
  });

  it("skips empty, oversized, and non-image files", () => {
    expect(decideResize({ type: "image/jpeg", name: "a.jpg", size: 0 }).kind).toBe(
      "skip"
    );
    expect(
      decideResize({
        type: "image/jpeg",
        name: "a.jpg",
        size: MAX_ORIGINAL_BYTES + 1,
      }).kind
    ).toBe("skip");
    expect(
      decideResize({ type: "text/plain", name: "notes.txt", size: 20 }).kind
    ).toBe("skip");
  });
});

describe("fitWithin", () => {
  it("scales the long edge down to 2048", () => {
    expect(fitWithin(4000, 3000, 2048)).toEqual({ width: 2048, height: 1536 });
    expect(fitWithin(3000, 4000, 2048)).toEqual({ width: 1536, height: 2048 });
  });

  it("leaves smaller images unchanged", () => {
    expect(fitWithin(800, 600, 2048)).toEqual({ width: 800, height: 600 });
  });

  it("returns zeros for empty dimensions", () => {
    expect(fitWithin(0, 100, 2048)).toEqual({ width: 0, height: 0 });
  });
});

describe("uploadContentType", () => {
  it("normalizes jpeg aliases and filename fallbacks", () => {
    expect(uploadContentType("image/jpg", "a.bin")).toBe("image/jpeg");
    expect(uploadContentType("", "pic.PNG")).toBe("image/png");
    expect(uploadContentType("application/octet-stream", "pic.heic")).toBe(
      "image/heic"
    );
  });
});
