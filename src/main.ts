import { marked } from "marked";
import { initTheme } from "./theme";
import { registerEmojiExtension } from "./marked-emoji";
import { registerFormTokenExtension } from "./marked-form-tokens";
import { registerTaggedTableExtension } from "./marked-tagged-table";
import { mountWeddingRsvp, teardownWeddingRsvp } from "./mount-wedding-rsvp";
import { mountAdmin } from "./mount-admin";

marked.setOptions({
  gfm: true,
  breaks: false,
});
registerEmojiExtension();
registerFormTokenExtension();
registerTaggedTableExtension();

const contentRoot = document.getElementById("content");
const mainEl = document.getElementById("main");

const loaders: Record<string, () => Promise<string>> = {
  "/": () => import("../content/index.md?raw").then((m) => m.default),
  "/wedding-rsvp": () =>
    import("../content/wedding-rsvp.md?raw").then((m) => m.default),
  "/admin": () => import("../content/admin.md?raw").then((m) => m.default),
};

const titles: Record<string, string> = {
  "/": "Kyle Provencal's Personal Site",
  "/wedding-rsvp": "Wedding RSVP",
  "/admin": "Admin — Submissions",
};

function normalizePathname(pathname: string): string {
  if (pathname === "" || pathname === "/") return "/";
  const trimmed = pathname.replace(/\/+$/, "") || "/";
  return trimmed;
}

function redirectIfLegacyHtml(): void {
  const { pathname, href } = window.location;
  const suffix = "wedding-rsvp.html";
  if (pathname.endsWith(suffix)) {
    const u = new URL(href);
    u.pathname = "/wedding-rsvp";
    u.hash = "";
    history.replaceState(null, "", u.toString());
    return;
  }
  if (pathname.endsWith("admin.html")) {
    const u = new URL(href);
    u.pathname = "/admin";
    u.hash = "";
    history.replaceState(null, "", u.toString());
  }
}

/** Old bookmarks using `/#/path` → `/path` (History API). */
function migrateHashRoute(): void {
  const hash = window.location.hash;
  if (!hash.startsWith("#/")) return;
  const path = normalizePathname(hash.slice(1));
  if (!(path in loaders)) return;
  const u = new URL(window.location.href);
  u.pathname = path;
  u.hash = "";
  history.replaceState(null, "", u.toString());
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

  if (path === "/wedding-rsvp") {
    mountWeddingRsvp();
  } else if (path === "/admin") {
    routeTeardown = mountAdmin();
  }
}

redirectIfLegacyHtml();
migrateHashRoute();
initTheme();

void renderRoute();
window.addEventListener("popstate", () => {
  void renderRoute();
});
document.addEventListener("click", onDocumentClickForSpaNav);
