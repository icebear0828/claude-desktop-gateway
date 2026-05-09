# Spec: Desktop Gateway MVP

## Objective

Build a lightweight cross-platform desktop configuration console for a local Anthropic-compatible inference gateway. The MVP starts with an OpenRouter provider using `inclusionai/ring-2.6-1t:free`, then expands to other free or low-cost OpenAI-compatible providers.

## Tech Stack

- Backend/core: Go 1.26, standard library HTTP server first.
- Desktop shell: Wails v2 in a later slice.
- Frontend: React + TypeScript in the Wails frontend.
- Upstream protocol: OpenAI-compatible Chat Completions for OpenRouter.
- Client protocol: Anthropic Messages API subset at `/v1/messages`.

## Commands

- Build core: `go test ./...`
- Run core: `go run ./cmd/gateway`
- Existing TS regression tests: `npm test`
- Existing TS build: `npm run build`
- Real OpenRouter E2E: requires `OPENROUTER_API_KEY`; each external provider must pass three sequential calls before being marked working.

## Project Structure

- `cmd/gateway/` starts the local gateway service.
- `internal/config/` owns config loading, validation, and model route resolution.
- `internal/gateway/` owns Anthropic-compatible HTTP endpoints.
- `internal/provider/openai/` owns OpenAI-compatible upstream calls and streaming conversion.
- `docs/specs/` and `docs/plans/` hold design decisions and implementation plans.
- Existing `src/` TypeScript code remains as a behavior reference until the Go core replaces it.

## Code Style

Use small Go packages with explicit structs at boundaries. Keep provider-specific logic out of HTTP handlers.

```go
route, ok := cfg.ResolveRoute(req.Model)
if !ok {
    return AnthropicError{Type: "invalid_request_error", Message: "model is not configured"}
}
```

## Testing Strategy

Use Go unit/integration tests with `httptest` for mocked upstream behavior. Test config parsing, auth, request translation, response normalization, and streaming SSE conversion. Real provider tests are opt-in via environment variables and must perform three sequential calls.

## Boundaries

- Always: validate config before serving, avoid logging prompt content, keep API keys out of config files.
- Ask first: adding third-party Go dependencies, changing the public config schema, deleting the existing TypeScript implementation.
- Never: commit real API keys, silently swallow unsupported provider capabilities, fallback after a stream has started.

## Success Criteria

- `/health`, `/v1/models`, and `/v1/messages` work in the Go core.
- `claude-opus-4-7`, `claude-sonnet-4-6`, and `claude-haiku-4-5` default to `inclusionai/ring-2.6-1t:free`.
- OpenRouter receives `/chat/completions` requests with the correct model, auth, and optional attribution headers.
- Non-stream and stream responses are normalized to Anthropic-compatible output.
- `go test ./...`, `npm test`, and `npm run build` pass after the first core slice.

## Open Questions

- Whether Wails v2 should be scaffolded in this repository immediately after the core slice or after one real OpenRouter E2E pass.
- Whether the first GUI stores keys in environment variables only or immediately adds OS keychain integration.
