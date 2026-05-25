# Technical Design

## Boundary

This is a cross-repo change:

- New API owns the authoritative group/model configuration and SSO verify payload.
- Atelier consumes the SSO payload and applies the model override only for hosted SSO image-generation paths.

The compatibility boundary is deliberate: `productflow_sso.*` option keys and `/api/productflow/sso/*` API paths remain
stable even though the visible brand is Atelier.

## Data Flow

```text
New API Atelier SSO settings
  productflow_sso.token_group = GPT-Image-2
  productflow_sso.image_model = gpt-image-2
        |
        v
SSO start provisions/reuses user's Atelier token with Group = GPT-Image-2
        |
        v
SSO verify returns token_group + image_model
        |
        v
Atelier auth session stores token_group + image_model
        |
        v
Image generation snapshots image_model at submit time
        |
        v
Atelier calls New API relay with user's token and model = gpt-image-2
```

## New API Backend

Add `productFlowOptionImageModel = "productflow_sso.image_model"` and a field on `productFlowSSOConfig`.

Validation rule:

- If SSO is disabled, skip model validation.
- If `TokenGroup` is empty, do not require `ImageModel`.
- If `TokenGroup` is set and enabled image models exist for that group:
  - `ImageModel` is required.
  - `ImageModel` must be one of those models.
- If no image models exist for the selected group, return a clear validation error and block save/test/start.

Expose a RootAuth endpoint for the frontend to fetch image models by group. The helper should query enabled abilities and
filter to models with `image-generation` endpoint metadata. Keep response deterministic and secret-free.

Extend ticket claims with:

- `TokenGroup string json:"token_group,omitempty"`
- `ImageModel string json:"image_model,omitempty"`

Do not change the existing `Group` field; it is the user's New API user group.

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
- Show unavailable saved model as a warning but prevent saving until admin picks a valid one.

## Atelier Backend

Extend auth/session contracts:

- `NewApiSessionClaims`: `token_group`, `image_model`.
- `AuthSession`: durable columns for `new_api_token_group` and `new_api_image_model`.
- `Principal` and `ProviderExecutionContext`: carry those values without exposing tokens in repr/logs.

Provider resolution:

- Credential override continues to set API key and relay base URL.
- Image model override should be applied before image provider creation.
- Local binding model remains fallback when no SSO image model exists.

Durable work:

- Persist the effective SSO image model on image-session generation tasks and workflow runs that can execute image
  generation.
- Workers reconstruct the same model from the durable row, not from a later session/config change.

## Atelier Frontend

In hosted SSO mode, settings must not present the local image model field as the active SSO model. The safest minimal UI is
read-only source text near provider config. A fuller settings redesign can happen in the UI refactor task.

## Compatibility

- Existing sessions without `image_model` continue to use the local provider binding model.
- Existing bookmarks to `/system-settings/operations/productflow-sso` should continue working through alias/redirect.
- Existing API clients using `/api/productflow/sso/*` continue working.
- Option storage does not need migration because `productflow_sso.image_model` is a new optional key.

