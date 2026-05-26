---
task: atelier-sso-model-source
parent_task: productflow-sso
date: 2026-05-25
scope_tier: B
cross_repo: true
---

# Atelier SSO Model Source of Truth

## Goal

Make New API the source of truth for the image-generation and text/copy models available to Atelier SSO sessions, while
the active models are chosen by the signed-in Atelier user in **生成设置** or the product workbench run settings.

The admin should configure the token group in the New API operations settings page:

- `Token group`: the New API group assigned to the per-user Atelier token.
- `Image model`: an optional default/preferred model. It is not the only model Atelier may use.

Atelier must consume the SSO-provided model lists for interactive image generation and product-workbench text/image
workflow runs. The selected model is submitted with that generation/run and snapshotted for retries. Atelier `/settings`
provider bindings remain provider-kind/bootstrap configuration and must not decide the active SSO image or text model.

Also finish the visible rebrand gap: the New API settings section should be exposed as **Atelier SSO**, with the route
slug `/system-settings/operations/atelier-sso`. Internal compatibility identifiers may still use `productflow_sso` and
`/api/productflow/sso/*`.

## Why Now

Production failure proved the current split-brain configuration is unsafe:

- New API SSO token group was changed to an image group.
- Atelier provider binding still sent a stale local image model.
- New API relay rejected the request because that group could not route the stale model.

The earlier `05-24-sso-token-group-selector` task intentionally left model selection as backlog. That is no longer a
nice-to-have; group and model must be configured in the same source system.

## Decisions

| # | Question | Decision | Reason |
|---|----------|----------|--------|
| 1 | Where does admin define available SSO models? | New API token group/channel config | Group membership and channel availability live in New API. |
| 2 | Where does the active model get chosen? | Atelier image-chat **生成设置** and product workbench run settings per user/session | Different users/groups can have different model options, and the choice belongs to the generation request/run. |
| 3 | Can admin choose multiple SSO groups? | Not in this task | New API token `Group` is a single field today. Multi-group routing needs a larger token/channel design. |
| 4 | How are image model options populated? | From enabled models available in the selected New API group and classified as image-generation capable | Prevents picking a model that the token group cannot route. |
| 5 | What if the group has one image model? | Atelier auto-selects it in 生成设置 | Avoids needless user work and matches the common dedicated image group case. |
| 6 | What if the group has multiple image models? | Atelier shows a personal dropdown in 生成设置 | The active hosted SSO model is a per-generation user choice. |
| 7 | What if the group has no image model? | Block save/test and show a validation message | Saving a guaranteed-broken SSO config recreates the production failure class. |
| 8 | Rename `productflow-sso` everywhere? | Rename visible route/nav/log wording; keep internal option keys, file names, and API paths unless a separate migration task is approved | Rebrand should not create avoidable cross-service compatibility risk. |
| 9 | Old URL compatibility | Keep `/system-settings/operations/productflow-sso` as an alias/redirect to `/system-settings/operations/atelier-sso` | Existing bookmarks and support links should not break. |
| 10 | Where do text/copy models come from? | Enabled non-image text-capable models in the same New API token group | The token group should explain both the routing and billing envelope for a user's Atelier work. |
| 11 | Is there a New API default text model setting? | Not in this task | Atelier can default to the first deterministic SSO `text_models` entry and let the user choose per run. |

## Requirements

### R1 - New API stores an optional default image model

- Add a new option key: `productflow_sso.image_model`.
- Default is empty. Empty is valid when the selected token group has one or more image-generation models.
- If populated, validation must confirm the default model is enabled for the selected token group.
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

### R3 - New API SSO settings page exposes optional default Image model

- Rename the visible section route/nav to `/system-settings/operations/atelier-sso`.
- Keep an alias/redirect for `/system-settings/operations/productflow-sso`.
- Add an optional **Image model** select below **Token group** as the preferred default shown first in Atelier.
- Show read-only **Text models** derived from the same token group so admins can confirm the group also supports
  product-workbench copy/text runs. Do not persist a New API default text model in this task.
- When token group changes:
  - reload image model options;
  - reload text model preview;
  - auto-select the only available image model;
  - clear or mark invalid a saved default model that is no longer available.
- Save/test buttons must be blocked when the selected group has no image model or the saved default model is unavailable.

### R4 - SSO verify carries image and text model options

Extend verify response `data` with:

- `token_group`: the group assigned to the Atelier token.
- `image_model`: optional default image-generation model.
- `image_models`: ordered image-generation models enabled for the token group.
- `text_model`: default text/copy model, currently the first ordered enabled text-capable model for the token group.
- `text_models`: ordered non-image text-capable models enabled for the token group.

The existing `group` claim remains the user's New API user group. Do not overload it with token group.

### R5 - Atelier consumes SSO model options for generation

- Store the returned `token_group`, default `image_model`, `image_models`, `text_model`, and `text_models` on the
  server-side auth session.
- Add these fields to `Principal` / provider execution context.
- For interactive SSO image generation, effective image model must come from the user's generation-setting selection when
  present, validated against the SSO `image_models` list. If the user has not selected one, Atelier may fall back to the
  default `image_model` or first available SSO image model.
- For product-workbench copy/text nodes, effective text model must come from the user's run-setting selection when
  present, validated against the SSO `text_models` list. If the user has not selected one, Atelier may fall back to the
  default `text_model` or first available SSO text model.
- Durable image-session/workflow rows should snapshot the image/text model chosen at submit time so retries keep the same
  billing/routing model.
- Local provider binding model values remain the fallback for non-SSO/bootstrap sessions only. Token-backed SSO sessions
  that do not carry required model options must fail with a safe re-entry/configuration message instead of silently using
  stale local models.

### R6 - Atelier settings UI avoids split-brain editing

- In hosted SSO mode, do not let admin edit the active SSO image model in Atelier `/settings`.
- Show read-only source information instead:
  - source: New API
  - token group
  - available image models / default image model

### R6b - Atelier image-chat generation settings show personal model options

- `GET /api/auth/session` exposes the SSO image model options for the current user.
- Image chat **生成设置** shows a model dropdown when SSO model options are present.
- The dropdown is locked to the user's New API token-group models; free-text local model editing is not available in SSO
  mode.
- If no SSO image models are available for a token-backed session, show a disabled "no available models" state and let the
  backend return the safe configuration error.
- Provider profile credentials/settings that are still relevant outside SSO can stay in the settings page, but the UI must
  not imply that its image model overrides New API SSO.

### R6c - Atelier product workbench run settings show personal model options

- `GET /api/auth/session` exposes the SSO text model options for the current user.
- Product workbench run settings show text and image model dropdowns when SSO model options are present.
- Running a copy node sends the selected `text_model`; running an image node sends the selected `image_model`.
- The backend validates submitted models against the current SSO session option lists and snapshots accepted choices on
  `workflow_runs`.

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
- Multi-default model policy in New API for text/copy models.

## Acceptance Criteria

- [ ] New API SSO settings is reachable at `/system-settings/operations/atelier-sso`.
- [ ] Old `/system-settings/operations/productflow-sso` links land on the same section without a blank page.
- [ ] The New API page shows `Token group` and optional default `Image model` together.
- [ ] The New API page shows read-only text models derived from the same token group.
- [ ] Selecting `GPT-Image-2` offers `gpt-image-2` and does not keep stale `gpt-image-1`.
- [ ] Saving invalid group/model combinations is blocked.
- [ ] SSO verify payload includes `token_group`, optional `image_model`, `image_models`, `text_model`, and `text_models`.
- [ ] Atelier image chat `生成设置` shows a per-user model dropdown from the SSO model list.
- [ ] Atelier image generation sends the user-selected model to the relay and snapshots it on the durable task.
- [ ] Atelier product workbench run settings show per-user text/image model dropdowns from the SSO model lists.
- [ ] Atelier workflow run sends selected `text_model` / `image_model` and snapshots them on `workflow_runs`.
- [ ] A regression test proves a stale local provider binding model is ignored when an SSO-selected image model is present.
