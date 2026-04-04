/**
 * Absolute URL for paths on the Go server (`/api/*`, `/admin/*`).
 * Leave `VITE_PUBLIC_API_URL` unset in dev so Vite can proxy `/api` and `/admin`.
 */
export function publicApiUrl(path: string): string {
  const base = import.meta.env.VITE_PUBLIC_API_URL?.trim() ?? "";
  const p = path.startsWith("/") ? path : `/${path}`;
  if (!base) return p;
  return `${base.replace(/\/$/, "")}${p}`;
}
