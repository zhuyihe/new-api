# Technical Design

## Boundary

This is a cross-repo change:

- New API owns the authoritative token group, available image model list, optional default model, and SSO verify payload.
- Atelier consumes the SSO payload, lets the user select a model in image-chat generation settings, and applies that model
  override only for hosted SSO image-generation paths.

The compatibility boundary is deliberate: `productflow_sso.*` option keys and `/api/productflow/sso/*` API paths remain
stable even though the visible brand is Atelier.

## Data Flow

```text
New API Atelier SSO settings
  productflow_sso.token_group = GPT-Image-2
  productflow_sso.image_model = optional default
        |
        v
SSO start provisions/reuses user's Atelier token with Group = GPT-Image-2
        |
        v
SSO verify returns token_group + image_model + image_models
        |
        v
Atelier auth session stores token_group + default image_model + image_models
        |
        v
User chooses a model in Atelier 生成设置
        |
        v
Image generation snapshots the chosen image_model at submit time
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

- `NewApiSessionClaims`: `token_group`, `image_model`, `image_models`.
- `AuthSession`: durable columns for `new_api_token_group`, `new_api_image_model`, and `new_api_image_models`.
- `Principal`: carry those values without exposing tokens in repr/logs.
- `ProviderExecutionContext`: carry the effective chosen model, not the whole option list.

Provider resolution:

- Credential override continues to set API key and relay base URL.
- The selected generation-setting model should be validated against the SSO option list, snapshotted on the durable task,
  and applied before image provider creation.
- Local binding model remains fallback only for non-SSO/bootstrap paths. Token-backed SSO sessions without image model
  options should fail clearly and ask the user to re-enter Atelier from AYNC-API, so stale local models cannot bypass the
  New API group contract.

Durable work:

- Persist the effective SSO image model on image-session generation tasks and workflow runs that can execute image
  generation.
- Workers reconstruct the same model from the durable row, not from a later session/config change.

## Atelier Frontend

In hosted SSO mode, settings must not present the local image model field as the active SSO model. Image chat generation
settings show a personal model dropdown sourced from the current SSO session. A fuller settings redesign can happen in the
UI refactor task.

## Compatibility

- Existing token-backed sessions without SSO model options must re-authenticate through AYNC-API before image generation.
  Non-SSO/bootstrap sessions continue to use the local provider binding model.
- Existing bookmarks to `/system-settings/operations/productflow-sso` should continue working through alias/redirect.
- Existing API clients using `/api/productflow/sso/*` continue working.
- Option storage does not need migration because `productflow_sso.image_model` is a new optional key.
