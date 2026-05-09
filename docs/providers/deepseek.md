# DeepSeek Provider Notes

## Status

Research only. Not implemented and no real E2E has passed yet.

## Official Sources

- First API call and OpenAI/Anthropic base URLs: https://api-docs.deepseek.com/
- Models and pricing: https://api-docs.deepseek.com/quick_start/pricing
- JSON output: https://api-docs.deepseek.com/guides/json_mode/
- Function calling: https://api-docs.deepseek.com/guides/function_calling/

## API Shape

DeepSeek documents an OpenAI-compatible base URL:

```text
https://api.deepseek.com
```

The current `openai-chat` adapter appends `/chat/completions`, so the gateway
provider `baseUrl` should be `https://api.deepseek.com`.

Auth uses bearer API key. Keep it in env, for example:

```json
{
  "providers": {
    "deepseek": {
      "profile": "openai-chat",
      "baseUrl": "https://api.deepseek.com",
      "apiKeyEnv": "DEEPSEEK_API_KEY",
      "capabilities": {
        "streaming": true,
        "tools": true,
        "jsonMode": true
      }
    }
  }
}
```

## Model Examples

Current docs list:

- `deepseek-v4-flash`
- `deepseek-v4-pro`
- `deepseek-chat` and `deepseek-reasoner`, with deprecation noted for
  `2026-07-24`

Suggested desktop route shape:

```json
{
  "routes": {
    "claude-deepseek/deepseek-v4-flash": [
      {
        "provider": "deepseek",
        "model": "deepseek-v4-flash",
        "displayName": "DeepSeek V4 Flash"
      }
    ]
  }
}
```

## Capability Notes

DeepSeek docs state OpenAI-compatible streaming, JSON output, and function
calling. Function calling strict mode requires the beta base URL and stricter
schemas, so do not enable strict mode by default.

## Implementation Notes

- The generic `openai-chat` adapter should work for normal chat completions.
- Reasoning-specific fields such as `thinking` and `reasoning_effort` are not
  represented by the Anthropic subset yet.
- Real support requires a `DEEPSEEK_API_KEY` and three consecutive real calls.
