# Claude Desktop E2E Checklist

## Scope

This verifies the full path:

```text
Claude Desktop -> local gateway -> OpenRouter -> Ring-2.6-1T free
```

The existing Go real test only verifies:

```text
Go gateway test client -> local gateway -> OpenRouter
```

## Start the Gateway

Use env for secrets and JSON for non-secret routing:

```bash
source .env.local
export CLAUDE_GATEWAY_CONFIG=gateway.local.json
GOCACHE=/private/tmp/go-build-cache go run ./cmd/gateway
```

Expected local URL:

```text
http://127.0.0.1:8787
```

For day-to-day local Desktop testing, prefer the managed background helper:

```bash
scripts/local-gateway start
scripts/local-gateway status
scripts/local-gateway stop
```

It keeps PID and logs under `.local-gateway/`. Restart it after editing
`gateway.local.json`.

Before changing Claude Desktop again, run the config doctor:

```bash
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config doctor
```

The doctor follows `Claude-3p/configLibrary/_meta.json` -> `appliedId` to find
the active profile, then validates that the active profile uses flat
`inferenceGateway*` fields. This catches the common mistake where a correct
profile exists but `_meta.json` still applies an older profile, or where a
stale LAN HTTP URL remains active.

To repair the local profile automatically:

```bash
source .env.local
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config apply-local
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config doctor
```

`apply-local` writes a fixed local gateway profile, updates `_meta.json` so that
profile is active, and writes `deploymentMode: "3p"` to both Claude root config
files. It reads `CLAUDE_GATEWAY_API_KEY` from env and does not print the value.
When `CLAUDE_GATEWAY_CONFIG` is set, it writes `inferenceModels` from the
gateway route keys. Fully quit Claude Desktop with `Cmd+Q` after applying.

For LAN testing from another computer, use `gateway.lan.json` and configure:

```text
https://<server-lan-ip>:8787
```

The server will refuse to start on `0.0.0.0` unless `CLAUDE_GATEWAY_API_KEY`
is set in env. Claude Desktop expects HTTPS for third-party gateway setup, so
the certificate must be trusted by the Claude Desktop machine.

On macOS, trust the generated certificate manually:

```bash
security add-trusted-cert -d -r trustRoot \
  -k ~/Library/Keychains/login.keychain-db \
  certs/gateway.crt
```

## Claude Desktop Configuration

Anthropic's current 3P setup flow recommends configuring through the in-app
window:

1. Launch Claude Desktop without signing in.
2. Enable developer mode:
   - macOS: `Help -> Troubleshooting -> Enable Developer Mode`
   - Windows: application menu `☰ -> Help -> Troubleshooting -> Enable Developer Mode`
3. Open `Developer -> Configure third-party inference`.
4. In `Connection`, set:
   - Inference provider: `Gateway`
   - Gateway base URL: `http://127.0.0.1:8787`
   - Gateway API key: `local-client-key`
   - Gateway auth scheme: `bearer`
5. Models can be omitted if Claude auto-discovers `/v1/models`; otherwise set
   the route key from `gateway.local.json`:

```json
[
  "claude-ring-2-6-1t-free"
]
```

6. Click `Apply locally`.
7. Fully quit Claude Desktop with `Cmd+Q`, then reopen it. The setup window can
   keep stale provider state after config changes.

## HTTPS Requirement

The current Anthropic docs for Cowork on 3P require HTTPS for managed gateway
configuration. LAN mode supports TLS through `tlsCertFile` and `tlsKeyFile` in
`gateway.lan.json`.

## Manual E2E Prompt

After applying the config, start a Cowork/3P session and send:

```text
Reply exactly: desktop-gateway-ok
```

Pass criteria:

- Claude Desktop sends at least one request to the local gateway.
- The response contains `desktop-gateway-ok`.
- No prompt/content is written to gateway logs.
- If the app fails before calling the gateway, inspect:
  - macOS: `~/Library/Logs/Claude/main.log`
  - Windows: `%APPDATA%\Claude\logs\main.log`

## Model ID Notes

Verified on 2026-05-09:

```text
claude-inclusionai/ring-2.6-1t:free
```

Claude Desktop accepted this custom model ID when it was written to
`inferenceModels`, and the gateway mapped it to the real upstream model:

```text
inclusionai/ring-2.6-1t:free
```

This proves that one `claude-` prefixed upstream-style model ID works in the
current Desktop setup, including `/` and `:` characters. It does not prove that
every custom `claude-*` or `anthropic/claude-*` model ID is accepted.

The gateway config keeps naming explicit:

```json
{
  "providers": {
    "openrouter": {
      "profile": "anthropic-messages",
      "baseUrl": "https://openrouter.ai/api/v1",
      "apiKeyEnv": "OPENROUTER_API_KEY"
    }
  },
  "routes": {
    "claude-ring-2-6-1t-free": [
      {
        "provider": "openrouter",
        "model": "inclusionai/ring-2.6-1t:free",
        "displayName": "OpenRouter Ring 2.6 1T Free",
        "cache": {
          "enabled": true,
          "ttlSeconds": 300
        }
      }
    ],
    "claude-free-agent": [
      {
        "provider": "openrouter",
        "model": "openrouter/free",
        "displayName": "OpenRouter Free Agent Auto",
        "dynamicFreeModels": {
          "enabled": true,
          "requiredParameters": ["tools", "tool_choice"],
          "minContextLength": 32768,
          "maxModels": 4,
          "catalogCacheTTLSeconds": 900,
          "fallback": [
            "inclusionai/ring-2.6-1t:free",
            "qwen/qwen3-coder:free",
            "z-ai/glm-4.5-air:free",
            "openai/gpt-oss-120b:free",
            "openrouter/free"
          ]
        },
        "cache": {
          "enabled": true,
          "ttlSeconds": 300
        }
      }
    ]
  }
}
```

`displayName` is returned by `/v1/models` and reserved for the GUI route editor.
Claude Desktop still uses the route key as the model ID.

For OpenRouter, prefer `profile: "anthropic-messages"` over `openai-chat` when
testing Claude Desktop compatibility. The Anthropic Messages profile rewrites
only the model ID and forwards Anthropic-native fields such as `tools`,
`tool_result`, `cache_control`, and `thinking` to OpenRouter's `/messages`
endpoint.

If a route has multiple upstream entries, the gateway tries them in order and
falls back on 402 provider spend-limit errors, 429, 5xx, or transport failures.
Authentication and validation errors still stop immediately so bad keys or bad
request shapes are visible.

For free defaults, prefer exposing a small set of desktop aliases:

```text
claude-ring-2-6-1t-free
claude-free-auto
claude-free-agent
claude-free-coder
claude-free-fast
```

`claude-ring-2-6-1t-free` is the dedicated Ring route for users who need that
specific model. `claude-free-auto` can point directly at `openrouter/free`. The
other aliases should use `dynamicFreeModels` so the gateway refreshes
OpenRouter's free model catalog at runtime and uses the hardcoded list only as
fallback. Keep `cache.enabled` on those routes so OpenRouter response caching is
requested, and verify `X-OpenRouter-Cache-Status` when diagnosing cache hit
rate.
