import { marked } from "marked";
import { initTheme } from "./theme";
import { configureMarked } from "./markdown-pipeline";
import { mountWeddingRsvp, teardownWeddingRsvp } from "./mount-rsvp";
import { mountAdmin } from "./mount-admin";

configureMarked();

const contentRoot = document.getElementById("content");
const mainEl = document.getElementById("main");

/** Single source for path → markdown loader and document title. */
const ROUTES = [
  {
    path: "/",
    title: "Kyle Provencal's Personal Site",
    load: () => import("../content/index.md?raw").then((m) => m.default),
  },
  {
    path: "/rsvp",
    title: "Wedding RSVP",
    load: () => import("../content/rsvp.md?raw").then((m) => m.default),
  },
  {
    path: "/wedding-rsvp",
    title: "Wedding RSVP",
    load: () => import("../content/rsvp.md?raw").then((m) => m.default),
  },
  {
    path: "/admin",
    title: "Admin — Submissions",
    load: () => import("../content/admin.md?raw").then((m) => m.default),
  },
] as const;

const loaders: Record<string, () => Promise<string>> = Object.fromEntries(
  ROUTES.map((r) => [r.path, r.load])
);
const titles: Record<string, string> = Object.fromEntries(
  ROUTES.map((r) => [r.path, r.title])
);

function normalizePathname(pathname: string): string {
  if (pathname === "" || pathname === "/") return "/";
  const trimmed = pathname.replace(/\/+$/, "") || "/";
  return trimmed;
}

function getRoutePath(): string {
  const path = normalizePathname(window.location.pathname);
  return path in loaders ? path : "/";
}

function isAppInternalPath(path: string): boolean {
  return path in loaders;
}

function onDocumentClickForSpaNav(e: MouseEvent): void {
  if (e.defaultPrevented || e.button !== 0) return;
  const a = (e.target as Element | null)?.closest?.("a");
  if (!a || a.target === "_blank" || a.hasAttribute("download")) return;
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;

  const rawHref = a.getAttribute("href");
  if (rawHref === null || rawHref === "") return;

  if (rawHref.startsWith("#") && !rawHref.startsWith("#/")) return;

  if (rawHref.startsWith("#/")) {
    const path = normalizePathname(`/${rawHref.slice(2)}`);
    if (!isAppInternalPath(path)) return;
    e.preventDefault();
    history.pushState(null, "", path);
    void renderRoute();
    return;
  }

  let url: URL;
  try {
    url = new URL(rawHref, window.location.origin);
  } catch {
    return;
  }
  if (url.origin !== window.location.origin) return;
  const path = normalizePathname(url.pathname);
  if (!isAppInternalPath(path)) return;
  e.preventDefault();
  history.pushState(null, "", `${path}${url.search}`);
  void renderRoute();
}

let routeTeardown: (() => void) | undefined;
let navSerial = 0;

function setRobots(noIndex: boolean): void {
  let el = document.querySelector<HTMLMetaElement>('meta[name="robots"]');
  if (!el) {
    el = document.createElement("meta");
    el.name = "robots";
    document.head.appendChild(el);
  }
  el.content = noIndex ? "noindex, nofollow" : "index, follow";
}

async function renderRoute(): Promise<void> {
  if (!contentRoot || !mainEl) return;

  const serial = ++navSerial;

  routeTeardown?.();
  routeTeardown = undefined;
  teardownWeddingRsvp();

  const path = getRoutePath();
  const load = loaders[path];
  const md = await load();
  if (serial !== navSerial || !contentRoot.isConnected) return;

  contentRoot.innerHTML = marked.parse(md) as string;

  document.title = titles[path] ?? titles["/"];
  setRobots(path === "/admin");
  mainEl.classList.toggle("admin-document", path === "/admin");

  if (path === "/rsvp" || path === "/wedding-rsvp") {
    mountWeddingRsvp();
  } else if (path === "/admin") {
    routeTeardown = mountAdmin();
  }
}

initTheme();

void renderRoute();
window.addEventListener("popstate", () => {
  void renderRoute();
});
document.addEventListener("click", onDocumentClickForSpaNav);
