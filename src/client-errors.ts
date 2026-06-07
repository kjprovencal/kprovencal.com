import { publicApiUrl } from "./api-base";

type ClientErrorPayload = {
  kind: "error" | "unhandledrejection";
  message: string;
  page: string;
  source?: string;
  line?: number;
  column?: number;
  stack?: string;
  user_agent?: string;
};

const DEDUPE_MS = 60_000;
const recent = new Map<string, number>();

function pageContext(): string {
  return `${window.location.pathname}${window.location.search}`;
}

function shouldSend(key: string): boolean {
  const now = Date.now();
  const last = recent.get(key);
  if (last !== undefined && now - last < DEDUPE_MS) return false;
  recent.set(key, now);
  return true;
}

function sendClientError(payload: ClientErrorPayload): void {
  const key = `${payload.kind}|${payload.page}|${payload.message}`;
  if (!shouldSend(key)) return;

  const url = publicApiUrl("/api/client-errors");
  const body = JSON.stringify({
    ...payload,
    user_agent: payload.user_agent ?? navigator.userAgent,
  });

  if (typeof navigator.sendBeacon === "function") {
    const ok = navigator.sendBeacon(
      url,
      new Blob([body], { type: "application/json" })
    );
    if (ok) return;
  }

  void fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body,
    keepalive: true,
    credentials: "same-origin",
  }).catch(() => {
    /* avoid reporting loops */
  });
}

function messageFromReason(reason: unknown): { message: string; stack?: string } {
  if (reason instanceof Error) {
    return {
      message: reason.message || String(reason),
      stack: reason.stack,
    };
  }
  if (typeof reason === "string") {
    return { message: reason };
  }
  try {
    return { message: JSON.stringify(reason) };
  } catch {
    return { message: String(reason) };
  }
}

/** Report uncaught errors and unhandled promise rejections to POST /api/client-errors. */
export function initClientErrorReporting(): void {
  window.addEventListener(
    "error",
    (ev) => {
      const err = ev.error;
      sendClientError({
        kind: "error",
        message: ev.message || (err instanceof Error ? err.message : "Script error"),
        page: pageContext(),
        source: ev.filename || undefined,
        line: ev.lineno > 0 ? ev.lineno : undefined,
        column: ev.colno > 0 ? ev.colno : undefined,
        stack: err instanceof Error ? err.stack : undefined,
      });
    },
    true
  );

  window.addEventListener("unhandledrejection", (ev) => {
    const { message, stack } = messageFromReason(ev.reason);
    sendClientError({
      kind: "unhandledrejection",
      message,
      page: pageContext(),
      stack,
    });
  });
}
