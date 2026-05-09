# Implementation Plan: OpenRouter Core MVP

## Overview

Implement a Go OpenRouter-only vertical slice before adding the desktop GUI. This keeps the highest-risk behavior, Anthropic compatibility and streaming proxying, testable from the command line.

## Architecture Decisions

- Use Go standard library HTTP first. Wails will embed/control the same core later.
- Keep the config schema provider-neutral internally, even though only OpenRouter is implemented in this slice.
- Keep the existing TypeScript implementation as a behavior reference until the Go core passes equivalent tests.

## Task List

### Phase 1: Foundation

- [x] Task 1: Record MVP spec and implementation plan.
  - Acceptance: spec and plan describe scope, commands, and boundaries.
  - Verify: files exist under `docs/`.

- [x] Task 2: Add Go config and gateway tests.
  - Acceptance: tests define default OpenRouter routes, gateway auth, model list, non-stream translation, and stream translation.
  - Verify: `go test ./...` fails before implementation.

### Phase 2: Go Core

- [x] Task 3: Implement config loading and route resolution.
  - Acceptance: env config maps all Claude aliases to Ring-2.6-1T free by default and supports alias overrides.
  - Verify: config tests pass.

- [x] Task 4: Implement Anthropic-compatible HTTP handlers.
  - Acceptance: `/health`, `/v1/models`, and `/v1/messages` expose the expected behavior.
  - Verify: mocked gateway tests pass.

- [x] Task 5: Implement OpenAI-compatible stream conversion.
  - Acceptance: OpenRouter SSE chunks become Anthropic SSE events.
  - Verify: stream gateway test passes.

### Phase 3: Verification

- [x] Task 6: Run local checks.
  - Acceptance: `go test ./...`, `npm test`, and `npm run build` pass.

- [x] Task 7: Run real OpenRouter E2E when `OPENROUTER_API_KEY` is available.
  - Acceptance: three sequential calls to `inclusionai/ring-2.6-1t:free` succeed.

## Next Checkpoint

After Phase 3, decide whether to scaffold Wails immediately or add provider-neutral route config first.
