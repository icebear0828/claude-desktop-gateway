# Project TODO Plan

## Overview

This plan tracks the next implementation slices after the OpenRouter Go core MVP. The target product is a lightweight cross-platform desktop configuration console for an Anthropic-compatible gateway. The gateway should stay provider-neutral, testable without the GUI, and safe for local, LAN, and VPS deployment.

## Current Priority

Build the GUI MVP next. Additional providers such as DeepSeek, NVIDIA, and
Cloudflare are documented as backlog items only and should not block the desktop
configuration console.

The GUI must follow the RawBlock visual direction: black/white, thick square
borders, no shadows, no rounded corners, uppercase controls, and no decorative
imagery.

## Current Baseline

- Go gateway runs `/health`, `/v1/models`, and `/v1/messages`.
- OpenRouter routes work through `inclusionai/ring-2.6-1t:free`.
- Claude Desktop local 3P setup works with `http://127.0.0.1:8787`.
- Gateway config routes can expose desktop-safe model IDs and optional
  `displayName` metadata for the future GUI.
- GUI can edit OpenRouter key env values in `.env.local` and model routes in
  `gateway.local.json` without writing inline secrets.
- LAN HTTP is rejected by Claude Desktop; LAN/VPS mode requires trusted HTTPS.
- Secrets are env-only. JSON config may reference secret env var names, but must not contain keys.

## Non-Negotiable Rules

- Write tests before behavior changes.
- No TypeScript `any`.
- No API keys in JSON, docs, logs, screenshots, or commits.
- Any external provider integration needs three consecutive real calls before it is marked working.
- Keep the TypeScript implementation as a behavior reference until Go parity is complete.

## Phase 1: Stabilize Local Desktop MVP

### Task 1: Add a repeatable local setup command

**Description:** Add a small command or documented script path that starts the gateway from `.env.local` plus `gateway.local.json`.

**Acceptance criteria:**
- [x] One command starts the Go gateway in local loopback mode.
- [x] Startup fails clearly when required env vars are missing.
- [x] The command does not print secrets.
- [x] A local helper can start, stop, restart, and report status for a
      background gateway process.

**Verification:**
- [x] `curl http://127.0.0.1:8787/health`
- [x] `curl -H 'Authorization: Bearer <gateway key>' http://127.0.0.1:8787/v1/models`
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./scripts`

**Dependencies:** None

**Files likely touched:** `README.md`, `cmd/gateway/`, optional `scripts/`

### Task 2: Harden Claude Desktop config workflow

**Description:** Document and optionally automate writing the flat Claude Desktop 3P config to the active config path.

**Acceptance criteria:**
- [x] Active `configLibrary/_meta.json` is handled correctly by the config doctor.
- [x] Config uses `http://127.0.0.1:8787` for local mode.
- [x] The doc explains `Cmd+Q` relaunch when Claude caches stale provider state.
- [x] Writing or repairing the active flat profile is automated.
- [x] `apply-local` can derive `inferenceModels` from gateway route keys.

**Verification:**
- [x] `go run ./cmd/claude-desktop-config doctor` identifies the active profile from `_meta.json`.
- [x] `go run ./cmd/claude-desktop-config apply-local --home <temp-home>` writes a fixed active profile and doctor passes.
- [ ] Manual Claude Desktop E2E returns the requested exact text.
- [ ] Claude logs show no stale `192.168.10.6` provider URL after relaunch.

**Dependencies:** Task 1

**Files likely touched:** `docs/e2e/claude-desktop.md`, optional `cmd/gateway/`

## Phase 2: Provider-Neutral Core

### Task 3: Extract upstream provider adapter boundary

**Description:** Move OpenAI-compatible request execution out of HTTP handlers into a provider package.

**Acceptance criteria:**
- [x] Gateway handlers only resolve routes, validate requests, and call an adapter.
- [x] OpenRouter behavior remains unchanged.
- [x] Mock upstream tests cover non-stream and stream paths.

**Verification:**
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./internal/gateway`
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./...`

**Dependencies:** Phase 1

**Files likely touched:** `internal/gateway/`, `internal/provider/openai/`, `internal/config/`

### Task 4: Add provider capability metadata

**Description:** Track provider capabilities such as streaming, tools, JSON mode, and OpenAI compatibility.

**Acceptance criteria:**
- [x] Config can represent provider capabilities without breaking existing files.
- [x] Unsupported request features return explicit Anthropic-style errors.
- [x] No silent fallback after a stream has started.

**Verification:**
- [x] Unit tests for unsupported tools, streaming disabled, and unknown provider profile.

**Dependencies:** Task 3

**Files likely touched:** `internal/config/`, `internal/gateway/`, provider tests

## Phase 3: Desktop Configuration Console

### Task 7: Scaffold Wails desktop shell

**Description:** Add a minimal Wails app that controls the existing Go core instead of duplicating gateway logic.

**Acceptance criteria:**
- [x] Desktop app builds without breaking CLI gateway mode.
- [x] GUI can show gateway status, listen address, and active model aliases.
- [x] Build remains cross-platform by design.

**Verification:**
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./...`
- [x] `GOCACHE=/private/tmp/go-build-cache /Users/c/go/bin/wails build`
- [ ] Manual desktop window visual QA after launching locally.

**Dependencies:** Phase 2

**Files likely touched:** `wails.json`, `frontend/`, `cmd/`, `internal/`

### Task 8: Build provider and route editor

**Description:** Add GUI forms for providers, model aliases, and Claude Desktop config output.

**Acceptance criteria:**
- [x] Users can create, update, and delete local OpenRouter/gateway env key
      values through `.env.local` without returning secret values to the UI.
- [x] Users can customize the Claude Desktop model ID and display name for each
      route.
- [x] Invalid route config cannot be saved silently.
- [x] Opening the local packaged `.app` can discover the repository config path
      instead of depending on the launch working directory.
- [ ] Users can add, edit, disable, and test providers.
- [ ] Full provider CRUD remains OpenRouter-only until provider support is
      implemented one provider at a time.
- [ ] Keychain support is deferred unless selected.

**Verification:**
- [x] UI-adjacent Go validation tests for config save, secret save/delete, and
      packaged app root discovery.
- [ ] Manual provider test from GUI triggers a real gateway check.

**Dependencies:** Task 7

**Files likely touched:** `frontend/`, `internal/config/`, `internal/gateway/`

### Task 8A: Add GUI bilingual i18n

**Description:** Add English and Simplified Chinese localization for the Wails
GUI without changing gateway API behavior or config schemas.

**Acceptance criteria:**
- [x] GUI supports `en` and `zh-CN`.
- [x] Language can be switched from the GUI and persists across app restarts.
- [x] Static and dynamic user-facing copy is localized.
- [x] Technical identifiers such as model IDs, env var names, paths, URLs, and
      issue codes remain untranslated.

**Verification:**
- [x] `node --check frontend/main.js`
- [x] `node --check frontend/i18n.js`
- [x] `npm test`
- [x] `npm run build`
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./...`
- [x] `GOCACHE=/private/tmp/go-build-cache /Users/c/go/bin/wails build`
- [x] Manual Wails visual check in English and Chinese.

**Dependencies:** Task 8

**Files likely touched:** `frontend/index.html`, `frontend/main.js`,
`frontend/styles.css`, `frontend/i18n.js`

### Task 9: Add Claude Desktop config actions to GUI

**Description:** Surface the existing doctor/apply-local workflow in the desktop
console so users do not edit Claude Desktop JSON manually.

**Acceptance criteria:**
- [ ] GUI shows active Claude Desktop profile, base URL, and model IDs.
- [ ] GUI can run config doctor and display actionable errors.
- [ ] GUI can apply the local profile using env-based gateway key without
      printing secrets.

**Verification:**
- [ ] Manual GUI check writes the same profile as `apply-local`.
- [ ] `go run ./cmd/claude-desktop-config doctor` reports no config issues.

**Dependencies:** Task 7

**Files likely touched:** `frontend/`, `internal/claudedesktop/`, `cmd/`

## Phase 4: LAN/VPS Ready Mode

### Task 10: Add deployment profiles

**Description:** Provide explicit local, LAN, and VPS profiles so users do not guess between loopback HTTP and trusted HTTPS.

**Acceptance criteria:**
- [ ] Local profile uses loopback HTTP.
- [ ] LAN/VPS profile requires gateway API key and HTTPS guidance.
- [ ] Self-signed certificate limitations are clearly documented.

**Verification:**
- [ ] Config validation tests for loopback, LAN without key, and TLS paths.

**Dependencies:** Phase 1

**Files likely touched:** `internal/config/`, `README.md`, `docs/e2e/`

## Backlog: Additional Free Providers

### Task 11: Keep provider research notes current

**Description:** Maintain official-source notes for DeepSeek, NVIDIA, and
Cloudflare Workers AI or AI Gateway.

**Acceptance criteria:**
- [x] Each provider note records base URL, auth header, request shape, streaming support, model examples, and known limits.
- [x] Each note states whether the API is OpenAI-compatible or needs a custom adapter.
- [ ] Notes are refreshed before any provider implementation starts.

**Verification:**
- [x] Source links are recorded in `docs/providers/`.

**Dependencies:** Task 4

**Files likely touched:** `docs/providers/`

### Task 12: Implement providers one at a time

**Description:** Add DeepSeek, NVIDIA, and Cloudflare only after GUI MVP is
usable.

**Acceptance criteria:**
- [ ] Provider has config examples with env-based secrets only.
- [ ] Mock tests cover auth headers, request body, errors, and streaming if supported.
- [ ] Three real calls pass before the provider is marked complete.

**Verification:**
- [ ] `GOCACHE=/private/tmp/go-build-cache go test ./...`
- [ ] Provider-specific real test with three sequential calls.

**Dependencies:** GUI MVP checkpoint and Task 11

**Files likely touched:** `internal/provider/`, `internal/config/`, `docs/providers/`, examples

## Checkpoints

### MVP Checkpoint

- [ ] Local Claude Desktop E2E passes after a clean relaunch.
- [ ] `go test ./...`, `npm test`, and `npm run build` pass.
- [ ] No secrets appear in tracked files.

### Provider Checkpoint

- [ ] At least two providers work with real 3-call E2E.
- [ ] Provider failures produce clear Anthropic-compatible errors.
- [ ] Route config can switch models without code changes.

### GUI Checkpoint

- [ ] GUI can start/stop or inspect the gateway using the same lifecycle
      semantics as `scripts/local-gateway`.
- [x] GUI can edit OpenRouter model routes and `.env.local` key values safely.
- [ ] Claude Desktop config can be generated or copied without manual JSON mistakes.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Claude Desktop 3P config format changes | High | Keep `docs/e2e/claude-desktop.md` current and verify against real Desktop after changes. |
| Free provider limits or model availability change | Medium | Treat provider docs as versioned notes and require real 3-call E2E before marking support complete. |
| GUI stores secrets unsafely | High | Start with env-only secrets; add OS keychain as a dedicated slice. |
| LAN HTTPS remains hard for users | Medium | Prefer domain plus Caddy or tunnel options over self-signed certificates. |

## Open Questions

- Should GUI MVP start as config-only, or also manage the gateway process lifecycle?
- Should OS keychain support be part of the first GUI release or the next release?
- Which provider should be added first after OpenRouter: DeepSeek, NVIDIA, or Cloudflare?
