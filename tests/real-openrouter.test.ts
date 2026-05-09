import { describe, expect, it } from "vitest";
import { createGatewayApp } from "../src/app.js";
import { DEFAULT_ALIAS_MODEL, loadConfigFromEnv } from "../src/config.js";

const hasOpenRouterKey = Boolean(process.env.OPENROUTER_API_KEY?.trim());
const runWhenConfigured = hasOpenRouterKey ? describe : describe.skip;

runWhenConfigured("Real: Claude Desktop gateway via OpenRouter", () => {
  it("completes three sequential Anthropic messages calls against OpenRouter", async () => {
    const gatewayApiKey = "real-gateway-test";
    const config = loadConfigFromEnv({
      ...process.env,
      CLAUDE_GATEWAY_API_KEY: gatewayApiKey,
      CLAUDE_GATEWAY_DEFAULT_MODEL: DEFAULT_ALIAS_MODEL,
      OPENROUTER_TITLE: "Claude Desktop Gateway Real Test",
    });
    const app = createGatewayApp(config);

    for (let i = 1; i <= 3; i++) {
      const res = await app.request("/v1/messages", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${gatewayApiKey}`,
        },
        body: JSON.stringify({
          model: "claude-opus-4-7",
          max_tokens: 96,
          messages: [
            {
              role: "user",
              content: `Reply with exactly: gateway-ok-${i}`,
            },
          ],
          stream: false,
          temperature: 0,
        }),
      });

      expect(res.status).toBe(200);
      const body = await res.json() as {
        content?: Array<{ type?: string; text?: string }>;
        usage?: { input_tokens?: number; output_tokens?: number };
      };
      const text = body.content?.map((block) => block.text ?? "").join("");
      expect(text).toContain(`gateway-ok-${i}`);
      expect(body.usage?.input_tokens ?? 0).toBeGreaterThan(0);
      expect(body.usage?.output_tokens ?? 0).toBeGreaterThan(0);
    }
  }, 120_000);
});
