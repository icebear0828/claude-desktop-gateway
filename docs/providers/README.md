# Provider Notes

These notes capture official provider API shapes before implementation. A
provider is not considered supported until mock tests and three consecutive real
calls pass.

Current adapter profile:

- `openai-chat`: sends `POST {baseUrl}/chat/completions` with OpenAI-compatible
  chat-completions JSON and bearer auth.

Planned provider notes:

- [DeepSeek](deepseek.md)
- [NVIDIA](nvidia.md)
- [Cloudflare](cloudflare.md)
