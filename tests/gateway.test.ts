import { describe, expect, it } from "vitest";
import {
  DEFAULT_ALIAS_MODEL,
  DEFAULT_MODEL_ALIASES,
  resolveModelAlias,
  type GatewayConfig,
} from "../src/config.js";
import { createGatewayApp } from "../src/app.js";

interface FetchCall {
  input: string | URL | Request;
  init?: RequestInit;
}

function testConfig(overrides?: Partial<GatewayConfig>): GatewayConfig {
  return {
    host: "127.0.0.1",
    port: 8787,
    openRouterApiKey: "or-test-key",
    gatewayApiKey: "client-test-key",
    openRouterBaseUrl: "https://openrouter.ai/api/v1",
    defaultModel: DEFAULT_ALIAS_MODEL,
    aliases: { ...DEFAULT_MODEL_ALIASES },
    referrer: "https://codex-proxy.local",
    title: "Claude Desktop Gateway",
    ...overrides,
  };
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: init?.status ?? 200,
    headers: { "Content-Type": "application/json", ...Object.fromEntries(init?.headers ?? []) },
  });
}

function sseResponse(lines: string[]): Response {
  const encoder = new TextEncoder();
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoder.encode(lines.join("\n")));
      controller.close();
    },
  });
  return new Response(stream, {
    status: 200,
    headers: { "Content-Type": "text/event-stream" },
  });
}

function parseJsonBody(init: RequestInit | undefined): Record<string, unknown> {
  if (typeof init?.body !== "string") {
    throw new Error("expected string JSON request body");
  }
  const parsed = JSON.parse(init.body) as unknown;
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    throw new Error("expected object JSON request body");
  }
  return parsed as Record<string, unknown>;
}

function authHeaders(): HeadersInit {
  return {
    "Content-Type": "application/json",
    Authorization: "Bearer client-test-key",
  };
}

function messagesBody(overrides?: Record<string, unknown>): Record<string, unknown> {
  return {
    model: "claude-opus-4-7",
    max_tokens: 128,
    messages: [{ role: "user", content: "hello" }],
    stream: false,
    ...overrides,
  };
}

describe("gateway config", () => {
  it("maps Claude Desktop shell aliases to the default OpenRouter free model", () => {
    expect(DEFAULT_MODEL_ALIASES["claude-opus-4-7"]).toBe(DEFAULT_ALIAS_MODEL);
    expect(DEFAULT_MODEL_ALIASES["claude-sonnet-4-6"]).toBe(DEFAULT_ALIAS_MODEL);
    expect(DEFAULT_MODEL_ALIASES["claude-haiku-4-5"]).toBe(DEFAULT_ALIAS_MODEL);
    expect(resolveModelAlias("claude-opus-4-7", testConfig())).toBe(DEFAULT_ALIAS_MODEL);
  });

  it("passes through direct OpenRouter model ids", () => {
    expect(resolveModelAlias("openai/gpt-4o-mini", testConfig())).toBe("openai/gpt-4o-mini");
  });
});

describe("gateway app", () => {
  it("requires the configured client API key before proxying", async () => {
    const fetchCalls: FetchCall[] = [];
    const app = createGatewayApp(testConfig(), async (input, init) => {
      fetchCalls.push({ input, init });
      return jsonResponse({});
    });

    const res = await app.request("/v1/messages", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(messagesBody()),
    });

    expect(res.status).toBe(401);
    expect(await res.json()).toEqual({
      type: "error",
      error: {
        type: "authentication_error",
        message: "Invalid API key",
      },
    });
    expect(fetchCalls).toHaveLength(0);
  });

  it("lists Claude-shaped aliases for Claude Desktop setup", async () => {
    const app = createGatewayApp(testConfig(), async () => jsonResponse({}));

    const res = await app.request("/v1/models", {
      headers: authHeaders(),
    });

    expect(res.status).toBe(200);
    const body = await res.json() as { object: string; data: Array<{ id: string }> };
    expect(body.object).toBe("list");
    expect(body.data.map((model) => model.id)).toContain("claude-opus-4-7");
    expect(body.data.map((model) => model.id)).toContain(DEFAULT_ALIAS_MODEL);
  });

  it("translates non-streaming Anthropic messages requests to OpenRouter chat completions", async () => {
    const fetchCalls: FetchCall[] = [];
    const app = createGatewayApp(testConfig(), async (input, init) => {
      fetchCalls.push({ input, init });
      return jsonResponse({
        id: "gen-1",
        choices: [
          {
            message: { role: "assistant", content: "hello from openrouter" },
            finish_reason: "stop",
          },
        ],
        usage: { prompt_tokens: 3, completion_tokens: 5 },
      });
    });

    const res = await app.request("/v1/messages", {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify(messagesBody({ system: "You are direct.", temperature: 0.2 })),
    });

    expect(res.status).toBe(200);
    expect(fetchCalls).toHaveLength(1);
    const call = fetchCalls[0];
    expect(String(call.input)).toBe("https://openrouter.ai/api/v1/chat/completions");
    expect(call.init?.headers).toMatchObject({
      Authorization: "Bearer or-test-key",
      "HTTP-Referer": "https://codex-proxy.local",
      "X-Title": "Claude Desktop Gateway",
    });

    const upstreamBody = parseJsonBody(call.init);
    expect(upstreamBody.model).toBe(DEFAULT_ALIAS_MODEL);
    expect(upstreamBody.stream).toBe(false);
    expect(upstreamBody.max_tokens).toBe(128);
    expect(upstreamBody.temperature).toBe(0.2);
    expect(upstreamBody.messages).toEqual([
      { role: "system", content: "You are direct." },
      { role: "user", content: "hello" },
    ]);

    expect(await res.json()).toEqual({
      id: "gen-1",
      type: "message",
      role: "assistant",
      model: "claude-opus-4-7",
      content: [{ type: "text", text: "hello from openrouter" }],
      stop_reason: "end_turn",
      stop_sequence: null,
      usage: {
        input_tokens: 3,
        output_tokens: 5,
      },
    });
  });

  it("translates OpenRouter streaming deltas to Anthropic SSE events", async () => {
    const app = createGatewayApp(testConfig(), async () =>
      sseResponse([
        'data: {"id":"gen-stream","choices":[{"delta":{"content":"hel"},"finish_reason":null}]}',
        "",
        'data: {"choices":[{"delta":{"content":"lo"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":4}}',
        "",
        "data: [DONE]",
        "",
      ]),
    );

    const res = await app.request("/v1/messages", {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify(messagesBody({ stream: true })),
    });

    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toContain("text/event-stream");
    const text = await res.text();
    expect(text).toContain("event: message_start");
    expect(text).toContain('"id":"gen-stream"');
    expect(text).toContain("event: content_block_delta");
    expect(text).toContain('"text":"hel"');
    expect(text).toContain('"text":"lo"');
    expect(text).toContain("event: message_stop");
  });
});
