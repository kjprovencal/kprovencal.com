# Markdown Form DSL

This project includes a `marked` extension that turns specific markdown links into form controls.

The extension lives in `src/marked-form-tokens.ts` and is registered once in `src/main.ts` before rendering markdown. The wedding RSVP form is wired in `src/mount-wedding-rsvp.ts`; the admin dashboard mounts into a `?slot?` from `content/admin.md` via `src/mount-admin.ts`, which **detaches** tagged tables (see below) and injects API rows into each `<tbody>`.

**Static assets (images, PDFs):** Put files in the repo **`public/`** directory (project root). Vite copies them to the build output root, so link as `/wedding-invitation.png`, not `/content/...`. The **`content/`** tree is markdown source only and is **not** deployed as static URLs.

## Tagged tables (`@table`)

`src/marked-tagged-table.ts` registers a **block** extension so you can label any GFM pipe table for stable DOM queries after `marked.parse`.

**Syntax** (blank lines between `@table` and the header row are optional). You can add an **optional tab label** after the slug (same line) for the admin UI; wrap it in quotes if needed:

```markdown
@table my-slug Optional tab title

| Column A | Column B |
| -------- | -------- |
| optional | body row |
```

**Output** (simplified):

- `class="md-tagged-table"`
- `id="md-table-my-slug"` (slug normalized for HTML ids)
- `data-md-table="my-slug"` — use in JS: `document.querySelector(\`table[data-md-table="${slug}"]\`)` or `taggedTableSelector(slug)` from `marked-tagged-table.ts`
- `data-md-tab-label="…"` when a tab label is present (admin dashboard tab text)
- If the markdown has **no body rows**, the renderer still emits an **empty `<tbody></tbody>`** so scripts can append rows.

Slugs must match `^[a-z][a-z0-9_-]*$` (case-insensitive on the `@table` line).

**Admin dashboard:** `src/mount-admin.ts` collects **every** `md-tagged-table` under `#content` in **document order**, builds one tab per table, and loads `GET /admin/{slug}` (after a small alias map, e.g. `wedding-rsvp` → `wedding-rsvps`). Built-in formatters exist for `wedding-rsvps`, `rsvps`, and `contacts`; any other slug still calls the same URL shape and renders a **generic** JSON array table (handy for new list endpoints). Match your header columns to the API fields you care about, or rely on the generic columns derived from the first row’s keys.

## Core Pattern

Use a markdown link where the link text contains a control spec:

`[Label ?type?](name "attr=value ...")`

Required fields use `*`:

`[Email ?email?*](email "...")`

You can also place the `?type?` first:

`[?email?* Email](email "...")`

## Built-in Types

- `form` -> opens `<form ...>`
- `form-end` -> closes `</form>`
- `text`, `email`, `number`, `hidden` -> `<input type="...">` plus label
- `textarea` -> `<textarea ...></textarea>` plus label
- `select` -> `<select ...>` plus generated `<option>` list
- `submit` -> `<button type="submit">...</button>`
- `status` -> empty `<p ...></p>` (for live status messages)
- `slot` -> optional hint text plus empty `<div ...></div>` (for JS-rendered content)
- `turnstile` -> empty `<div ...></div>` host for [Cloudflare Turnstile](https://developers.cloudflare.com/turnstile/) (wired in page JS; set `VITE_TURNSTILE_SITE_KEY` for production)

## Link Parts

### 1) Label + type

- Label text becomes the rendered label (or submit button text)
- Type chooses the control renderer

Examples:

- `[Full name ?text?*](name "...")`
- `[Submit RSVP ?submit?](submit "...")`
- `[End form ?form-end?](-)`

### 2) `href` as the field name

- `href` is used as the default `name` (and for `form`, can be default `id`)
- Use `-` when you do not want a name/id derived from href (e.g. `form-end`)

### 3) Title attributes

The optional title string is parsed into HTML attributes:

- Boolean: `required`, `disabled`, `novalidate`
- Key/value: `id=foo`, `class=my-class`, `rows=4`
- Quoted values allowed: `placeholder="Type your message"`

Examples:

- `"id=rsvp-email class=rsvp-form__input autocomplete=email"`
- `"class=rsvp-form__submit disabled"`

## Select Options

`select` reads options from a title attribute named `options`.

Format:

`options="value:Label|value2:Label 2|..."`.

Example:

`[Guest count ?select?*](guest_count "id=rsvp-guest-count options=\":Choose...|1:1|2:2|3:3\"")`

Notes:

- First option with an empty value is treated as placeholder and rendered `selected disabled`.
- If no `:` is provided, value and label are the same token.

## Slot Blocks

Use `slot` for content your page script fills dynamically.

Example:

`[Meal choices ?slot?](rsvp-meal-rows "class=rsvp-form__meal-rows hint=\"Choose one entree per guest.\"")`

This renders:

- Optional hint paragraph
- Empty div target for JS (`id="rsvp-meal-rows"`)

## Turnstile host

Use `turnstile` for a labeled container your script renders into with explicit Turnstile `render()` (see `src/mount-wedding-rsvp.ts`). Default classes include `rsvp-form__turnstile-host`; set `id` if you need a fixed selector (e.g. `id=cf-turnstile`).

## RSVP Example Snippet

```md
[RSVP form ?form?](wedding-rsvp-form "class=rsvp-form novalidate")
[Full name ?text?*](name "id=rsvp-name class=rsvp-form__input labelClass=rsvp-form__label")
[Email ?email?*](email "id=rsvp-email class=rsvp-form__input labelClass=rsvp-form__label")
[Number of attendees in your party ?select?*](guest_count "id=rsvp-guest-count class=rsvp-form__select labelClass=rsvp-form__label options=\":Choose...|0:0 (not attending)|1:1|2:2\"")
[Meal choices ?slot?](rsvp-meal-rows "class=rsvp-form__meal-rows hint=\"Choose one entree per guest.\"")
[Allergies, dietary restrictions, or other notes ?textarea?](notes "id=rsvp-notes class=rsvp-form__textarea labelClass=rsvp-form__label rows=4")
[Security check ?turnstile?](- "id=cf-turnstile labelClass=rsvp-form__label")
[Status ?status?](rsvp-form-status "class=rsvp-form__status role=status aria-live=polite")
[Submit RSVP ?submit?](submit "class=rsvp-form__submit disabled")
[End form ?form-end?](-)
```

## Current Limitations

- No nested group syntax yet (fieldset wrappers are not tokenized in link DSL yet).
- Attribute parser is intentionally simple and whitespace-based.
- Unknown `?type?` falls back to `text` input behavior.

If you add new types, update both `src/marked-form-tokens.ts` and this doc.
