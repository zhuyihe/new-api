# Implementation Plan

## Phase 0 - Confirm Contracts

- [x] Re-read New API `productflow-sso.md` contract and Atelier provider/auth specs.
- [x] Inspect current New API model metadata / abilities helpers and pick the least invasive image-model query helper.
- [x] Confirm ProductFlow/Atelier dirty worktree changes are unrelated before editing backend files.

## Phase 1 - New API Backend

- [x] Keep `productflow_sso.image_model` as an optional default, validate it only when present, and keep status/config DTO plumbing aligned.
- [x] Add RootAuth image-model choices endpoint for a token group.
- [x] Extend SSO ticket claims and verify response with `token_group`, optional `image_model`, and `image_models`.
- [x] Extend SSO ticket claims and verify response with `text_model` and `text_models` derived from enabled text-capable
  models in the token group.
- [x] Add backend tests for valid blank/default model behavior, stale model rejection, and verify payload.

## Phase 2 - New API Frontend

- [x] Rename visible operations section route id to `atelier-sso`.
- [x] Add legacy alias handling for `productflow-sso`.
- [x] Add optional default `Image model` select tied to selected token group.
- [x] Show read-only text model options for the selected token group on the New API Atelier SSO settings page.
- [x] Add i18n keys for new visible messages.
- [x] Build/lint targeted frontend.

## Phase 3 - Atelier Backend

- [x] Add auth-session columns for `new_api_token_group`, `new_api_image_model`, and `new_api_image_models`.
- [x] Add auth-session/workflow-run columns for `new_api_text_model` and `new_api_text_models` where durable text model
  selection is needed.
- [x] Parse and persist new SSO verify fields, including the model option list.
- [x] Carry model options through `Principal`; pass only the effective chosen model through `ProviderExecutionContext`.
- [x] Override effective image model from the user's generation-setting selection for SSO image-generation paths.
- [x] Override effective text model from the user's product-workbench run selection for SSO copy/text paths.
- [x] Snapshot model on durable image tasks/workflow runs.
- [x] Add regression tests proving stale local image binding does not override SSO model.

## Phase 4 - Atelier UI

- [x] Make the hosted SSO active image model non-editable in admin settings.
- [x] Add the per-user New API model dropdown to image-chat `生成设置`.
- [x] Add per-user New API text/image model dropdowns to product workbench run settings.
- [x] Avoid larger visual redesign; leave that to the Atelier UI refactor task.

## Phase 5 - Validation

- [x] New API backend targeted tests.
- [x] New API web build/lint.
- [x] Atelier backend ruff + targeted tests.
- [ ] Manual staging check after deployment: enter Atelier image chat, choose a New API model in `生成设置`, generate one image, and confirm the relay sees the selected model.

## Phase 6 - SSO Start/Status UX Hardening

- [x] Ensure unauthenticated `/api/productflow/sso/start` requests redirect to sign-in before config validation.
- [x] Return a browser-friendly HTML error page for authenticated browser users when saved SSO config blocks start.
- [x] Keep JSON error responses for JSON clients.
- [x] Add safe `configuration_message` / `configuration_issues` fields to the status endpoint.
- [x] Render the status issues in the New API Atelier SSO settings card.
