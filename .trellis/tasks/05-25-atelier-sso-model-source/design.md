# Technical Design

## Boundary

This is a cross-repo change:

- New API owns the authoritative token group, available image/text model lists, optional default image model, and SSO
  verify payload.
- Atelier consumes the SSO payload, lets the user select models in image-chat generation settings and product-workbench
  run settings, and applies those overrides only for hosted SSO generation paths.

The compatibility boundary is deliberate: `productflow_sso.*` option keys and `/api/productflow/sso/*` API paths remain
stable even though the visible brand is Atelier.

## Data Flow

```text
New API Atelier SSO settings
  productflow_sso.token_group = Atelier
  productflow_sso.image_model = optional default
        |
        v
SSO start provisions/reuses user's Atelier token with Group = Atelier
        |
        v
SSO verify returns token_group + image_model + image_models + text_model + text_models
        |
        v
Atelier auth session stores token_group + image/text defaults and model lists
        |
        v
User chooses image model in 生成设置 or image/text models in product workbench run settings
        |
        v
Generation snapshots the chosen image_model/text_model at submit time
        |
        v
Atelier calls New API relay with user's token and the selected model
```

## New API Backend

Add `productFlowOptionImageModel = "productflow_sso.image_model"` and a field on `productFlowSSOConfig`.

Validation rule:

- If SSO is disabled, skip model validation.
- If `TokenGroup` is empty, do not require `ImageModel`.
- If `TokenGroup` is set and enabled image models exist for that group:
  - `ImageModel` is optional.
  - When present, `ImageModel` must be one of those models.
- If no image models exist for the selected group, return a clear validation error and block save/test/start.

Expose a RootAuth endpoint for the frontend to fetch image models by group. The helper should query enabled abilities and
filter to models with `image-generation` endpoint metadata. Keep response deterministic and secret-free.

Extend ticket claims with:

- `TokenGroup string json:"token_group,omitempty"`
- `ImageModel string json:"image_model,omitempty"`
- `ImageModels []string json:"image_models,omitempty"`
- `TextModel string json:"text_model,omitempty"`
- `TextModels []string json:"text_models,omitempty"`

`TextModels` is derived from enabled non-image, text-capable models in the token group. There is no persisted
`productflow_sso.text_model` option in this task; `TextModel` is the first deterministic item from `TextModels` when the
list is non-empty.

Do not change the existing `Group` field; it is the user's New API user group.

SSO start UX:

- Check the browser session before validating SSO configuration so public requests cannot infer whether Atelier SSO is
  disabled or misconfigured.
- Keep JSON responses for JSON clients, but return a compact HTML error page for browser clients when configuration
  blocks the redirect.
- Root admins may see the safe validation message and a link to `/system-settings/operations/atelier-sso`; ordinary
  users see a generic "contact administrator" message.

SSO status UX:

- Extend the status response with `configuration_message` and `configuration_issues`.
- Issues should be specific enough for the settings card to act on missing base URL, missing shared secret, token groups
  with no image models, and stale selected default image models.
- Never include secret values, token material, prompts, relay bodies, or stack traces in those fields.

## New API Frontend

Operations section registry:

- Change visible section id to `atelier-sso`.
- Accept `productflow-sso` as a legacy alias and navigate/resolve to `atelier-sso`.
- Keep component/file names unless a later cleanup task is approved.

SSO settings section:

- Add `productflow_sso.image_model` to form defaults, normalized values, dirty diff, and type definitions.
- Add a React Query hook for image models by selected group.
- Use the existing Select pattern from token group.
- Auto-select when the model list has exactly one item.
- Treat the field as an optional default. Show unavailable saved model as a warning and prevent saving until admin clears
  it or picks a valid one.

## Atelier Backend

Extend auth/session contracts:

- `NewApiSessionClaims`: `token_group`, `image_model`, `image_models`, `text_model`, and `text_models`.
- `AuthSession`: durable columns for `new_api_token_group`, `new_api_image_model`, `new_api_image_models`,
  `new_api_text_model`, and `new_api_text_models`.
- `Principal`: carry those values without exposing tokens in repr/logs.
- `ProviderExecutionContext`: carry the effective chosen image/text model, not the whole option list.

Provider resolution:

- Credential override continues to set API key and relay base URL.
- The selected generation-setting model should be validated against the SSO option list, snapshotted on the durable task,
  and applied before image provider creation.
- The selected product-workbench text model should be validated against the SSO `text_models` list and applied to both
  brief and copy model slots for the run.
- Local binding model values remain fallback only for non-SSO/bootstrap paths. Token-backed SSO sessions without required
  model options should fail clearly and ask the user to re-enter Atelier from AYNC-API, so stale local models cannot
  bypass the New API group contract.

Durable work:

- Persist the effective SSO image model on image-session generation tasks and workflow runs that can execute image
  generation.
- Persist the effective SSO text model on workflow runs that can execute copy/text generation.
- Workers reconstruct the same model choices from the durable row, not from a later session/config change.

## Atelier Frontend

In hosted SSO mode, settings must not present local model fields as the active SSO model source. Image chat generation
settings show a personal image model dropdown sourced from the current SSO session. Product workbench run settings show
personal text and image model dropdowns from the same session contract. A fuller settings redesign can happen in the UI
refactor task.

## Compatibility

- Existing token-backed sessions without required SSO model options must re-authenticate through AYNC-API before
  generation. Non-SSO/bootstrap sessions continue to use the local provider binding model values.
- Existing bookmarks to `/system-settings/operations/productflow-sso` should continue working through alias/redirect.
- Existing API clients using `/api/productflow/sso/*` continue working.
- Option storage does not need migration because `productflow_sso.image_model` is a new optional key.
