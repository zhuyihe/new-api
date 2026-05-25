# Implementation Plan

## Phase 0 - Confirm Contracts

- [ ] Re-read New API `productflow-sso.md` contract and Atelier provider/auth specs.
- [ ] Inspect current New API model metadata / abilities helpers and pick the least invasive image-model query helper.
- [ ] Confirm ProductFlow/Atelier dirty worktree changes are unrelated before editing backend files.

## Phase 1 - New API Backend

- [ ] Add `productflow_sso.image_model` config option, defaults, validation, and status/config DTO plumbing.
- [ ] Add RootAuth image-model choices endpoint for a token group.
- [ ] Extend SSO ticket claims and verify response with `token_group` and `image_model`.
- [ ] Add backend tests for valid group/model, stale model rejection, and verify payload.

## Phase 2 - New API Frontend

- [ ] Rename visible operations section route id to `atelier-sso`.
- [ ] Add legacy alias handling for `productflow-sso`.
- [ ] Add `Image model` select tied to selected token group.
- [ ] Add i18n keys for new visible messages.
- [ ] Build/lint targeted frontend.

## Phase 3 - Atelier Backend

- [ ] Add auth-session columns for `new_api_token_group` and `new_api_image_model`.
- [ ] Parse and persist new SSO verify fields.
- [ ] Carry image model through `Principal` and `ProviderExecutionContext`.
- [ ] Override effective image model for SSO image-generation paths.
- [ ] Snapshot model on durable image tasks/workflow runs.
- [ ] Add regression tests proving stale local image binding does not override SSO model.

## Phase 4 - Atelier UI

- [ ] Make the hosted SSO active image model read-only in settings.
- [ ] Show source/group/model in a compact admin-facing area.
- [ ] Avoid larger visual redesign; leave that to the Atelier UI refactor task.

## Phase 5 - Validation

- [ ] New API backend targeted tests.
- [ ] New API web build/lint.
- [ ] Atelier backend ruff + targeted tests.
- [ ] Manual staging check: select `GPT-Image-2` + `gpt-image-2`, enter Atelier image chat, generate one image.

