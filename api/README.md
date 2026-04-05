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

Optional variables are documented inline in `env.example` (`LISTEN_ADDR`, `BADGER_PATH`, `CORS_ORIGIN`, `TURNSTILE_SECRET_KEY`, `TRUST_PROXY`, `APP_ENV`, `ADMIN_SESSION_HOURS`, `LOG_LEVEL`).

## Logging

The server logs to **stderr** with `log/slog` (text format). Every HTTP request logs method, path, status, duration, and client address (using the first `X-Forwarded-For` hop when present). Set **`LOG_LEVEL=debug`** to include **`requireAdmin`** rejection details; default is **`info`**. Passwords, tokens, and full cookies are never logged.

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

**CI:** [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) runs the Go and frontend test suites with coverage on pushes to **`main`** and on pull requests. Reports are uploaded to **[Codecov](https://about.codecov.io/)** (merge the repo in the Codecov app for PR comments and trends). For **private** repositories, add a **`CODECOV_TOKEN`** secret; public repos usually work without it.

## Deploy (single droplet)

Server bootstrap (nginx, systemd, TLS, firewall) and paths: **`deploy/SETUP.txt`**. Templates: **`deploy/nginx-site.conf.example`**, **`deploy/personal-site-api.service`**.

On **push to `main`**, after tests pass, the **`deploy`** job rsyncs **`dist/`**, runs **`chgrp -R www-data dist/`** (so nginx can read static files), installs the API binary with **`install(1)`** as **`www-data:www-data`**, and restarts **`personal-site-api`**. See **`deploy/SETUP.txt`** (add **`github-deploy`** to group **`www-data`**). Set repository secrets **`SSH_HOST`**, **`SSH_USER`**, **`DEPLOY_PATH`** (absolute path, no trailing slash), and **`SSH_PRIVATE_KEY`**.

## Endpoints

| Method | Path                   | Notes                                                                  |
| ------ | ---------------------- | ---------------------------------------------------------------------- |
| `GET`  | `/healthz`             | Liveness; plain `ok`                                                   |
| `GET`  | `/api/events`          | Published events (slug + title)                                        |
| `POST` | `/api/contact`         | JSON contact form                                                      |
| `POST` | `/api/rsvp`            | JSON event RSVP (`event_slug`, …)                                      |
| `POST` | `/api/wedding-rsvp`    | JSON wedding RSVP; optional Turnstile if `TURNSTILE_SECRET_KEY` is set |
| `POST` | `/admin/login`         | Form: `password` → sets session cookie                                 |
| `POST` | `/admin/logout`        | Clears session (authenticated)                                         |
| `GET`  | `/admin/session`       | `{ "authenticated": true }` if cookie valid                            |
| `GET`  | `/admin/contacts`      | JSON list (authenticated)                                              |
| `GET`  | `/admin/rsvps`         | JSON list (authenticated)                                              |
| `GET`  | `/admin/wedding-rsvps` | JSON list (authenticated)                                              |
| `GET`  | `/admin/events`        | JSON list (authenticated)                                              |
| `POST` | `/admin/events`        | Create event (form body; authenticated)                                |

`OPTIONS` on `/api/contact`, `/api/rsvp`, and `/api/wedding-rsvp` returns CORS preflight headers when `CORS_ORIGIN` is set. The same applies to **`/admin/*`** routes when the admin UI is loaded from another origin (credentialed `fetch`).

## Admin UI (static site)

The site is a single **`index.html`** SPA using **path-based** client routing (`/`, `/wedding-rsvp`, `/admin`). Open **`/admin`** (e.g. `http://127.0.0.1:5173/admin` with Vite). The admin view is markdown in `content/admin.md`: **`@table slug [label]`** blocks (see `src/marked-tagged-table.ts`) produce GFM **tables**; **`mount-admin.ts`** collects those tables in **document order**, builds **tabs** from them, and fills each **`<tbody>`** from **`GET /admin/{slug}`** (with tailored row renderers for `wedding-rsvps`, `rsvps`, and `contacts`, and a generic renderer for other list endpoints). A **`?slot?`** provides the login/dashboard shell. **Vite** proxies **`/admin/*`** API traffic to this server in dev, while **`GET /admin`** as an HTML document still loads the SPA (see `vite.config.ts` `proxy.bypass`). Legacy **`/admin.html`** URLs are rewritten to **`/admin`**. Deploy the static build behind a host that **falls back to `index.html`** for unknown paths (same as any History-API SPA). Old **`/#/…`** links are upgraded once to **`/…`** on load.

When the HTML is served from a **different origin** than the API, set **`CORS_ORIGIN`** to that HTML origin and **`VITE_PUBLIC_API_URL`** on the static build to your API URL. With **`APP_ENV=production`**, session cookies use **`SameSite=None`** and **`Secure`** so the browser can send them on cross-site requests to the API host.

Authenticated list endpoints return JSON with **snake_case** field names (`created_at`, `guest_count`, etc.).

## CORS

Set `CORS_ORIGIN` to the **browser origin that serves your HTML** (e.g. `https://www.example.com` or `http://127.0.0.1:5173` for Vite), not the API hostname. Required for public `POST` endpoints and for the **admin page** when it is not same-origin with the API.

## Wedding RSVP and Turnstile

If `TURNSTILE_SECRET_KEY` is unset, wedding RSVPs are accepted without a token (useful for local dev). In production, set it to your Cloudflare Turnstile **secret** and ensure the client sends `turnstile_token` in the JSON body.

## Frontend dev

The Vite app in the repo root proxies **`/api`** and **`/admin`** to this server (see root `vite.config.ts`). For production builds, set **`VITE_PUBLIC_API_URL`** to your API origin so browser requests go to the API host.
