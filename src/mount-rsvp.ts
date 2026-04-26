import { publicApiUrl } from "./api-base";
import { escapeAttr, escapeHtml } from "./html-escape";

const TURNSTILE_SITE_KEY =
  import.meta.env.VITE_TURNSTILE_SITE_KEY?.trim() ?? "";

/** Meal `label` strings must stay in sync with `allowedWeddingMealLabels` in `api/public.go`. */
const MEAL_OPTIONS: { value: string; label: string; description: string }[] = [
  {
    value: "chicken-alfredo",
    label: "Chicken Alfredo",
    description:
      "sliced chicken breast in a creamy parmesan alfredo sauce served on penne pasta",
  },
  {
    value: "scampi",
    label: "Scampi",
    description:
      "shrimp sautéed with diced tomatoes and parsley in a garlic butter wine sauce over penne pasta",
  },
  {
    value: "verdura-al-napoleon",
    label: "Verdura al Napoleon",
    description:
      "roasted eggplant, zucchini, summer squash, tomato, onion and peppers with marinara sauce",
  },
];

function mealSelectHtml(guestIndex: number): string {
  const options = MEAL_OPTIONS.map(
    (o) =>
      `<option value="${escapeAttr(o.value)}">${escapeHtml(o.label)}</option>`
  ).join("");
  return `
    <label class="rsvp-form__label">
      <span class="rsvp-form__meal-guest">Guest ${guestIndex}</span>
      <select class="rsvp-form__select" name="meal_${guestIndex}" required>
        <option value="" selected disabled>Choose…</option>
        ${options}
      </select>
    </label>
  `;
}

function loadTurnstileScript(): Promise<void> {
  if (window.turnstile) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src =
      "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
    s.async = true;
    s.onload = () => resolve();
    s.onerror = () => reject(new Error("Turnstile script failed to load"));
    document.head.appendChild(s);
  });
}

let turnstileWidgetId: string | undefined;

function renderMealRows(count: number, container: HTMLElement): void {
  if (!Number.isFinite(count) || count < 1) {
    container.innerHTML = "";
    return;
  }
  container.innerHTML = Array.from({ length: count }, (_, i) =>
    mealSelectHtml(i + 1)
  ).join("");
}

export function mountWeddingRsvp(): void {
  const form = document.getElementById(
    "rsvp-form"
  ) as HTMLFormElement | null;
  const guestSelect = document.getElementById(
    "rsvp-guest-count"
  ) as HTMLSelectElement | null;
  const mealContainer = document.getElementById("rsvp-meal-rows");
  const statusEl = document.getElementById("rsvp-form-status");

  function refreshSubmitEnabled(): void {
    if (!form) return;
    const submitButton = form.querySelector(
      "button[type='submit']"
    ) as HTMLButtonElement;
    submitButton.disabled = !form.checkValidity();
  }

  async function initTurnstile(): Promise<void> {
    const el = document.getElementById("cf-turnstile");
    if (!el) return;

    if (!TURNSTILE_SITE_KEY) {
      el.innerHTML =
        '<p class="rsvp-form__hint">Security check is not configured for this build.</p>';
      return;
    }

    try {
      await loadTurnstileScript();
      if (!window.turnstile) return;
      turnstileWidgetId = window.turnstile.render(el, {
        sitekey: TURNSTILE_SITE_KEY,
        theme: "auto",
        callback: () => refreshSubmitEnabled(),
        "error-callback": () => {
          console.warn("Turnstile: widget error");
          if (statusEl) {
            statusEl.textContent =
              "Security check failed to load. Please refresh the page.";
          }
        },
        "expired-callback": () => refreshSubmitEnabled(),
      });
    } catch (err) {
      console.warn("Turnstile: failed to load or render", err);
      if (statusEl) {
        statusEl.textContent =
          "Could not load security check. Please refresh the page.";
      }
    }
  }

  if (guestSelect && mealContainer) {
    guestSelect.addEventListener("change", () => {
      const n = Number(guestSelect.value);
      renderMealRows(n, mealContainer);
      if (statusEl) statusEl.textContent = "";
    });
  }

  if (form) {
    form.addEventListener("input", () => {
      refreshSubmitEnabled();
    });
  }

  void initTurnstile();

  form?.addEventListener("submit", (e) => {
    e.preventDefault();
    void (async () => {
      if (!statusEl || !form) return;

      const n = guestSelect ? Number(guestSelect.value) : null;
      if (n === null) {
        statusEl.textContent =
          "Please choose how many people are in your party.";
        return;
      }

      const fd = new FormData(form);
      const name = String(fd.get("name") ?? "").trim();
      const email = String(fd.get("email") ?? "").trim();
      if (!name || !email) {
        statusEl.textContent = "Please enter your name and email.";
        return;
      }

      const meals: string[] = [];
      for (let i = 1; i <= n; i++) {
        const v = String(fd.get(`meal_${i}`) ?? "").trim();
        if (!v) {
          statusEl.textContent = `Please choose a meal for guest ${i}.`;
          return;
        }
        const label = MEAL_OPTIONS.find((o) => o.value === v)?.label ?? v;
        meals.push(`Guest ${i}: ${label}`);
      }

      const notes = String(fd.get("notes") ?? "").trim();

      let turnstileToken: string | undefined;
      if (TURNSTILE_SITE_KEY) {
        if (!turnstileWidgetId || !window.turnstile) {
          statusEl.textContent =
            "Security check is still loading. Please wait a moment.";
          return;
        }
        turnstileToken = window.turnstile.getResponse(turnstileWidgetId);
        if (!turnstileToken) {
          statusEl.textContent = "Please complete the security check above.";
          return;
        }
      }

      const submitButton = form.querySelector(
        "button[type='submit']"
      ) as HTMLButtonElement;
      submitButton.disabled = true;
      statusEl.textContent = "Sending…";

      const payload: Record<string, unknown> = {
        name,
        email,
        guest_count: n,
        meals,
        notes,
      };
      if (turnstileToken) payload.turnstile_token = turnstileToken;

      try {
        const res = await fetch(publicApiUrl("/api/rsvp"), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        if (!res.ok) {
          const msg = await res.text().catch(() => "");
          statusEl.textContent =
            msg.trim() || "Could not send RSVP. Please try again.";
          if (turnstileWidgetId && window.turnstile) {
            window.turnstile.reset(turnstileWidgetId);
          }
          refreshSubmitEnabled();
          return;
        }

        if (turnstileWidgetId !== undefined && window.turnstile) {
          try {
            window.turnstile.remove(turnstileWidgetId);
          } catch (err) {
            console.warn("Turnstile: remove failed after successful RSVP", err);
          }
        }
        turnstileWidgetId = undefined;

        const confirmation = document.createElement("div");
        confirmation.id = "rsvp-form-confirmation";
        confirmation.className = "rsvp-form__confirmation";
        confirmation.setAttribute("role", "status");
        confirmation.setAttribute("aria-live", "polite");
        confirmation.tabIndex = -1;
        confirmation.textContent =
          "Thanks — your RSVP has been submitted. We'll send you a confirmation email shortly.";
        form.insertAdjacentElement("afterend", confirmation);
        form.hidden = true;
        confirmation.focus();
      } catch {
        statusEl.textContent = "Network error. Please try again.";
        if (turnstileWidgetId && window.turnstile) {
          window.turnstile.reset(turnstileWidgetId);
        }
        refreshSubmitEnabled();
      }
    })();
  });
}

export function teardownWeddingRsvp(): void {
  if (turnstileWidgetId !== undefined && window.turnstile) {
    try {
      window.turnstile.remove(turnstileWidgetId);
    } catch (err) {
      console.warn("Turnstile: remove failed during teardown", err);
    }
  }
  turnstileWidgetId = undefined;
}
