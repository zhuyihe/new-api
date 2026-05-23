# ProductFlow SSO Contract

ProductFlow callback:

- Browser lands on `GET /auth/new-api/callback?ticket=<ticket>`.
- ProductFlow backend calls New API `POST /api/productflow/sso/verify`.
- Request header: `Authorization: Bearer <PRODUCTFLOW_SSO_SECRET>`.
- Request body: `{"ticket":"<ticket>"}`.

Configuration:

- New API stores the bridge settings in database-backed `productflow_sso.*` options.
- ProductFlow stores its matching New API bridge settings in database-backed `new_api_*` runtime settings.
- These integration fields are edited through each app's settings UI and must not depend on env fallback at runtime.
- ProductFlow bootstrap settings must also ignore `NEW_API_*` env/.env/file-secret values so stale deployment variables cannot override or break the database-backed bridge.

Expected New API verify response data:

- `user_id`
- `username`
- `email`
- `group`
- `role`
- `token` or `api_key` or `key`
- `token_id`
- `token_name`
- `expires_in` or `session_expires_in`

Security boundary:

- The browser redirect from New API to ProductFlow carries only the ticket.
- ProductFlow stores token material server-side.
- Ordinary ProductFlow frontend code must not receive the token.

Token behavior:

- Token is per New API user.
- Token name defaults to `ProductFlow`.
- Token can be created or reused.
- Recommended token settings are unlimited token quota, optional model whitelist, optional group.
- Ticket TTL and ProductFlow session TTL are also database-backed settings.
