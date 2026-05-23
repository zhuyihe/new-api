# ProductFlow SSO Patch

## Goal

Add the minimal New API-side integration required for ProductFlow to behave as a New API-backed image workspace. New API remains authoritative for users, billing, quotas, tokens, channels, and consumption logs. ProductFlow owns image workspace data and uses a per-user New API token only on its backend.

## Requirements

- A logged-in New API user can open ProductFlow from the New API sidebar in a new browser tab.
- The sidebar entry points to a New API SSO start endpoint, not directly to ProductFlow.
- The SSO start endpoint validates the browser's New API session, creates a short-lived one-time ticket, creates or reuses that user's dedicated ProductFlow token, then redirects the browser to ProductFlow `/auth/new-api/callback?ticket=...`.
- If the browser is not logged into New API, the SSO start endpoint redirects to New API sign-in and preserves a redirect back to the SSO start endpoint.
- ProductFlow verifies the ticket with a server-to-server New API endpoint protected by a shared secret.
- Verification consumes the ticket exactly once and returns the New API user identity plus the user's dedicated ProductFlow token to ProductFlow's backend.
- The browser must never receive the token from New API directly.
- ProductFlow token provisioning must be configurable through database-backed settings on the New API side, and ProductFlow's matching `new_api_*` runtime settings must also be database-backed. Environment variables must not be required as a runtime fallback for these integration fields.
- Keep the patch thin and easy to rebase: do not modify New API relay, billing, channel routing, quota settlement, or generic user behavior.

## Acceptance Criteria

- [x] New API exposes `GET /api/productflow/sso/start`.
- [x] New API exposes `POST /api/productflow/sso/verify`.
- [x] `POST /api/productflow/sso/verify` rejects missing or wrong `Authorization: Bearer <secret>`.
- [x] A valid ticket can be verified once and fails on a second verification attempt.
- [x] SSO start redirects unauthenticated users to New API sign-in.
- [x] SSO start for an authenticated user creates or reuses the configured token name, default `ProductFlow`.
- [x] The token is returned only by server-to-server verification, never through the browser redirect.
- [x] The new default UI sidebar entry opens the SSO start endpoint in a new tab.
- [x] Backend tests cover secret validation, single-use ticket behavior, unauthenticated redirect, and token provisioning.
- [x] Frontend type-check or build validates the sidebar change.

## Validation

- `go test ./controller -run ProductFlow -count=1 -timeout 60s`
- `go test ./router ./model -count=1 -timeout 60s`
- `npm run typecheck` from `web/default`
- `npm run build` from `web/default`
- `git diff --check`

## Configuration

- New API stores the ProductFlow bridge settings in database-backed system settings under `productflow_sso.*`.
- ProductFlow stores its matching New API bridge settings in database-backed runtime settings under `new_api_*`.
- Environment variables remain reserved for bootstrap secrets and infrastructure wiring, not as the normal configuration source for this bridge.
- ProductFlow ignores `NEW_API_*` env/.env/file-secret values during bootstrap; only explicit database/runtime overrides or code defaults hydrate these bridge fields.

## Out Of Scope

- ProductFlow database/session/tenant-isolation work.
- New API billing expression, model pricing, relay provider behavior, quota logic, or channel selection.
- Classic UI sidebar changes unless explicitly requested later.
