# Atelier SSO Contract (ProductFlow-Compatible Internals)

## Scenario: New API-backed Atelier SSO

### 1. Scope / Trigger

- Trigger: New API exposes a server-side SSO bridge for Atelier and provisions per-user Atelier API tokens.
- Scope: `GET /api/productflow/sso/start`, `POST /api/productflow/sso/verify`, Atelier token create-or-reuse behavior, selected SSO image model propagation, and sidebar entry wiring.
- Out of scope: relay billing, provider channel selection, quota settlement, and Atelier tenant storage.
- Compatibility note: the visible brand is Atelier, but existing API paths and option keys keep the `productflow` /
  `productflow_sso` prefix until a separate compatibility migration is approved.

### 2. Signatures

- `GET /api/productflow/sso/start`
  - Browser endpoint.
  - Requires a valid New API browser session.
  - Redirects to ProductFlow callback with a one-time ticket.
- `POST /api/productflow/sso/verify`
  - Server-to-server endpoint.
  - Requires `Authorization: Bearer <PRODUCTFLOW_SSO_SECRET>`.
  - Body: `{"ticket":"<one-time-ticket>"}`.
- `GET /api/productflow/sso/status`
  - Admin endpoint guarded by `RootAuth()`.
  - Returns `{ enabled, configured, redis_enabled, callback_url_preview, configuration_message, configuration_issues, last_test_result }`.
  - `callback_url_preview` must use the same URL-resolution semantics as the
    browser redirect path (`redirectProductFlowUser` / `common.BuildURL`),
    not string concatenation. Base URLs with extra path segments must still
    preview the canonical `/auth/new-api/callback` target.
  - `configuration_message` and `configuration_issues` are safe,
    admin-actionable summaries of missing/invalid SSO settings. They must not
    include secret values, token material, relay request bodies, prompts, or
    stack traces.
  - `last_test_result` is the persisted probe outcome that the status card
    renders across reloads.
  - Used by the system settings status card; no caller-supplied parameters.
- `POST /api/productflow/sso/test`
  - Admin endpoint guarded by `RootAuth()`.
  - Optional body `{ base_url?: string }`; the saved value is used when omitted.
  - Probes `<base_url>/api/health/sso` with a 3s timeout and persists the
    outcome to `productflow_sso.last_test_result` for the status endpoint to
    return.
- `PUT /api/option/batch`
  - Admin endpoint guarded by `RootAuth()`.
  - Body `{ updates: [{ key, value }, ...] }`. Each entry is validated
    individually through `validateProductFlowOptionValue` before any DB writes.
  - All updates and the corresponding `LogTypeManage` audit row commit
    inside a single GORM transaction (`UpdateOptionsBatchAtomic`). Sensitive
    keys (`*Secret`, `*Key`, `*Token`, `*api_key`) are masked as
    `***<sha8>` in the audit `changes[]` payload.
- `GET /api/health/sso` (ProductFlow side, public)
  - Returns `{ ok, version, supports_sso }` so new-api's Test Connection
    button can classify the bridge as connected / network_error /
    application_error.
  - Rate-limited via slowapi at 6 requests per minute per client IP to
    deter scanning.

### 3. Contracts

Database-backed option keys:

- `productflow_sso.enabled`: optional, defaults to `true`. When `false`,
  `GET /api/productflow/sso/start` returns `503` with `"Atelier SSO is
  disabled"` and the startup probe logs a single `INFO` line so operators
  can tell the toggle apart from a misconfiguration.
- `productflow_sso.base_url`: required for SSO start, for example `https://image.aync.cc.cd`.
- `productflow_sso.shared_secret`: required for verify.
- `productflow_sso.token_name`: optional, defaults to `Atelier`.
- `productflow_sso.token_group`: optional token group.
- `productflow_sso.image_model`: optional selected image-generation model for Atelier SSO. When a token group is
  configured and New API can resolve image-generation models for that group, this value must be one of those enabled
  models. Atelier uses this model for hosted SSO image generation instead of an independently edited local image model.
- `productflow_sso.ticket_ttl_seconds`: optional, defaults to `60`.
- `productflow_sso.session_ttl_seconds`: optional, defaults to `1209600`.
- `productflow_sso.admin_session_ttl_seconds`: optional, defaults to `3600`.
- `productflow_sso.last_test_result`: managed by `POST /api/productflow/sso/test`;
  not editable from the settings form.

The Atelier SSO bridge must read these values from New API's option store (`common.OptionMap` backed by the options
table). Environment variables are not a fallback for this bridge, so stale deployment env cannot silently change SSO
behavior.

Verify response `data` fields:

- `user_id`
- `username`
- `email`
- `group`
- `role`
- `token`
- `token_id`
- `token_name`
- `token_group`
- `image_model`
- `expires_in`

`role` is serialized as the decimal string form of the New API user role.
`expires_in` uses `session_ttl_seconds` for ordinary users and
`admin_session_ttl_seconds` for admin/root users.
`group` remains the user's New API user group. `token_group` is the group assigned to the generated Atelier token.
`image_model` is the New API-selected image-generation model that Atelier should use for hosted SSO image calls.

Security contract:

- Browser redirects carry only `ticket`.
- Token material is returned only from `POST /api/productflow/sso/verify`.
- Tickets are short-lived and single-use.
- Redis is preferred for ticket storage when enabled; in-memory ticket storage is an allowed single-process fallback.
- Redis ticket payloads contain token material; do not use helper wrappers that log values in debug mode.

### 4. Validation & Error Matrix

- No valid browser session on start -> `302` to `/sign-in?redirect=%2Fapi%2Fproductflow%2Fsso%2Fstart`.
- Start validates the browser session before configuration so public requests
  cannot probe whether SSO is disabled or misconfigured.
- Disabled (`productflow_sso.enabled=false`) on start -> `503` with
  `"Atelier SSO is disabled"` for JSON clients and a browser-friendly HTML
  page for HTML clients.
- Missing Atelier base URL on start -> `503`; browser clients receive a
  friendly HTML page instead of raw JSON.
- Invalid Atelier base URL on start -> `503`; browser clients receive a
  friendly HTML page instead of raw JSON.
- Missing Atelier shared secret on start -> `503`; browser clients receive a
  friendly HTML page instead of raw JSON.
- Missing Atelier shared secret on verify -> `503`.
- Disabled user on start -> `403`.
- Missing or wrong verify secret -> `401`.
- Invalid JSON verify body -> `400`.
- Missing, expired, or already consumed ticket -> `401`.
- Batch save with one invalid entry -> `400`; no DB writes occur and the
  failing key is returned in `failed_keys`.
- Batch save transaction failure -> `500` with a generic message; the raw
  cause is logged via `common.SysError` (never returned to the client).
- Admin/root role tickets use `admin_session_ttl_seconds`; ordinary user
  tickets use `session_ttl_seconds`.
- A configured `productflow_sso.image_model` that is not enabled for
  `productflow_sso.token_group` -> validation error before saving or starting a
  known-broken SSO flow.
- Test connection transport failure -> `200` with a `network_error`
  category whose `message` is a fixed string (no raw exception text).
- Status callback preview with a base URL path segment -> canonical callback
  root, not a concatenated nested path.
- Status endpoint must round-trip `last_test_result` from OptionMap so the UI
  can show the latest test after reload.
- Status endpoint must report safe `configuration_issues` so the settings UI
  can tell the admin exactly which SSO field needs attention.

### 5. Good/Base/Bad Cases

- Good: logged-in user clicks sidebar Image, New API redirects to the Atelier callback, Atelier verifies the ticket once, and stores the returned token server-side.
- Base: user is not logged in, New API redirects to sign-in, then a browser-level redirect returns to `/api/productflow/sso/start`.
- Bad: ProductFlow retries the same ticket or sends a wrong shared secret; New API rejects the request.

### 6. Tests Required

- Secret validation rejects missing/wrong bearer secret.
- Stored ticket verifies once and fails on the second attempt.
- Start redirects unauthenticated browser sessions to sign-in with preserved redirect.
- Start redirects unauthenticated browser sessions before SSO configuration
  validation, even when the saved config is broken.
- Start returns a browser-friendly HTML error page for authenticated browser
  users when SSO configuration blocks the redirect.
- Start with a valid browser session creates or reuses the configured Atelier token.
- Verify response includes `token_group` and `image_model` when configured, without overloading the existing `group`
  field.
- Config validation rejects a selected image model that is not enabled for the selected token group.
- Redirect URL does not contain `sk-` token material.
- `productflow_sso.enabled=false` returns the disabled `503` body and
  emits the disabled INFO when other settings are populated.
- Batch endpoint commits all updates atomically and writes one
  `LogTypeManage` row whose `Other.admin_info.changes[]` masks secrets.
- Test endpoint classifies happy / network_error / application_error
  responses correctly and persists `last_test_result` for the status
  endpoint to surface.
- Status endpoint reports safe configuration issues for missing base URL,
  missing shared secret, unavailable token group image models, and stale image
  model selections.
- Admin/root role tickets use the admin TTL while ordinary user tickets
  keep the normal session TTL.
- ProductFlow `/api/health/sso` returns the documented schema and the
  7th request from the same client in a minute is rate-limited (`429`).

### 7. Wrong vs Correct

#### Wrong

```go
// Browser receives a token-bearing URL.
c.Redirect(http.StatusFound, productFlowURL+"?token=sk-...")
```

```go
// Debug logging can leak the ticket payload because it contains token material.
common.RedisSet(ticketKey, string(payload), ttl)
```

#### Correct

```go
// Browser receives only a one-time ticket; ProductFlow verifies it server-side.
c.Redirect(http.StatusFound, callbackURLWithTicket)
```

```go
// Store the ticket without routing token material through value-logging helpers.
common.RDB.Set(context.Background(), ticketKey, string(payload), ttl).Err()
```

#### Wrong

```go
// String concatenation drifts when the base URL has extra path segments.
callbackPreview := strings.TrimRight(cfg.BaseURL, "/") + "/auth/new-api/callback"
```

#### Correct

```go
callbackURL, _ := buildProductFlowCallbackBaseURL(cfg.BaseURL)
callbackPreview := callbackURL.String()
```

### 8. Deployment Modes

| Deployment | Redis enabled | Supported |
|---|---:|---|
| Single process | Yes | Yes |
| Single process | No | Yes, with in-memory ticket fallback |
| Multi process or multi instance | Yes | Yes |
| Multi process or multi instance | No | No, SSO ticket reuse will fail across processes |

When `productflow_sso` is configured and Redis is disabled, new-api logs a
startup `WARN` because ticket storage falls back to process-local memory.
Multi-process deployments must enable Redis for Atelier SSO to remain
reliable.
