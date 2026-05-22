# ProductFlow SSO Contract

ProductFlow callback:

- Browser lands on `GET /auth/new-api/callback?ticket=<ticket>`.
- ProductFlow backend calls New API `POST /api/productflow/sso/verify`.
- Request header: `Authorization: Bearer <PRODUCTFLOW_SSO_SECRET>`.
- Request body: `{"ticket":"<ticket>"}`.

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
