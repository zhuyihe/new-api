---
task: productflow-sso-followups
parent_task: productflow-sso
audit_session: 2026-05-23
related_productflow_task: multitenant-hardening
---

# ProductFlow SSO Followups - Startup WARN and Deployment Mode Doc

## Goal

Surface the only deviation the 2026-05-23 grill-me audit found in the new-api
ProductFlow SSO implementation: the in-memory ticket fallback is per-process and
therefore unusable for multi-process deployments. The behavior itself is
acceptable (the contract permits a "single-process" fallback), but no operator
warning exists. Add a startup `WARN` when the dangerous configuration combination
is present and document the deployment-mode contract so future deployers find it.

## Why Now

ProductFlow is preparing to ship to `image.aync.cc.cd` as a public multi-tenant
workspace. ProductFlow's smoke script (in the sibling task) will repeat the SSO
flow three times to catch the multi-process symptom, but a single-line startup
log on the new-api side is the cheapest way to make the constraint visible
before users see 401s.

## Requirements

- When `productflow_sso` is operable for browser start/verify
  (`productflow_sso.base_url` and `productflow_sso.shared_secret` both validate)
  and `common.RedisEnabled` is `false`, new-api logs a single `WARN` line at
  startup naming the constraint and pointing operators to the deployment-mode
  documentation.
- The `productflow-sso.md` spec gains a new section "Deployment Modes" listing
  which combinations of `RedisEnabled` and process count are supported.
- No behavior change in the request path. The in-memory fallback continues to
  work for single-process deploys.

## Acceptance Criteria

- [ ] Starting new-api with valid `productflow_sso.base_url` and
      `productflow_sso.shared_secret` values and Redis disabled emits exactly
      one `WARN` log line within five seconds of startup. The line names the
      multi-process limitation.
- [ ] Starting new-api with the same SSO config and Redis enabled emits no
      such warning.
- [ ] Starting new-api with only partial SSO config (for example, shared
      secret without a valid base URL) emits no warning because the flow is not
      yet operable.
- [ ] `productflow-sso.md` includes a "Deployment Modes" section with a table
      showing which combinations are supported.
- [ ] No existing test fails. A new test under `controller/` covers the WARN
      emission (capture log output, assert presence/absence based on
      configuration).

## Out of Scope

- Switching the in-memory fallback to a cross-process store (DB table or
  similar). The PRD's "thin patch" rule rules out structural changes to
  ticket storage.
- Hard-failing startup when Redis is disabled. The contract explicitly allows
  single-process deploys to run without Redis.
- Any change to the SSO request handlers.

## Implementation Plan

### File: `controller/productflow_sso.go` (or appropriate init location)

Add an `init()` (or a function called from `main.go` near startup) that checks
the configuration:

```go
func warnIfTicketFallbackIsRiskyOnStartup() {
    cfg, err := getProductFlowSSOConfig()
    if err != nil || cfg.SharedSecret == "" {
        return // SSO not configured; nothing to warn about
    }
    if common.RedisEnabled {
        return // safe path
    }
    common.SysLog(
        "WARN: productflow_sso is configured but Redis is disabled; " +
            "ticket storage falls back to in-process memory and " +
            "ONLY supports single-process deployments. Multi-process " +
            "or multi-instance deployments will silently fail SSO. " +
            "See .trellis/spec/backend/productflow-sso.md section 8.",
    )
}
```

Wire the call into the existing startup sequence (where other `init` /
`Initialize*` helpers run).

### File: `.trellis/spec/backend/productflow-sso.md`

Append a "Deployment Modes" section:

```markdown
### 8. Deployment Modes

| Deployment | Redis enabled | Supported |
|------------|---------------|-----------|
| Single process | yes | Yes |
| Single process | no | Yes (in-memory ticket fallback) |
| Multi process / multi instance | yes | Yes |
| Multi process / multi instance | no | NO - SSO will fail intermittently |

When `productflow_sso` is configured and Redis is disabled, new-api logs a
startup `WARN` naming this constraint. Multi-process deployers must enable
Redis for ProductFlow SSO to function.
```

### Tests

- `controller/productflow_sso_warn_test.go` (new): two cases, one asserting the
  WARN line appears, one asserting it does not. Use a log-capture helper if one
  exists; otherwise add a small one.

## Validation

```bash
go test ./controller -run ProductFlow -count=1 -timeout 60s
```

## Risk Points

- The startup log must not crash the process if `getProductFlowSSOConfig`
  returns an error during early init (some test harnesses start without a DB).
  The early `return` on error keeps this safe.
- `common.SysLog` is the standard logging path; using anything else risks the
  WARN being filtered by log-level configuration. Confirm `SysLog` honors the
  WARN level convention used elsewhere.
