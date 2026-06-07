# Admin

Use the password configured for this site’s API to sign in. You can review user submissions below.

[← Back to home](/)

<!-- `@table slug [tab label]` — slug should match `GET /admin/{slug}` (e.g. `rsvps`, `contacts`). Optional label sets the tab title. -->

@table rsvps RSVPs

| When | Name | Email | Guests | Meals | Notes |
| ---- | ---- | ----- | ------ | ----- | ----- |

@table contacts Contacts

| When | Name | Email | Message |
| ---- | ---- | ----- | ------- |

@table client-errors Frontend errors

| When | Kind | Page | Location | Detail |
| ---- | ---- | ---- | -------- | ------ |

[Dashboard ?slot?](admin-app)
