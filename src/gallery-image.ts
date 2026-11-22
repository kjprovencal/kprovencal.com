/** Longest side of the JPEG shown in the gallery and offered as the Web download. */
export const WEB_MAX_EDGE = 2048;

/** JPEG quality for the web-sized copy (0–1). */
export const WEB_JPEG_QUALITY = 0.82;

export const MAX_ORIGINAL_BYTES = 50 * 1024 * 1024;

/** Web JPEG cap. A 2048px photo at quality 0.82 is usually under 2 MB. */
export const MAX_WEB_BYTES = 12 * 1024 * 1024;

const ACCEPTED_TYPES = new Set([
  "image/jpeg",
  "image/jpg",
  "image/pjpeg",
  "image/png",
  "image/webp",
  "image/heic",
  "image/heif",
]);

export type ResizeDecision =
  | { kind: "skip"; reason: string }
  | { kind: "resize"; maxEdge: number; quality: number };

export function fileExtension(name: string): string {
  const match = /\.([a-z0-9]+)$/i.exec(name.trim());
  return match ? `.${match[1].toLowerCase()}` : "";
}

export function isAcceptedImage(type: string, name: string): boolean {
  const ct = type.toLowerCase().split(";")[0]?.trim() ?? "";
  if (ACCEPTED_TYPES.has(ct)) return true;
  switch (fileExtension(name)) {
    case ".jpg":
    case ".jpeg":
    case ".png":
    case ".webp":
    case ".heic":
    case ".heif":
      return true;
    default:
      return false;
  }
}

/** Content type sent to the API. Phones sometimes leave `file.type` empty. */
export function uploadContentType(type: string, name: string): string {
  const ct = type.toLowerCase().split(";")[0]?.trim() ?? "";
  if (ct === "image/jpg" || ct === "image/pjpeg") return "image/jpeg";
  if (ACCEPTED_TYPES.has(ct)) return ct;
  switch (fileExtension(name)) {
    case ".jpg":
    case ".jpeg":
      return "image/jpeg";
    case ".png":
      return "image/png";
    case ".webp":
      return "image/webp";
    case ".heic":
      return "image/heic";
    case ".heif":
      return "image/heif";
    default:
      return ct;
  }
}

export function decideResize(file: {
  type: string;
  name: string;
  size: number;
}): ResizeDecision {
  if (file.size <= 0) return { kind: "skip", reason: "This file is empty." };
  if (file.size > MAX_ORIGINAL_BYTES) {
    return { kind: "skip", reason: "This file is larger than 50 MB." };
  }
  if (!isAcceptedImage(file.type, file.name)) {
    return {
      kind: "skip",
      reason: "Use a JPEG, PNG, WebP, or HEIC photo.",
    };
  }
  return {
    kind: "resize",
    maxEdge: WEB_MAX_EDGE,
    quality: WEB_JPEG_QUALITY,
  };
}

/** Fit width and height inside maxEdge without upscaling. */
export function fitWithin(
  width: number,
  height: number,
  maxEdge: number
): { width: number; height: number } {
  if (width <= 0 || height <= 0 || maxEdge <= 0) {
    return { width: 0, height: 0 };
  }
  const longest = Math.max(width, height);
  const scale = Math.min(1, maxEdge / longest);
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  };
}
