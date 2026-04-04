/**
 * Absolute URL for paths on the Go server (`/api/*`, `/admin/*`).
 * Leave `VITE_PUBLIC_API_URL` unset in dev so Vite can proxy `/api` and `/admin`.
 *
 * @param baseOverride — optional base (e.g. in tests); when omitted, uses `import.meta.env.VITE_PUBLIC_API_URL`.
 */
export function publicApiUrl(path: string, baseOverride?: string): string {
  const base =
    baseOverride !== undefined
      ? baseOverride.trim()
      : (import.meta.env.VITE_PUBLIC_API_URL?.trim() ?? "");
  const p = path.startsWith("/") ? path : `/${path}`;
  if (!base) return p;
  return `${base.replace(/\/$/, "")}${p}`;
}
