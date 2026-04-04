const STORAGE_KEY = "personal-site-theme";
export type ThemePreference = "light" | "dark" | "system";

export function getStoredPreference(): ThemePreference {
  const v = localStorage.getItem(STORAGE_KEY);
  if (v === "light" || v === "dark" || v === "system") return v;
  return "system";
}

function effectiveTheme(pref: ThemePreference): "light" | "dark" {
  if (pref === "system") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  }
  return pref;
}

function cyclePreference(current: ThemePreference): ThemePreference {
  if (current === "system") return "light";
  if (current === "light") return "dark";
  return "system";
}

export function labelFor(pref: ThemePreference): string {
  if (pref === "system") return "Theme: Auto";
  if (pref === "light") return "Theme: Light";
  return "Theme: Dark";
}

export function applyTheme(pref: ThemePreference): void {
  const resolved = effectiveTheme(pref);
  document.documentElement.dataset.theme = resolved;
  document.documentElement.dataset.preference = pref;
  const btn = document.getElementById("theme-toggle");
  if (btn) btn.setAttribute("aria-label", labelFor(pref));
}

export function initTheme(): ThemePreference {
  const pref = getStoredPreference();
  applyTheme(pref);

  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      if (getStoredPreference() === "system") applyTheme("system");
    });

  const btn = document.getElementById("theme-toggle");
  btn?.addEventListener("click", () => {
    const next = cyclePreference(getStoredPreference());
    localStorage.setItem(STORAGE_KEY, next);
    applyTheme(next);
  });

  return pref;
}
