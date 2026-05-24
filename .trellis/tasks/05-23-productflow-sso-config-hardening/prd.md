---
task: productflow-sso-config-hardening
parent_task: productflow-sso
design_session: 2026-05-23 grill-me
related_productflow_task: null
cross_repo_files:
  - ProductFlow/backend/src/productflow_backend/presentation/routes/health.py
scope_tier: B
estimated_days: 4
cross_repo: true
---

# ProductFlow SSO Config Page Hardening

## Goal

Transform the new-api `/system-settings/operations/productflow-sso` configuration
page from a barely-usable form into a production-grade admin surface that
matches the operational reality of a public multi-tenant deployment to
`api.aync.cc.cd` ↔ `image.aync.cc.cd`. Specifically:

1. Fix the silent shared-secret clearing bug that breaks SSO on any partial
   save (P0).
2. Make the multi-language admin experience real (today the page is English-only
   in 5 of 6 supported locales).
3. Add the missing operational primitives: enable/disable toggle, top status
   summary, test-connection probe, callback URL preview, TTL human-readable
   helper, secret strength meter, secret-modification confirmation dialog,
   base_url tri-level safety warning, and atomic batch save.
4. Capture the missing audit trail: configuration changes, SSO start success,
   and all SSO failures must land in the existing `usage-logs` UI so operators
   can answer "who, when, what changed, and what happened" without log-diving.
5. Extend the existing manage-type detail dialog with a generic
   `adminInfo.changes[]` renderer so this page (and future settings sections)
   can show before/after diffs without per-page custom rendering.

## Why Now

`image.aync.cc.cd` is shipping as a public multi-tenant workspace this wave.
The current ProductFlow SSO config page was acceptable for a private
deployment audited only by its author, but is unfit for a configuration that:

- Holds the shared secret authorizing all server-to-server ProductFlow ticket
  verification (a leak or accidental clear = total auth bypass on one side and
  total auth-failure on the other).
- Cannot be temporarily disabled without clearing the base_url field (so any
  ProductFlow maintenance window forces a config rewrite, increasing the
  probability of typos and missed restorations).
- Has zero observability into the configuration change history.
- Is shown in English to every Chinese, French, Japanese, Russian, and
  Vietnamese admin (the page calls `t()` 23 times but ships 0 translations).

The shared-secret clearing bug specifically (Q5 in the decision matrix below)
is reachable by editing any other field and clicking Save - the placeholder
text claims `Leave blank to keep the existing secret`, but the implementation
overwrites it with empty string. Until this is fixed, the page is one stray
Save away from breaking SSO for the entire user base.

## Decision Matrix (from 2026-05-23 grill-me)

The full Q&A is in `research/grill-me-2026-05-23.md`. This table is the
canonical reference for "what was decided and why" so reviewers can challenge
specific calls without re-running the session.

| # | Decision Point | Resolution | Why |
|---|----------------|------------|-----|
| 1 | Scope tier | **B mid-scope** (3-4 days) | A too narrow to justify session overhead; C requires ProductFlow co-changes outside this wave |
| 2 | Enable/disable mechanism | New `productflow_sso.enabled` option, default `true` | UI toggle maps 1:1 to a real field; preserves config during maintenance windows; backward compatible |
| 3 | Status code when disabled | **503** Service Unavailable | Reuses ProductFlow's existing `error_sso_unavailable` variant; aligns with current "config missing" path |
| 4 | Startup WARN when disabled | **No WARN** + new INFO `productflow_sso disabled` line | A disabled SSO has zero single-process Redis fallback risk; WARN would be noise |
| 5 | Shared-secret blank bug | Front-end filters empty secret from `changedKeys` in `onSubmit` | Backend update endpoint stays generic; placeholder text already correct |
| 6 | Save atomicity | New `PUT /api/option/batch` endpoint with GORM transaction | Eliminates partial-failure inconsistency and 6-toast spam; establishes pattern for other settings sections |
| 7 | Test-connection probe | ProductFlow exposes `GET /api/health/sso`; new-api server-side fetches via `POST /api/productflow/sso/test` | Covers 90% of misconfigurations (DNS, TLS, ProductFlow down, reverse proxy); secret-mismatch deferred to C-tier |
| 8 | Callback URL display | Front-end hardcoded constant `PRODUCTFLOW_SSO_CALLBACK_PATH = '/auth/new-api/callback'` + Copy button | Callback path is a two-side protocol contract, not an admin choice |
| 9 | Top-of-page layout | Top status card (toggle + status chip + callback URL + Redis warning) above the form | Single-glance "what state is SSO in" answer; Redis WARN moves from startup log to always-visible |
| 10 | TTL field unit | Seconds kept; `FormDescription` adds real-time human conversion `≈ 14 days` | Minimal schema change; works for all values (60s, 90s, 1209600s) |
| 11 | i18n strategy | zh + en hand-translated (48 strings); `bun run i18n:sync` propagates key structure to fr/ja/ru/vi (English fallback values) | en serves as i18next fallback authority; minority locales structurally ready for future community translation |
| 12 | Task structure | Single task with Phase 0-5 internal ordering | Same files touched throughout; double-task would deadlock |
| 13 | Disabled-state field editability | Fields stay editable + amber banner `Saved configuration takes effect on next enable` | Supports "configure then enable" workflow; banner prevents confusion |
| 14 | Status-card refresh | Initial fetch + invalidate on save + manual refresh button (no polling) | All status changes are admin-action-triggered; polling wastes requests |
| 15 | Test-connection error granularity + timeout | Three-tier classification (`network` / `application` / `other`) + 3s fixed timeout | Enough diagnosis fidelity; ProductFlow health probe should return in <100ms |
| 16 | Secret strength verification | Real-time strength bar (weak/medium/strong) + zod `.min(16)` with `[A-Za-z]` and `[0-9]` enforced | Double-layer defense; only shown when field non-empty (preserves Q5 blank-skip semantics) |
| 17 | Logging scope this wave | b-tier: test-connection sys log + config-change audit + SSO start/verify ticket_hash correlation | Larger ProductFlow-side trace columns deferred; new-api side fully covered |
| 18 | Detail-dialog manage rendering | Generic `adminInfo.changes[]` table in `isManage` branch | Reusable for all settings sections; protocol-driven not page-driven |
| 19 | SSO start/verify DB writes | Tier D: `start success` + all failures go to `model.Log` (`LogTypeSystem`); `verify success` stays sys log only | Balances compliance traceability with logs-table growth (~54MB/year est.) |
| 20 | Toggle commit timing | Toggle joins form dirty state; only commits via Save | Prevents accidental production SSO down/up; consistent with batch atomicity |
| 21 | Test-connection URL source | Form draft value (not saved value) with `draft` chip label | Supports edit-then-test workflow; probe is read-only so draft is safe |
| 22 | Audit write timing | Inside the same GORM transaction as the OptionMap updates | Compliance-first: audit failure rolls back the business change |
| 23 | base_url validation | Three tri-level warnings (http / loopback / private IP), all non-blocking | Catches dev/internal-network misconfigurations without forbidding legitimate setups |
| 24 | Secret-change confirmation | `AlertDialog` on Save if `shared_secret` field is dirty | Distinct blast radius vs other fields warrants distinct friction |
| 25 | ProductFlow health probe schema + rate limit | `{ok, version, supports_sso}` + slowapi `6/minute` per IP | Enough for diagnosis without leaking deployment internals; rate limit deters scanning |

## Requirements

### R1 - P0: shared_secret blank-keep correctness

When the admin submits the form without modifying the secret field (value is
empty string after trim), the secret MUST NOT be written. The current
front-end filter must include this clause:

```ts
.filter((key) => !(key === 'productflow_sso.shared_secret' && normalized[key] === ''))
```

A unit test asserts that submitting with `shared_secret = ''` plus another
modified field produces a single batch request that omits `shared_secret`.

### R2 - i18n parity

- `zh.json` and `en.json` each gain 24 keys covering every `t()` invocation in
  the section + the new components (toggle banner, status chips, warnings,
  test-connection labels, strength meter, confirmation dialog).
- `bun run i18n:sync` runs clean; the four minority locales (`fr / ja / ru /
  vi`) gain the same key structure with English values as a fallback.
- No `t()` call in the touched components resolves to its raw key string in
  any locale.

### R3 - SSO enable/disable toggle

- New option `productflow_sso.enabled` with default `true`. Persisted as
  string `"true"` / `"false"` in `OptionMap`, parsed by Go bool helper.
- `validateForStart()` returns `errSSODisabled` (sentinel error) as its first
  check when `!cfg.Enabled`.
- `WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup` already calls
  `validateForStart()`, so disabled deploys are silent (Q4).
- Add `common.SysLog("ProductFlow SSO disabled (productflow_sso.enabled=false)")`
  to startup when disabled, so operators can confirm intentional state.
- `StartProductFlowSSO` returns `503` with body `{"success":false,"message":"ProductFlow SSO is disabled"}`
  when disabled (Q3).

### R4 - Status endpoint + top card

- New `GET /api/productflow/sso/status` (RootAuth) returning:
  ```json
  {
    "enabled": bool,
    "configured": bool,        // base_url && shared_secret both present and valid
    "redis_enabled": bool,
    "callback_url_preview": string,  // computed from current base_url + canonical path
    "last_test_result": null | {
      "ok": bool,
      "category": "connected" | "network_error" | "application_error" | "other",
      "latency_ms": int,
      "tested_at": int64,
      "tested_against": "draft" | "saved",
      "message": string         // brief diagnostic, never raw exception text
    }
  }
  ```
- Front-end status card renders these signals as chips, a banner for the
  Redis warning, and the callback URL preview with copy button.
- Card refreshes on mount, after successful batch save (via
  `invalidateQueries`), and on manual refresh button (Q14).

### R5 - Batch save endpoint with in-transaction audit

- New `PUT /api/option/batch` (RootAuth) accepting:
  ```json
  { "updates": [{ "key": string, "value": string|number|bool }, ...] }
  ```
- Validates every key against existing `validateProductFlowOptionValue` (and
  any other per-key validators reused from the single-key endpoint).
- Single GORM transaction: updates `OptionMap` rows then creates one
  `model.Log` row with `LogTypeManage`, `Content = "ProductFlow SSO config
  updated"`, and `Other.admin_info = { admin_id, admin_username, changes: [
  { key, before, after } ] }`.
- `before` and `after` for keys matching `*secret`, `*key`, `*token` are
  masked as `***` + first 8 hex chars of sha256 of the value. Other keys
  store raw values.
- If any single update fails validation: rollback, return 400 with
  `{ "success": false, "failed_keys": ["productflow_sso.base_url"], "message": "..."}`.
- `onSuccess` invalidates both `system-options` and
  `productflow-sso-status` query keys, fires exactly one success toast.

### R6 - Test-connection endpoint

- New `POST /api/productflow/sso/test` (RootAuth) accepting:
  ```json
  { "base_url": "https://..." }   // optional, falls back to OptionMap value
  ```
- Server uses `http.Client{Timeout: 3 * time.Second}` to fetch
  `<base_url>/api/health/sso`. Classifies result into one of `connected`,
  `network_error`, `application_error`, `other` and returns:
  ```json
  {
    "ok": bool,
    "category": "...",
    "latency_ms": int,
    "tested_against": "draft" | "saved",
    "message": "..."
  }
  ```
- Test result is persisted to OptionMap under `productflow_sso.last_test_result`
  for the status card to read across reloads. Test result is itself audited
  with a sys log line, not a manage-log (Q17).

### R7 - ProductFlow health probe

- ProductFlow adds `GET /api/health/sso` returning `{"ok": bool, "version":
  str, "supports_sso": bool}` where `supports_sso = bool(cfg.shared_secret)`.
- No authentication required.
- Rate-limited via `slowapi` to `6/minute` per remote IP (Q25).
- This endpoint becomes a locked contract: the schema may add fields in
  later waves but cannot remove or rename existing ones.

### R8 - Callback URL preview

- Front-end exports `PRODUCTFLOW_SSO_CALLBACK_PATH = '/auth/new-api/callback'`
  constant in `web/default/src/features/auth/lib/productflow-sso.ts`.
- `buildProductFlowCallbackURL(baseUrl)` helper applies
  `removeTrailingSlash`.
- Status card shows the computed URL with a Copy button; updates in real time
  as the admin edits the base_url field (form watch).

### R9 - TTL human-readable conversion

- `formatSecondsHuman(s, t)` helper returns localized strings like
  `≈ 14 days` / `≈ 23 hours 15 minutes` / `≈ 60 seconds`.
- Rendered inside `FormDescription` for both `ticket_ttl_seconds` and
  `session_ttl_seconds`.
- Schema additionally warns (not blocks) when:
  - `session_ttl_seconds > 30 days` (`Long session lifetimes increase abuse risk`)
  - `ticket_ttl_seconds < 10` (`Ticket TTL too short to complete SSO redirect`)

### R10 - Secret strength meter + change confirmation

- Strength meter shown only when the field is non-empty (preserves Q5
  blank-skip semantics):
  - Weak (red): `< 16` chars, or only letters, or only digits
  - Medium (amber): `>= 16` chars + letters + digits + `< 24` chars
  - Strong (green): `>= 24` chars + letters + digits + at least one symbol
- Schema enforces `value === '' || (value.length >= 16 && /[A-Za-z]/.test(value)
  && /[0-9]/.test(value))`.
- On Save, if `shared_secret` field is dirty, an `AlertDialog` confirms with
  the explicit consequence text:
  ```
  Modify shared secret?
  This will immediately invalidate all current ProductFlow sessions.
  All logged-in users will need to re-authorize through New API.
  ```

### R11 - base_url tri-level warnings (non-blocking)

- `classifyBaseUrl(url)` returns warnings for:
  - `protocol === 'http:'` -> "Recommend HTTPS for production"
  - host in `{localhost, 127.0.0.1, 0.0.0.0}` -> "Loopback detected, OK for dev, not for production"
  - host matches RFC1918 private IP ranges -> "Private IP detected, browser may not reach across origins"
- Warnings render in amber `FormDescription` block beneath the existing
  description text; Save button stays enabled.

### R12 - Disabled-state UX

- When `enabled === false`, top card chip turns gray and shows
  "Disabled - saved configuration takes effect on next enable".
- All form fields remain editable.
- Save button still works; on Save with `enabled === true` going false the
  AlertDialog is NOT shown (only secret-change triggers Q24's dialog).
- `beforeunload` browser dialog if `form.formState.isDirty` (catch
  accidental tab close mid-edit).

### R13 - SSO lifecycle audit

In `controller/productflow_sso.go` `StartProductFlowSSO`:

```go
// success branch
model.RecordLog(user.Id, model.LogTypeSystem,
    fmt.Sprintf("productflow_sso start: user=%d ticket=%s callback=%s",
        user.Id, ticketHashHex16, callbackURLForLog))

// failure branches: disabled / not configured / user disabled / token error
model.RecordLog(userId, model.LogTypeSystem,
    fmt.Sprintf("productflow_sso start failed: reason=%s", reason))
```

In `VerifyProductFlowSSO`:

```go
// failure only (success path is high-frequency and already logged by start)
model.RecordLog(0, model.LogTypeSystem,
    fmt.Sprintf("productflow_sso verify failed: ticket=%s reason=%s",
        ticketHashHex16, reason))
```

Where `ticketHashHex16 = hex.EncodeToString(sha256.Sum256([]byte(ticket)))[:16]`.

### R14 - Detail dialog generic manage diff

In `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`,
under the `isManage` branch, render a new `<ConfigChangesTable />` if
`adminInfo.changes` is a non-empty array:

```tsx
<div className="rounded-md border p-3">
  <Label className="text-xs font-medium">
    {t('Configuration changes')}
  </Label>
  <div className="mt-2 grid grid-cols-[1fr_auto_1fr] gap-x-3 gap-y-1 text-xs">
    {changes.map((c) => (
      <Fragment key={c.key}>
        <div className="font-mono">{c.key}</div>
        <div className="text-muted-foreground">{c.before} -> {c.after}</div>
      </Fragment>
    ))}
  </div>
</div>
```

Extend `web/default/src/features/usage-logs/types.ts` `admin_info` shape
(currently at lines 109-111) to add the typed landing for the new array,
otherwise the generic renderer has no type anchor:

```ts
// Manage audit fields (type=3, admin only)
admin_username?: string
admin_id?: number | string
changes?: Array<{ key: string; before: string; after: string }>
```

i18n: 1 new key `Configuration changes`.

### R15 - Spec updates

`.trellis/spec/backend/productflow-sso.md` gains:

- A new "Configuration Surface" section enumerating the seven option keys,
  their types, defaults, and validation rules.
- A "Two-side Contracts" section locking:
  - Callback path: `/auth/new-api/callback` (ProductFlow side, immutable)
  - Health probe: `GET /api/health/sso`, schema `{ok, version, supports_sso}`,
    rate limit `6/minute`
- An "Audit Logging" section documenting the audit log types, ticket_hash
  format, and the `adminInfo.changes[]` protocol.
- Updates to the existing "Deployment Modes" table reflecting the new
  `productflow_sso.enabled` flag.

## Acceptance Criteria

- [ ] R1 unit test: submitting the form without secret change + another field
      change produces exactly one batch request omitting `shared_secret`.
- [ ] R2: running `bun run i18n:sync` from `web/default/` produces zero
      missing keys for `zh` and `en`; `_reports/` shows `fr / ja / ru / vi`
      missing keys are filled with English fallback values.
- [ ] R3: starting new-api with `productflow_sso.enabled=false` and otherwise
      valid SSO config emits no startup WARN and one INFO line; the
      `StartProductFlowSSO` endpoint returns 503; the toggle on the UI shows
      off.
- [ ] R4: `GET /api/productflow/sso/status` returns the documented shape and
      is reflected in the top status card; manual refresh button updates
      `last_test_result` if a test was just run from another tab.
- [ ] R5: a batch save with 5 valid changes commits all 5 atomically and
      writes exactly one `model.Log` row with the changes array; a batch save
      with one invalid change rolls everything back and returns the failed key
      list.
- [ ] R6: pressing Test Connection from the UI with a draft URL probes that
      URL (not the saved one) and renders the result chip with classification
      and latency; the result persists on page reload.
- [ ] R7: ProductFlow's `GET /api/health/sso` returns the documented schema;
      the seventh request from the same IP within a minute returns 429.
- [ ] R8: the callback URL preview updates in real time as the admin types in
      base_url; the Copy button copies the full URL to clipboard.
- [ ] R9: editing `session_ttl_seconds` from 60 to 1209600 shows
      `≈ 14 days`; values over 30 days show a warning under the helper.
- [ ] R10: typing a 10-char secret shows a red Weak chip; typing a 24-char
      mixed-character secret shows a green Strong chip; clicking Save with
      a dirty secret field opens the AlertDialog before submission.
- [ ] R11: typing `http://localhost:8080` shows two amber warnings (http +
      loopback); typing `https://image.aync.cc.cd` shows none.
- [ ] R12: toggling enabled off and editing fields shows the amber banner;
      navigating away with unsaved changes triggers the browser's
      beforeunload dialog.
- [ ] R13: a successful SSO start from a test account appears in
      `/usage-logs/common?type=4` with the ticket_hash; a forced failure
      appears with `start failed: reason=...`.
- [ ] R14: the manage-type detail dialog for the audit row written by R5
      renders the changes table with masked secret diff and raw other fields.
- [ ] R15: `.trellis/spec/backend/productflow-sso.md` contains all five new
      sections; the section IDs are reachable via the same anchors used in the
      WARN log line and any other cross-references.

## Out of Scope

The following items were explicitly grilled and deferred to a C-tier or
follow-on wave:

- ProductFlow-side endpoint to list active sessions or force-revoke all
  sessions (would let admins kill all sessions after a secret rotation
  without waiting for the 14-day TTL).
- ProductFlow `audit_logs` table gaining a `ticket_hash` column for full
  end-to-end SSO trace correlation (currently only new-api side carries
  ticket_hash).
- new-api backend error message i18n via `go-i18n` (currently bare English
  strings in HTTP responses).
- Model whitelist multi-select picker (currently admin types model IDs
  free-form).
- One-click strong-secret generator button (32 random characters).
- Configuration JSON export/import for cross-environment migration.
- Danger Zone collapse for destructive actions.
- IP allowlist for the ProductFlow health probe (rate limit is the chosen
  protection mechanism).
- `beforeunload` is included (R12) but a per-route in-app guard against
  losing unsaved changes is not.

## Implementation Plan

The full step-by-step is in `implement.md`. Summary:

- Phase 0 (0.5d): P0 bug fix (R1) + zh/en i18n (R2) - independently releasable
- Phase 1 (1.0d): all new-api backend changes (R3, R4, R5, R6, R13)
- Phase 2 (0.3d): ProductFlow health probe (R7)
- Phase 3 (1.0d): front-end status card + toggle + callback URL + banner +
  base_url warnings (R4, R8, R11, R12)
- Phase 4 (0.7d): test-connection + batch save + secret confirmation + TTL
  helper + strength meter (R6, R9, R10)
- Phase 5 (0.5d): manage-diff dialog + spec + sync minority locales (R14, R15)

## Risk Points

- The `useUpdateOption` mutation in `web/default/src/features/system-settings/hooks/use-update-option.ts`
  is shared by every settings section. The new
  `useUpdateProductFlowSSOOptions` mutation must be a sibling, not a
  replacement; the global one keeps its current behavior to avoid breaking
  the seven other sections.
- The status card depends on `redis_enabled` from the backend, which today
  is determined at process start by env var. If a deploy enables Redis but
  the new-api process predates that change, the card will show stale info
  until restart - document this as expected.
- The audit log writes the admin_id from the JWT context. If a session is
  invalidated mid-save (rare), the audit row will reference a now-missing
  admin. Acceptable: the audit captures intent at the time of action.
- The detail dialog generic changes table (R14) is new shared UI; ensure
  the column count and styling pass design review before merging Phase 5.
- The shared_secret AlertDialog (R10) cannot be bypassed by keyboard
  shortcuts that submit the form (e.g. Enter in an input field); test
  explicitly that hitting Enter on the secret field opens the dialog rather
  than submitting silently.
- ProductFlow `/api/health/sso` (R7) must NOT call the database or any
  external service - it must respond in <100ms to keep the 3s test timeout
  realistic.

## Current Status

- Not started. Decisions sealed by 2026-05-23 grill-me Q1-Q25.
- Phase 0 can begin immediately and is fully decoupled from later phases.
- Phase 2 requires coordination with whoever lands the ProductFlow health
  probe (likely the same operator, since ProductFlow is in `d:/netcup/ProductFlow`).
