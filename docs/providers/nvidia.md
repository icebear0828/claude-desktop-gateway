# NVIDIA Provider Notes

## Status

Research only. Not implemented and no real E2E has passed yet.

## Official Sources

- Hosted NVIDIA API catalog LLM endpoint: https://docs.api.nvidia.com/nim/reference/llm-apis
- API catalog quickstart: https://docs.api.nvidia.com/nim/docs/api-quickstart
- NIM LLM 2.0.3 API reference: https://docs.nvidia.com/nim/large-language-models/2.0.3/reference/api-reference.html
- NIM run-anywhere options: https://docs.api.nvidia.com/nim/docs/run-anywhere

## API Shape

NVIDIA has two relevant modes:

- Hosted API catalog endpoint: `https://integrate.api.nvidia.com/v1`
- Self-hosted NIM default endpoint: `http://localhost:8000/v1`

Both expose OpenAI-style chat completions at `/v1/chat/completions`. The current
gateway adapter appends `/chat/completions`, so configure `baseUrl` with the
trailing `/v1`.

```json
{
  "providers": {
    "nvidia": {
      "profile": "openai-chat",
      "baseUrl": "https://integrate.api.nvidia.com/v1",
      "apiKeyEnv": "NVIDIA_API_KEY",
      "capabilities": {
        "streaming": true,
        "tools": false,
        "jsonMode": false
      }
    }
  }
}
```

## Model Examples

Official docs show both hosted catalog models and self-hosted NIM models. Use
the exact model ID from the selected model page or `/v1/models`.

Examples seen in docs:

- `meta/llama-3.1-8b-instruct`
- `openai/gpt-oss-20b`
- `openai/gpt-oss-120b`
- `qwen/qwen3-coder-480b-a35b-instruct`

Suggested route shape:

```json
{
  "routes": {
    "claude-nvidia/qwen3-coder": [
      {
        "provider": "nvidia",
        "model": "qwen/qwen3-coder-480b-a35b-instruct",
        "displayName": "NVIDIA Qwen3 Coder"
      }
    ]
  }
}
```

## Capability Notes

NIM LLM docs describe `/v1/chat/completions` as supporting streaming and tool
calling. Hosted catalog support can vary by model, so start with `tools: false`
until a specific model passes mock and real tool-call tests.

## Implementation Notes

- The generic `openai-chat` adapter should work for chat completions.
- Self-hosted NIM may not require bearer auth in local examples, but the gateway
  config currently requires a provider API key. Use a harmless env value only
  for local NIM after adding explicit support for authless local providers.
- Real support requires a valid NVIDIA/API catalog key and three consecutive
  real calls for the chosen model.
