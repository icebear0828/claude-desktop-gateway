# Cloudflare Provider Notes

## Status

Research only. Not implemented and no real E2E has passed yet.

## Official Sources

- Workers AI OpenAI-compatible API: https://developers.cloudflare.com/workers-ai/configuration/open-ai-compatibility/
- Workers AI text generation models: https://developers.cloudflare.com/workers-ai/models/text-generation/
- Workers AI streaming: https://developers.cloudflare.com/workers-ai/configuration/response-streaming/
- AI Gateway OpenAI-compatible endpoints: https://developers.cloudflare.com/ai-gateway/providers/openai-compatible-endpoints/

## API Shape

Workers AI exposes OpenAI-compatible chat completions under an account-specific
base URL:

```text
https://api.cloudflare.com/client/v4/accounts/<account_id>/ai/v1
```

The current gateway adapter appends `/chat/completions`, so configure `baseUrl`
with the trailing `/ai/v1`.

```json
{
  "providers": {
    "cloudflare-workers-ai": {
      "profile": "openai-chat",
      "baseUrl": "https://api.cloudflare.com/client/v4/accounts/${CLOUDFLARE_ACCOUNT_ID}/ai/v1",
      "apiKeyEnv": "CLOUDFLARE_API_TOKEN",
      "capabilities": {
        "streaming": true,
        "tools": false,
        "jsonMode": false
      }
    }
  }
}
```

Do not literally commit `${CLOUDFLARE_ACCOUNT_ID}` in a working config unless
the loader has variable expansion; today it does not. Write the resolved account
URL in ignored local JSON.

Cloudflare AI Gateway also offers OpenAI-compatible routing:

```text
https://gateway.ai.cloudflare.com/v1/<account_id>/<gateway_id>/compat
```

For AI Gateway, the model name includes the routed provider, such as
`deepseek-ai/deepseek-chat`.

## Model Examples

Workers AI model IDs include:

- `@cf/meta/llama-3.1-8b-instruct`
- `@cf/openai/gpt-oss-120b`
- `@cf/qwen/qwen2.5-coder-32b-instruct`

Suggested route shape:

```json
{
  "routes": {
    "claude-cloudflare/meta-llama-3.1-8b": [
      {
        "provider": "cloudflare-workers-ai",
        "model": "@cf/meta/llama-3.1-8b-instruct",
        "displayName": "Cloudflare Llama 3.1 8B"
      }
    ]
  }
}
```

## Capability Notes

Workers AI documents streaming support. Tool calling and JSON mode are
model/provider-specific and should stay disabled until verified for a specific
model.

## Implementation Notes

- The generic `openai-chat` adapter should work for Workers AI chat
  completions if the account URL is resolved in config.
- AI Gateway can front multiple providers; capabilities should be treated as
  route/provider-specific, not globally guaranteed.
- Real support requires a Cloudflare account ID, API token, and three
  consecutive real calls.
