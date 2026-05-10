# Project Memory

## 2026-05-10 OpenRouter Anthropic Messages Profile

- Added gateway provider profile `anthropic-messages` for OpenRouter's `/messages` endpoint.
- The profile rewrites only the request `model` from Claude Desktop route ID to upstream model and preserves Anthropic-native fields such as `tools`, `tool_choice`, `tool_result`, `cache_control`, and `thinking`.
- Non-stream `/messages` responses rewrite top-level `model` back to the Claude Desktop model ID. Streaming `/messages` responses pass SSE through and rewrite `message_start.message.model` back to the Desktop ID.
- Route arrays now have runtime fallback semantics: upstream 429, 5xx, and transport failures fall through to the next route; 400/401/403 do not fallback.
- OpenRouter errors now include safe upstream metadata such as provider name and metadata raw message when present; keys and prompt content remain hidden.
- Real OpenRouter `anthropic-messages` non-stream three-call test passed on 2026-05-10 against `inclusionai/ring-2.6-1t:free`.
- Real stream/tools tests can still fail due to `Novita` 429 on the free Ring model. Treat this as an upstream/provider stability issue unless a fallback route or steadier model is configured.

## 2026-05-10 Dynamic Free Model Defaults

- Route entries can now include `dynamicFreeModels` to fetch OpenRouter's `/models` catalog at runtime, filter zero-price models by required parameters such as `tools` and `tool_choice`, and cache the catalog for `catalogCacheTTLSeconds`.
- Hardcoded free model IDs should live under `dynamicFreeModels.fallback` only. The gateway appends them after dynamically discovered free models, or uses them when catalog discovery fails.
- Route entries can include `cache` with `enabled` and `ttlSeconds`; enabled routes send OpenRouter response-cache headers and forward safe cache status headers such as `X-OpenRouter-Cache-Status` back to the client.
- Example configs expose `claude-ring-2-6-1t-free`, `claude-free-auto`, `claude-free-agent`, `claude-free-coder`, and `claude-free-fast`. Ring is a dedicated selectable route; `claude-free-auto` points directly at `openrouter/free`; the task-oriented aliases use dynamic discovery first.
- Gateway fallback now treats upstream 402 provider spend-limit errors as retryable, so a single free provider such as Venice exhausting an API-key spend limit does not block the whole route chain.
- On 2026-05-10, `gateway.local.json` and the active Claude Desktop local profile were updated to expose `claude-ring-2-6-1t-free` as a standalone selectable model alongside the free auto/agent/coder/fast aliases. Doctor reported no config issues; Claude Desktop still needs Cmd+Q restart to reload profile state.
- Follow-up review fixes on 2026-05-10: env-only OpenRouter now defaults to `anthropic-messages`; GUI route edits preserve `dynamicFreeModels` and `cache` metadata instead of dropping them on save.

## 2026-05-09 Local Gateway Startup

- Preferred local MVP startup command is `./scripts/run-local`.
- The script loads `.env.local`, uses `gateway.local.json`, requires `OPENROUTER_API_KEY` and `CLAUDE_GATEWAY_API_KEY`, and starts the Go gateway on loopback.
- Use `./scripts/run-local --dry-run` to validate local files and required env vars without starting the server. It must not print secret values.

## 2026-05-09 Claude Desktop Config Diagnosis

- Do not guess which Claude Desktop JSON is active. Read `Claude-3p/configLibrary/_meta.json`, follow `appliedId`, then inspect `configLibrary/<appliedId>.json`.
- Added `go run ./cmd/claude-desktop-config doctor` to diagnose the active profile, flat `inferenceGateway*` fields, stale base URLs, LAN HTTP misuse, and profiles that exist but are not applied.
- The doctor reports paths, profile IDs, and base URLs only; it must not print gateway API key values.
- Added `go run ./cmd/claude-desktop-config apply-local` to write a fixed local profile (`00000000-0000-4000-8000-000000087870`), update `_meta.json.appliedId`, and set `deploymentMode: "3p"` in both Claude root configs.
- `apply-local` reads `CLAUDE_GATEWAY_API_KEY` from env, writes it only to the local Claude Desktop profile, redacts it from output, and preserves non-gateway fields from the previously active profile.
- On this machine, `apply-local` was run successfully and doctor reports active profile `00000000-0000-4000-8000-000000087870` with `http://127.0.0.1:8787`. The old profile `779eec7e-4d53-46a9-974d-ffbfc7f6d01a` remains in `configLibrary` but is no longer active.

## 2026-05-09 Claude Desktop 3P Gateway Setup

- Claude Desktop custom 3P provider requires `baseUrl` to use HTTPS, except loopback HTTP is allowed. LAN HTTP such as `http://192.168.10.6:8787` fails with `baseUrl: must use https (or http on loopback)`.
- Working local Desktop config uses loopback:
  - `inferenceGatewayBaseUrl`: `http://127.0.0.1:8787`
  - `inferenceGatewayAuthScheme`: `bearer`
  - `inferenceModels`: JSON string containing the Claude aliases exposed by the gateway.
- Only the current official-style aliases are verified with Claude Desktop: `claude-sonnet-4.6`, `claude-opus-4.7`, `claude-haiku-4.5`, `claude-sonnet-4-6`, `claude-opus-4-7`, and `claude-haiku-4-5`.
- Custom readable aliases were initially unverified. On 2026-05-09, user manually verified Claude Desktop can use `claude-inclusionai/ring-2.6-1t:free`, where the real upstream model name is prefixed with `claude-`. Gateway route mapped it to `inclusionai/ring-2.6-1t:free`.
- The verified custom alias still contains `/` and `:` after the `claude-` prefix, so slash/colon characters are accepted in this tested case. Do not generalize to every provider/model without E2E.
- `anthropic/claude-*` aliases are also not verified in this project. Treat them as a hypothesis only.
- The config file consumed by Claude Desktop 3P is a flat managed-preferences JSON object, not a nested `enterpriseConfig` object.
- Relevant config paths:
  - `/Users/c/Library/Application Support/Claude-3p/configLibrary/00000000-0000-4000-8000-000000087870.json` is the fixed active profile written by `apply-local`.
  - `/Users/c/Library/Application Support/Claude-3p/configLibrary/779eec7e-4d53-46a9-974d-ffbfc7f6d01a.json`
  - `/Users/c/Library/Application Support/Claude-3p/claude_desktop_config.json`
  - `/Users/c/Library/Application Support/Claude/claude_desktop_config.json`
  - `/Users/c/Library/Application Support/Claude-3p/configLibrary/_meta.json` identifies the active `appliedId`.
- If Claude still shows old provider errors after editing config, fully quit Claude Desktop with `Cmd+Q` and reopen. The setup window can hold stale provider state.
- Local gateway verification passed:
  - `curl http://127.0.0.1:8787/health`
  - `curl -H 'Authorization: Bearer <gateway key>' http://127.0.0.1:8787/v1/models`
  - Three real `/v1/messages` calls through the gateway to OpenRouter completed successfully.
- Secrets must stay in environment variables or ignored local env files. Do not put provider keys or gateway API keys directly in JSON config.
- For LAN or VPS use, Claude Desktop needs trusted HTTPS. Preferred options are domain + Caddy/Nginx + Let's Encrypt, Cloudflare Tunnel, or Tailscale Funnel. Self-signed certs fail with `ERR_CERT_AUTHORITY_INVALID` unless the client OS trusts the certificate.

## 2026-05-09 Desktop Model Naming Contract

- Gateway JSON routes now support optional `displayName` on each route entry. This is returned as `display_name` from `/v1/models` and reserved for the GUI route editor.
- Route map keys are the Claude Desktop model IDs. `apply-local` derives `inferenceModels` from those route keys when `CLAUDE_GATEWAY_CONFIG` is set, while `--models` remains a manual override.
- Default GUI-generated desktop IDs should use `claude-` + upstream model unless the upstream model already starts with `claude-` or `anthropic/claude-`. This is a conservative implementation rule, not a universal Claude Desktop guarantee.
- `/v1/models` should expose desktop model IDs only, not duplicate raw upstream IDs, so the Desktop dropdown remains readable.

## 2026-05-09 Local Gateway Lifecycle Helper

- Added `scripts/local-gateway <start|stop|restart|status> [--dry-run]` for local background process management.
- The helper validates `.env.local` and `gateway.local.json`, builds a local binary under `.local-gateway/`, writes PID/log files there, and never prints secret values.
- `.local-gateway/` is gitignored. GUI lifecycle controls should reuse this start/stop/status behavior rather than inventing a separate process model.

## 2026-05-09 Provider Adapter Boundary

- Gateway message handling now selects an upstream adapter by provider profile. `openai-chat` is the supported profile for OpenRouter/OpenAI-compatible providers.
- Unsupported provider profiles return a local Anthropic-style 400 error (`provider profile is not supported`) and do not call the upstream server.
- OpenAI-compatible request execution and response conversion live behind `upstreamAdapter`, so future DeepSeek/NVIDIA/Cloudflare adapters should plug in there instead of adding provider-specific logic to `handleMessages`.

## 2026-05-09 Provider Capabilities

- Providers now carry `capabilities` metadata with `streaming`, `tools`, and `jsonMode`; config defaults keep streaming/tools enabled for OpenAI-compatible providers.
- If `streaming` or `tools` is disabled, `/v1/messages` rejects matching requests locally with Anthropic-style 400 errors before calling upstream.
- This preflight check prevents stream requests from silently falling through and failing after SSE output has started.

## 2026-05-09 Provider Research Notes

- Added official-source notes under `docs/providers/` for DeepSeek, NVIDIA, and Cloudflare.
- DeepSeek likely works through `openai-chat` with `baseUrl: https://api.deepseek.com`.
- NVIDIA hosted API catalog and self-hosted NIM use OpenAI-style `/v1/chat/completions`; hosted `baseUrl` should include `/v1`.
- Cloudflare Workers AI OpenAI-compatible base URL is account-specific and must be resolved in ignored local config; the loader does not expand `${...}` placeholders.
- These are research notes only. No provider is complete until mock tests and three real consecutive calls pass.
- On 2026-05-09, user redirected priority back to GUI MVP. Additional provider implementation is backlog and should not block the desktop configuration console.

## 2026-05-09 GUI MVP and RawBlock Direction

- GUI MVP uses Wails v2 with a static `frontend/` shell and Go bindings from the root `main` package.
- The first GUI slice is dashboard-only: gateway health/status, listen URL, config path/errors, provider summaries, route/model aliases, and Claude Desktop doctor state.
- The GUI reads safe config summaries and must not expose real API keys. It may show env var names such as `OPENROUTER_API_KEY` and `CLAUDE_GATEWAY_API_KEY`.
- RawBlock is the required visual direction: black/white, heavy square borders, no shadows, no rounded corners, uppercase labels/buttons, no decorative imagery, blue only for links.
- `frontend/wailsjs/` and `build/` are generated by Wails and ignored. Generated Wails TypeScript may contain `any`, so do not commit it unless replacing it with compliant hand-written bindings.
- Wails CLI is installed at `/Users/c/go/bin/wails` on this machine; it may not be on PATH.

## 2026-05-09 GUI OpenRouter Editor

- The Wails GUI now includes an OpenRouter-first editor slice:
  - `KEYS` can create/update/delete `OPENROUTER_API_KEY` and `CLAUDE_GATEWAY_API_KEY` in `.env.local`.
  - Secret values are never returned to the frontend; the UI only gets present/absent status.
  - `MODELS` can add/edit/delete route entries with desktop ID, display name, provider, and upstream model.
  - Default desktop ID generation still follows `claude-` + upstream unless upstream already starts with `claude-` or `anthropic/claude-`.
- Config saves are routed through Go validation and always write to the service's configured `gateway.local.json`; frontend-supplied paths are ignored.
- Saved JSON omits inline `apiKey` and `gatewayApiKey` fields entirely and keeps only `apiKeyEnv` / `gatewayApiKeyEnv`.
- Local packaged `.app` startup can discover the repository root by walking upward from the executable path, so direct `open build/bin/claude-desktop-gateway.app` uses the repo config instead of `/gateway.local.json`.
- Remaining GUI debt:
  - provider CRUD/test-provider is not complete;
  - gateway start/stop/restart controls are not wired;
  - Claude Desktop apply-local/doctor actions are still dashboard-only/read-only from the GUI;
  - keychain storage is deferred.

## 2026-05-09 GUI i18n Planning

- User requested an English / Simplified Chinese GUI i18n plan.
- Plan lives at `docs/plans/gui-i18n-bilingual.md`.
- Scope is GUI-only first; do not localize gateway API responses, config schema, model IDs, env var names, URLs, paths, logs, provider names, or issue codes.
- Preferred first implementation is a lightweight `frontend/i18n.js` dictionary with `en` and `zh-CN`, `localStorage` persistence, system-language fallback, and no new dependency.
- Add a RawBlock language switcher in the top bar. Verify both English and Chinese manually in Wails because RawBlock uppercase/heavy typography can cause CJK layout issues.

## 2026-05-09 GUI i18n Implementation

- Added `frontend/i18n.js` with `en` and `zh-CN` dictionaries, fallback, interpolation, locale detection, and `localStorage` persistence.
- `frontend/index.html` uses `data-i18n` / `data-i18n-attr` for static GUI copy and loads `frontend/main.js` as an ES module.
- `frontend/main.js` localizes dynamic copy including status badges, route counts, empty states, validation messages, confirmations, save/delete messages, key placeholders, and provider capability states.
- Technical identifiers remain untranslated: env var names, model IDs, provider names, URLs, file paths, profile IDs, and issue codes.
- Manual Wails checks passed for switching English -> Chinese -> English. A status badge regression was fixed by making dynamic status rendering own badge text after initial translation.
