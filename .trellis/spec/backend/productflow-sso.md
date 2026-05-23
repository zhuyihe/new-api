# ProductFlow SSO Contract

## Scenario: New API-backed ProductFlow SSO

### 1. Scope / Trigger

- Trigger: New API exposes a server-side SSO bridge for ProductFlow and provisions per-user ProductFlow API tokens.
- Scope: `GET /api/productflow/sso/start`, `POST /api/productflow/sso/verify`, ProductFlow token create-or-reuse behavior, and sidebar entry wiring.
- Out of scope: relay billing, provider channel selection, quota settlement, and ProductFlow tenant storage.

### 2. Signatures

- `GET /api/productflow/sso/start`
  - Browser endpoint.
  - Requires a valid New API browser session.
  - Redirects to ProductFlow callback with a one-time ticket.
- `POST /api/productflow/sso/verify`
  - Server-to-server endpoint.
  - Requires `Authorization: Bearer <PRODUCTFLOW_SSO_SECRET>`.
  - Body: `{"ticket":"<one-time-ticket>"}`.

### 3. Contracts

Database-backed option keys:

- `productflow_sso.base_url`: required for SSO start, for example `https://image.aync.cc.cd`.
- `productflow_sso.shared_secret`: required for verify.
- `productflow_sso.token_name`: optional, defaults to `ProductFlow`.
- `productflow_sso.token_model_limits`: optional comma-separated model whitelist; whitespace is trimmed.
- `productflow_sso.token_group`: optional token group.
- `productflow_sso.ticket_ttl_seconds`: optional, defaults to `60`.
- `productflow_sso.session_ttl_seconds`: optional, defaults to `1209600`.

The ProductFlow SSO bridge must read these values from New API's option store (`common.OptionMap` backed by the options
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
- `expires_in`

Security contract:

- Browser redirects carry only `ticket`.
- Token material is returned only from `POST /api/productflow/sso/verify`.
- Tickets are short-lived and single-use.
- Redis is preferred for ticket storage when enabled; in-memory ticket storage is an allowed single-process fallback.
- Redis ticket payloads contain token material; do not use helper wrappers that log values in debug mode.

### 4. Validation & Error Matrix

- Missing ProductFlow base URL on start -> `503`.
- Invalid ProductFlow base URL on start -> `503`.
- Missing ProductFlow shared secret on start or verify -> `503`.
- No valid browser session on start -> `302` to `/sign-in?redirect=%2Fapi%2Fproductflow%2Fsso%2Fstart`.
- Disabled user on start -> `403`.
- Missing or wrong verify secret -> `401`.
- Invalid JSON verify body -> `400`.
- Missing, expired, or already consumed ticket -> `401`.

### 5. Good/Base/Bad Cases

- Good: logged-in user clicks sidebar Image, New API redirects to ProductFlow callback, ProductFlow verifies ticket once, and stores the returned token server-side.
- Base: user is not logged in, New API redirects to sign-in, then a browser-level redirect returns to `/api/productflow/sso/start`.
- Bad: ProductFlow retries the same ticket or sends a wrong shared secret; New API rejects the request.

### 6. Tests Required

- Secret validation rejects missing/wrong bearer secret.
- Stored ticket verifies once and fails on the second attempt.
- Start redirects unauthenticated browser sessions to sign-in with preserved redirect.
- Start with a valid browser session creates or reuses the configured ProductFlow token.
- Redirect URL does not contain `sk-` token material.

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

### 8. Deployment Modes

| Deployment | Redis enabled | Supported |
|---|---:|---|
| Single process | Yes | Yes |
| Single process | No | Yes, with in-memory ticket fallback |
| Multi process or multi instance | Yes | Yes |
| Multi process or multi instance | No | No, SSO ticket reuse will fail across processes |

When `productflow_sso` is configured and Redis is disabled, new-api logs a
startup `WARN` because ticket storage falls back to process-local memory.
Multi-process deployments must enable Redis for ProductFlow SSO to remain
reliable.
