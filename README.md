# Claude Desktop Gateway

Thin local inference gateway for Claude Desktop. It accepts Anthropic Messages
requests at `/v1/messages` and forwards them to OpenRouter Chat Completions.

## Run Go Core

The Go core is the new MVP path for the desktop gateway. It keeps the same
Anthropic-compatible HTTP surface while preparing for a Wails configuration GUI.

For local Claude Desktop testing, use the repeatable script entrypoint:

```bash
cp .env.local.example .env.local
cp gateway.local.example.json gateway.local.json
# Edit .env.local and replace OPENROUTER_API_KEY.
./scripts/run-local
```

The script loads `.env.local`, uses `gateway.local.json`, requires
`OPENROUTER_API_KEY` and `CLAUDE_GATEWAY_API_KEY`, and starts the gateway on
`http://127.0.0.1:8787`. It prints only variable names and file paths, not
secret values. To validate local setup without starting the server:

```bash
./scripts/run-local --dry-run
```

Verify a running local gateway:

```bash
curl http://127.0.0.1:8787/health
curl -H "Authorization: Bearer $CLAUDE_GATEWAY_API_KEY" http://127.0.0.1:8787/v1/models
```

For normal Claude Desktop use, run the gateway in the background:

```bash
scripts/local-gateway start
scripts/local-gateway status
scripts/local-gateway stop
```

`scripts/local-gateway start` validates `.env.local` and `gateway.local.json`,
builds a local binary into `.local-gateway/`, starts it on
`http://127.0.0.1:8787`, and writes logs to `.local-gateway/gateway.log`. Use
`scripts/local-gateway restart` after changing config. The state directory is
gitignored and must not contain committed artifacts.

Diagnose the Claude Desktop 3P files that are actually active:

```bash
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config doctor
```

The doctor reads `Claude-3p/configLibrary/_meta.json`, opens the active
profile from `appliedId`, checks flat gateway fields, and reports mismatched
URLs such as stale LAN HTTP values. It never prints API key values.

Repair or apply the local Claude Desktop 3P profile:

```bash
source .env.local
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config apply-local
```

`apply-local` writes a fixed profile in `Claude-3p/configLibrary/`, updates
`_meta.json` so `appliedId` points to it, and sets both Claude root config files
to `deploymentMode: "3p"`. It uses `CLAUDE_GATEWAY_API_KEY` from env for
Claude Desktop's bearer token and redacts it from output. When
`CLAUDE_GATEWAY_CONFIG` is set, it derives `inferenceModels` from the gateway
routes. Use `--models` only for manual override or experiments. Preview without
writing:

```bash
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config apply-local --dry-run
```

Manual env-only startup is also supported:

```bash
export OPENROUTER_API_KEY="..."
export CLAUDE_GATEWAY_API_KEY="local-client-key"
go run ./cmd/gateway
```

Or use a local JSON config file. `gateway.local.json` is ignored by git:

```bash
export CLAUDE_GATEWAY_CONFIG=gateway.local.json
go run ./cmd/gateway
```

Use `gateway.local.example.json` as the shape reference. JSON config must not
contain secrets; use `apiKeyEnv` to reference an environment variable. Route
keys are the Claude Desktop model IDs. `displayName` is optional metadata for
`/v1/models` and the future GUI route editor.

For local-only secrets without the script, copy `.env.local.example` to
`.env.local`, edit it, and source it before running Go directly:

```bash
source .env.local
export CLAUDE_GATEWAY_CONFIG=gateway.local.json
go run ./cmd/gateway
```

## LAN or VPS Binding

For another machine to reach the gateway, bind to all interfaces:

```bash
cp gateway.lan.example.json gateway.lan.json
source .env.local
export CLAUDE_GATEWAY_CONFIG=gateway.lan.json
go run ./cmd/gateway
```

`gateway.lan.json` uses:

```json
{
  "host": "0.0.0.0",
  "port": 8787,
  "gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
  "tlsCertFile": "certs/gateway.crt",
  "tlsKeyFile": "certs/gateway.key"
}
```

When binding to any non-loopback host, the Go core requires
`CLAUDE_GATEWAY_API_KEY`; startup fails without it. Claude Desktop's third-party
gateway flow uses HTTPS, so LAN mode should run with TLS. From another LAN
machine, set Claude Desktop's gateway URL to `https://<server-lan-ip>:8787`.

For local LAN certificates, create a certificate with subject alternative names
for the server IP and trust it on the Claude Desktop machine. Example with
OpenSSL:

```bash
mkdir -p certs
cat > certs/gateway-openssl.cnf <<'EOF'
[req]
distinguished_name=req_distinguished_name
x509_extensions=v3_req
prompt=no

[req_distinguished_name]
CN=192.168.10.6

[v3_req]
subjectAltName=@alt_names

[alt_names]
IP.1=192.168.10.6
IP.2=127.0.0.1
DNS.1=localhost
EOF

openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/gateway.key \
  -out certs/gateway.crt \
  -config certs/gateway-openssl.cnf
```

Self-signed certificates must be trusted by the client OS before Claude Desktop
will accept them.

On macOS, run this manually in Terminal or import `certs/gateway.crt` through
Keychain Access and set it to Always Trust:

```bash
security add-trusted-cert -d -r trustRoot \
  -k ~/Library/Keychains/login.keychain-db \
  certs/gateway.crt
```

For a public VPS, do not expose plain HTTP directly. Put the gateway behind
HTTPS, firewall the port, and use a strong `CLAUDE_GATEWAY_API_KEY`.

Run local Go tests:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./...
```

## Run Desktop GUI

The GUI MVP is a Wails desktop shell that reads the existing Go/config state
instead of duplicating gateway protocol logic. It currently shows gateway
health, listen URL, config errors, providers, route aliases, and Claude Desktop
doctor status. Start/stop controls and route editing are next-phase work.

Launch the desktop app directly:

```bash
GOCACHE=/private/tmp/go-build-cache go run .
```

Build a signed local macOS app bundle:

```bash
GOCACHE=/private/tmp/go-build-cache /Users/c/go/bin/wails build
```

If `wails` is on your PATH, `wails build` is equivalent. The UI follows the
RawBlock direction: black/white, thick square borders, no shadows, no rounded
corners, uppercase controls, and no decorative imagery.

## Run TypeScript Reference

The TypeScript service is kept as a behavior reference during the Go rewrite.

```bash
export OPENROUTER_API_KEY="..."
export CLAUDE_GATEWAY_API_KEY="local-client-key"
npm run dev
```

Default URL: `http://127.0.0.1:8787`

## Claude Desktop

```json
{
  "inferenceProvider": "gateway",
  "inferenceGatewayBaseUrl": "http://127.0.0.1:8787",
  "inferenceGatewayApiKey": "local-client-key",
  "inferenceGatewayAuthScheme": "bearer",
  "inferenceModels": [
    "claude-inclusionai/ring-2.6-1t:free"
  ]
}
```

## Model Routes

The JSON route shape keeps Claude Desktop naming separate from the real
upstream model:

```json
{
  "providers": {
    "openrouter": {
      "profile": "openai-chat",
      "baseUrl": "https://openrouter.ai/api/v1",
      "apiKeyEnv": "OPENROUTER_API_KEY",
      "capabilities": {
        "streaming": true,
        "tools": true,
        "jsonMode": false
      }
    }
  },
  "routes": {
    "claude-inclusionai/ring-2.6-1t:free": [
      {
        "provider": "openrouter",
        "model": "inclusionai/ring-2.6-1t:free",
        "displayName": "OpenRouter Ring 2.6 1T Free"
      }
    ]
  }
}
```

The route key is what Claude Desktop sends back to `/v1/messages`; `model` is
the provider model sent upstream. If a GUI adds a route from an upstream model,
the default desktop ID is the upstream model prefixed with `claude-`, unless it
already starts with `claude-` or `anthropic/claude-`. The only custom ID verified
with Claude Desktop so far is `claude-inclusionai/ring-2.6-1t:free`.

Provider `capabilities` default to streaming and tools enabled. Set
`"streaming": false` or `"tools": false` for providers that cannot support
those request shapes; the gateway rejects unsupported requests before calling
upstream.

Without a JSON config, the env-only fallback keeps these Claude-shaped aliases
mapped to OpenRouter's current free 1T model:

```text
claude-opus-4-7   -> inclusionai/ring-2.6-1t:free
claude-opus-4.7   -> inclusionai/ring-2.6-1t:free
claude-sonnet-4-6 -> inclusionai/ring-2.6-1t:free
claude-sonnet-4.6 -> inclusionai/ring-2.6-1t:free
claude-haiku-4-5  -> inclusionai/ring-2.6-1t:free
claude-haiku-4.5  -> inclusionai/ring-2.6-1t:free
```

Override with JSON:

```bash
export CLAUDE_MODEL_ALIASES='{"claude-opus-4-7":"provider/model-id"}'
```

Or comma-separated pairs:

```bash
export CLAUDE_MODEL_ALIASES='claude-opus-4-7=provider/model-id,claude-haiku-4-5=provider/other-model-id'
```

## Environment

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENROUTER_API_KEY` | required | OpenRouter upstream API key |
| `CLAUDE_GATEWAY_API_KEY` | none | Optional client key required by Claude Desktop |
| `CLAUDE_GATEWAY_DEFAULT_MODEL` | `inclusionai/ring-2.6-1t:free` | Default target for built-in aliases |
| `CLAUDE_MODEL_ALIASES` | built-ins | JSON object or comma-separated alias map |
| `OPENROUTER_BASE_URL` | `https://openrouter.ai/api/v1` | OpenRouter-compatible base URL |
| `OPENROUTER_REFERRER` | none | Optional OpenRouter attribution header |
| `OPENROUTER_TITLE` | `Codex Proxy Claude Gateway` | Optional OpenRouter attribution header |
| `HOST` | `127.0.0.1` | Listen host |
| `PORT` | `8787` | Listen port |
| `CLAUDE_GATEWAY_CONFIG` | none | Optional non-secret JSON config file path, for example `gateway.local.json` |

## Real OpenRouter Test

After exporting `OPENROUTER_API_KEY`, run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/gateway -run TestRealOpenRouterCompletesThreeSequentialCalls -count=1
```

The test performs three sequential calls against `inclusionai/ring-2.6-1t:free`.

## Claude Desktop E2E

See `docs/e2e/claude-desktop.md` for the full Claude Desktop configuration and
manual end-to-end checklist.
