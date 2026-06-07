# API

Small Go HTTP service for this site: **Badger** (embedded key-value) for storage, `net/http` routing, optional **CORS** when the static site lives on another origin (e.g. `www` vs `api`).

## Requirements

- Go **1.23+** (see `go.mod`)

## Configure

Copy `env.example` into your process environment (shell, systemd, hosting dashboard). Required:

| Variable                | Purpose                                             |
| ----------------------- | --------------------------------------------------- |
| `SESSION_SECRET`        | At least 32 characters; signs admin session cookies |
| `ADMIN_PASSWORD_BCRYPT` | Bcrypt hash of the admin password                   |

Generate the hash:

```bash
go run ./cmd/hashpassword 'your-password'
```

Optional variables are documented inline in `env.example` (`LISTEN_ADDR`, `BADGER_PATH`, `CORS_ORIGIN`, `TURNSTILE_SECRET_KEY`, `TRUST_PROXY`, `APP_ENV`, `ADMIN_SESSION_HOURS`, `LOG_LEVEL`, `LOG_PATH`).

## Logging

The server logs to **stderr** with `log/slog` (text format). On the droplet, systemd stores that in the journal: `journalctl -u personal-site-api -f`. **`LOG_PATH`** defaults to `api.log` beside **`BADGER_PATH`** (e.g. `/var/www/kprovencal/data/api.log`). The same stream is written to stderr (journal) and that file; it powers **Admin → Logs** via **`GET /admin/logs`**.

Everything at or above **`LOG_LEVEL`** is stored — default **`info`** includes **every HTTP request** (`method`, `path`, `status`, `duration_ms`, `remote_addr`), plus warnings and errors. **`LOG_LEVEL=error`** keeps only errors (no request lines). Set **`LOG_LEVEL=debug`** for extra auth detail. Passwords, tokens, and full cookies are never logged.

## Rate limiting

**Nginx (production):** `deploy/nginx-http-rate-limits.conf.example` defines per-IP `limit_req` zones; `deploy/nginx-site.conf.example` applies them to static files, the SPA, `/api`, and admin API paths. This is the main defense against bots probing random URLs.

**Go API:** A per-IP sliding window limits **GET/HEAD** (default 300/min) and **POST** (default 60/min), keyed off `requestRemoteAddr` (honours **`TRUST_PROXY=1`**). **`GET /healthz`** and **`OPTIONS`** are exempt. Tune with **`RATE_LIMIT_GET_MAX`**, **`RATE_LIMIT_POST_MAX`**, or set **`RATE_LIMIT_DISABLED=1`** for local dev. **`POST /admin/login`** has a separate tighter cap (30/min).

## Run

Copy `env.example` to **`.env`** in this directory (gitignored), set `SESSION_SECRET` and `ADMIN_PASSWORD_BCRYPT`, then:

```bash
chmod +x run-dev.sh   # once
./run-dev.sh
```

The script loads `.env` and runs `go run .`. Override the file path with `API_ENV_FILE=/path/to/file ./run-dev.sh`.

Manual alternative:

```bash
export SESSION_SECRET="$(openssl rand -base64 32)"
export ADMIN_PASSWORD_BCRYPT='…'   # from hashpassword
go run .
```

Defaults: listen on **`:8080`**, data under **`./data/badger`** (created if missing).

## Build

```bash
go build -o personal-site-api .
```

## Tests

```bash
go test ./...
```

With a coverage profile:

```bash
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out
```

Vitest runs from the **repository root**: `npm test`. HTML/LCOV coverage: `npm run test:coverage` (output in `coverage/`).

**CI:** [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) runs the Go and frontend test suites with coverage on pushes to **`main`** and on pull requests. The frontend **build** step receives **`VITE_TURNSTILE_SITE_KEY`** from repository secrets (optional). Reports are uploaded to **[Codecov](https://about.codecov.io/)** (merge the repo in the Codecov app for PR comments and trends). For **private** repositories, add a **`CODECOV_TOKEN`** secret; public repos usually work without it.

## Deploy (single droplet)

Server bootstrap (nginx, systemd, TLS, firewall) and paths: **`deploy/SETUP.txt`**. Templates: **`deploy/nginx-site.conf.example`**, **`deploy/personal-site-api.service`**.

On **push to `main`**, after tests pass, the **`deploy`** job rsyncs **`dist/`**, runs **`chgrp -R www-data dist/`** (so nginx can read static files), installs the API binary with **`install(1)`** as **`www-data:www-data`**, and restarts **`personal-site-api`**. See **`deploy/SETUP.txt`** (add **`github-deploy`** to group **`www-data`**). Set repository secrets **`SSH_HOST`**, **`SSH_USER`**, **`DEPLOY_PATH`** (absolute path, no trailing slash), **`SSH_PRIVATE_KEY`**, and optionally **`VITE_TURNSTILE_SITE_KEY`** (Cloudflare Turnstile **site** key for the production static build).

## Endpoints

| Method | Path                   | Notes                                                                  |
| ------ | ---------------------- | ---------------------------------------------------------------------- |
| `GET`  | `/healthz`             | Liveness; plain `ok`                                                   |
| `GET`  | `/api/events`          | Published events (slug + title)                                        |
| `POST` | `/api/contact`         | JSON contact form                                                      |
| `POST` | `/api/rsvp`            | JSON RSVP (guests, meals, notes); optional Turnstile if `TURNSTILE_SECRET_KEY` is set |
| `POST` | `/admin/login`         | Form: `password` → sets session cookie                                 |
| `POST` | `/admin/logout`        | Clears session (authenticated)                                         |
| `GET`  | `/admin/session`       | `{ "authenticated": true }` if cookie valid                            |
| `GET`  | `/admin/contacts`      | JSON list (authenticated)                                              |
| `GET`  | `/admin/rsvps`         | JSON list of RSVPs (authenticated)                                    |
| `GET`  | `/admin/logs`          | Tail or time range of `LOG_PATH` (authenticated). Query: `limit` (max 500), `since`, `until` (RFC3339 or `YYYY-MM-DD`; date-only `until` is end of that UTC day). Response includes `matched`, `truncated` when the file exceeds the scan window. |
| `GET`  | `/admin/events`        | JSON list (authenticated)                                              |
| `POST` | `/admin/events`        | Create event (form body; authenticated)                                |

`OPTIONS` on `/api/contact` and `/api/rsvp` returns CORS preflight headers when `CORS_ORIGIN` is set. The same applies to **`/admin/*`** routes when the admin UI is loaded from another origin (credentialed `fetch`).

## Admin UI (static site)

The site is a single **`index.html`** SPA using **path-based** client routing (`/`, `/rsvp` and `/wedding-rsvp` share the same RSVP page, `/admin`). Open **`/admin`** (e.g. `http://127.0.0.1:5173/admin` with Vite). The admin view is markdown in `content/admin.md`: **`@table slug [label]`** blocks (see `src/marked-tagged-table.ts`) produce GFM **tables**; **`mount-admin.ts`** collects those tables in **document order**, builds **tabs** from them, and fills each **`<tbody>`** from **`GET /admin/{slug}`** (with tailored row renderers for `rsvps` and `contacts`, and a generic renderer for other list endpoints). A **`?slot?`** provides the login/dashboard shell. **Vite** proxies **`/admin/*`** API traffic to this server in dev, while **`GET /admin`** as an HTML document still loads the SPA (see `vite.config.ts` `proxy.bypass`). Legacy **`/admin.html`** URLs are rewritten to **`/admin`**. Deploy the static build behind a host that **falls back to `index.html`** for unknown paths (same as any History-API SPA). Old **`/#/…`** links are upgraded once to **`/…`** on load.

When the HTML is served from a **different origin** than the API, set **`CORS_ORIGIN`** to that HTML origin and **`VITE_PUBLIC_API_URL`** on the static build to your API URL. With **`APP_ENV=production`**, session cookies use **`SameSite=None`** and **`Secure`** so the browser can send them on cross-site requests to the API host.

Authenticated list endpoints return JSON with **snake_case** field names (`created_at`, `guest_count`, etc.).

## CORS

Set `CORS_ORIGIN` to the **browser origin that serves your HTML** (e.g. `https://www.example.com` or `http://127.0.0.1:5173` for Vite), not the API hostname. Required for public `POST` endpoints and for the **admin page** when it is not same-origin with the API.

## Wedding RSVP and Turnstile

If `TURNSTILE_SECRET_KEY` is unset, wedding RSVPs are accepted without a token (useful for local dev). In production, set it to your Cloudflare Turnstile **secret** and ensure the client sends `turnstile_token` in the JSON body.

## Frontend dev

The Vite app in the repo root proxies **`/api`** and **`/admin`** to this server (see root `vite.config.ts`). For production builds, set **`VITE_PUBLIC_API_URL`** to your API origin so browser requests go to the API host.
