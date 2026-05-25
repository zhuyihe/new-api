---
task: atelier-sso-model-source
parent_task: productflow-sso
date: 2026-05-25
scope_tier: B
cross_repo: true
---

# Atelier SSO Image Model Source of Truth

## Goal

Make New API the source of truth for the image-generation model used by Atelier SSO sessions.

The admin should choose both values in the New API operations settings page:

- `Token group`: the New API group assigned to the per-user Atelier token.
- `Image model`: the concrete image model Atelier sends to the New API relay.

Atelier must consume that SSO-provided model for interactive image generation instead of using the editable image model
from Atelier `/settings` while an SSO model is present.

Also finish the visible rebrand gap: the New API settings section should be exposed as **Atelier SSO**, with the route
slug `/system-settings/operations/atelier-sso`. Internal compatibility identifiers may still use `productflow_sso` and
`/api/productflow/sso/*`.

## Why Now

Production failure proved the current split-brain configuration is unsafe:

- New API SSO token group was changed to `GPT-Image-2`.
- Atelier provider binding still sent `model=gpt-image-1`.
- New API relay rejected the request because that group had channels for `gpt-image-2`, not `gpt-image-1`.

The earlier `05-24-sso-token-group-selector` task intentionally left model selection as backlog. That is no longer a
nice-to-have; group and model must be configured in the same source system.

## Decisions

| # | Question | Decision | Reason |
|---|----------|----------|--------|
| 1 | Where does admin choose the image model? | New API `System Settings -> Operations -> Atelier SSO` | Group membership and channel availability live in New API; choosing the model in Atelier can drift. |
| 2 | Does Atelier `/settings` still edit the active SSO image model? | No, read-only/hidden under SSO mode | The local provider binding is a fallback for non-SSO/bootstrap usage, not the active hosted SSO model. |
| 3 | Can admin choose multiple SSO groups? | Not in this task | New API token `Group` is a single field today. Multi-group routing needs a larger token/channel design. |
| 4 | How are image model options populated? | From enabled models available in the selected New API group and classified as image-generation capable | Prevents picking a model that the token group cannot route. |
| 5 | What if the group has one image model? | Auto-select it | Avoids needless admin work and matches the common dedicated image group case. |
| 6 | What if the group has multiple image models? | Admin selects one | This is the only place model choice belongs for hosted SSO. |
| 7 | What if the group has no image model? | Block save/test and show a validation message | Saving a guaranteed-broken SSO config recreates the production failure class. |
| 8 | Rename `productflow-sso` everywhere? | Rename visible route/nav/log wording; keep internal option keys, file names, and API paths unless a separate migration task is approved | Rebrand should not create avoidable cross-service compatibility risk. |
| 9 | Old URL compatibility | Keep `/system-settings/operations/productflow-sso` as an alias/redirect to `/system-settings/operations/atelier-sso` | Existing bookmarks and support links should not break. |

## Requirements

### R1 - New API stores the selected image model

- Add a new option key: `productflow_sso.image_model`.
- Default is empty. Empty is invalid when SSO is enabled and `productflow_sso.token_group` is set to a group that has one
  or more image-generation models.
- Validation must confirm the selected model is enabled for the selected token group.
- The key name intentionally keeps the `productflow_sso.*` prefix for compatibility with existing option handling.

### R2 - New API exposes image model choices for the selected group

- Add a RootAuth-protected endpoint or extend the existing SSO config API surface to return image model choices for a
  group.
- The source must be New API channel/model configuration, not a hard-coded list.
- The response should be deterministic and small, for example:

```json
{
  "success": true,
  "data": {
    "group": "GPT-Image-2",
    "models": ["gpt-image-2"]
  }
}
```

Implementation guidance:

- Start from enabled `abilities` rows for the selected group.
- Prefer models whose metadata/endpoints include `image-generation`.
- Sort deterministically.
- Do not expose channel API keys or channel internals.

### R3 - New API SSO settings page adds Image model

- Rename the visible section route/nav to `/system-settings/operations/atelier-sso`.
- Keep an alias/redirect for `/system-settings/operations/productflow-sso`.
- Add an **Image model** select below **Token group**.
- When token group changes:
  - reload image model options;
  - auto-select the only available image model;
  - clear or mark invalid a saved model that is no longer available.
- Save/test buttons must be blocked when the selected group has no image model or the selected model is unavailable.

### R4 - SSO verify carries the image model

Extend verify response `data` with:

- `token_group`: the group assigned to the Atelier token.
- `image_model`: the selected image-generation model.

The existing `group` claim remains the user's New API user group. Do not overload it with token group.

### R5 - Atelier consumes SSO model for image generation

- Store the returned `token_group` and `image_model` on the server-side auth session.
- Add these fields to `Principal` / provider execution context.
- For interactive SSO image generation, effective image model must come from the SSO model claim when present.
- Durable image-session/workflow rows should snapshot the image model chosen at submit time so retries keep the same
  billing/routing model.
- Local provider binding image model remains the fallback for non-SSO/bootstrap or sessions created before the new claim.

### R6 - Atelier settings UI avoids split-brain editing

- In hosted SSO mode, do not let admin edit the active SSO image model in Atelier `/settings`.
- Show read-only source information instead:
  - source: New API
  - token group
  - image model
- Provider profile credentials/settings that are still relevant outside SSO can stay in the settings page, but the UI must
  not imply that its image model overrides New API SSO.

### R7 - Error UX stays safe

- If New API returns quota/model-disabled/group-missing errors, Atelier should show a safe, actionable message and not
  leak tokens, raw relay URLs, upstream request bodies, prompts, or stack traces.
- The detailed relay classification remains a follow-up unless it is needed to make this task testable.
- New API's browser SSO start endpoint must not render raw JSON for configuration errors. Unauthenticated users should
  still be redirected to sign-in first; authenticated browser users should see a compact human-readable error page, and
  root admins should get a link to `/system-settings/operations/atelier-sso`.
- The New API Atelier SSO status endpoint should return safe `configuration_message` / `configuration_issues` fields so
  the settings page can show exactly which saved field needs attention without exposing secret material.

## Out of Scope

- Multi-token or multi-group selection for a single Atelier user.
- Product/session-level cost aggregation.
- Quota balance display inside Atelier.
- Renaming internal Go/Python identifiers, option keys, database columns, file names, or `/api/productflow/sso/*`.
- Reworking text/copy model source-of-truth. Until that is separately designed, the selected SSO token group must also
  support the text models configured in Atelier provider bindings if those flows are used.

## Acceptance Criteria

- [ ] New API SSO settings is reachable at `/system-settings/operations/atelier-sso`.
- [ ] Old `/system-settings/operations/productflow-sso` links land on the same section without a blank page.
- [ ] The page shows `Token group` and `Image model` together.
- [ ] Selecting `GPT-Image-2` offers `gpt-image-2` and does not keep stale `gpt-image-1`.
- [ ] Saving invalid group/model combinations is blocked.
- [ ] SSO verify payload includes `token_group` and `image_model`.
- [ ] Atelier image generation sends the SSO-provided `image_model` to the relay.
- [ ] A regression test proves a stale local provider binding model is ignored when an SSO image model is present.
