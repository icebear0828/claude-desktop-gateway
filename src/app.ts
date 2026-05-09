import { Hono } from "hono";
import type { StatusCode } from "hono/utils/http-status";
import type {
  AnthropicErrorBody,
  AnthropicErrorType,
  GatewayConfig,
  OpenRouterChatCompletion,
} from "./types.js";
import { parseAnthropicMessagesRequest, toAnthropicMessagesResponse, toOpenRouterChatRequest } from "./translation.js";
import { openRouterStreamToAnthropic } from "./stream.js";

export type FetchLike = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

function makeError(type: AnthropicErrorType, message: string): AnthropicErrorBody {
  return { type: "error", error: { type, message } };
}

function errorTypeForStatus(status: number): AnthropicErrorType {
  if (status === 401) return "authentication_error";
  if (status === 403) return "permission_error";
  if (status === 404) return "not_found_error";
  if (status === 429) return "rate_limit_error";
  if (status === 400 || status === 422) return "invalid_request_error";
  if (status === 529) return "overloaded_error";
  return "api_error";
}

function statusCode(status: number): StatusCode {
  if (status >= 400 && status <= 599) return status as StatusCode;
  return 500;
}

function bearerToken(value: string | undefined): string | null {
  if (!value) return null;
  const prefix = "Bearer ";
  return value.startsWith(prefix) ? value.slice(prefix.length) : null;
}

function hasValidGatewayAuth(headers: Headers, config: GatewayConfig): boolean {
  if (!config.gatewayApiKey) return true;
  const xApiKey = headers.get("x-api-key");
  const bearer = bearerToken(headers.get("authorization") ?? undefined);
  return xApiKey === config.gatewayApiKey || bearer === config.gatewayApiKey;
}

async function upstreamError(response: Response): Promise<string> {
  try {
    const body = await response.json() as unknown;
    if (typeof body === "object" && body !== null && "error" in body) {
      const error = (body as { error?: unknown }).error;
      if (typeof error === "object" && error !== null && "message" in error) {
        const message = (error as { message?: unknown }).message;
        if (typeof message === "string") return message;
      }
      if (typeof error === "string") return error;
    }
  } catch {
    // Fall back to status text below.
  }
  return response.statusText || "OpenRouter request failed";
}

function openRouterHeaders(config: GatewayConfig): Record<string, string> {
  const headers: Record<string, string> = {
    Authorization: `Bearer ${config.openRouterApiKey}`,
    "Content-Type": "application/json",
  };
  if (config.referrer) headers["HTTP-Referer"] = config.referrer;
  if (config.title) headers["X-Title"] = config.title;
  return headers;
}

function modelsList(config: GatewayConfig): { object: "list"; data: Array<{ id: string; object: "model"; created: number; owned_by: string }> } {
  const ids = new Set<string>([...Object.keys(config.aliases), config.defaultModel]);
  return {
    object: "list",
    data: [...ids].map((id) => ({
      id,
      object: "model",
      created: 1700000000,
      owned_by: "openrouter",
    })),
  };
}

export function createGatewayApp(config: GatewayConfig, fetchImpl: FetchLike = fetch): Hono {
  const app = new Hono();

  app.get("/health", (c) => c.json({ ok: true }));

  app.use("/v1/*", async (c, next) => {
    if (!hasValidGatewayAuth(c.req.raw.headers, config)) {
      c.status(401);
      return c.json(makeError("authentication_error", "Invalid API key"));
    }
    await next();
  });

  app.get("/v1/models", (c) => c.json(modelsList(config)));

  app.post("/v1/messages", async (c) => {
    if (!config.openRouterApiKey) {
      c.status(500);
      return c.json(makeError("api_error", "OPENROUTER_API_KEY is required"));
    }

    let rawBody: unknown;
    try {
      rawBody = await c.req.json();
    } catch {
      c.status(400);
      return c.json(makeError("invalid_request_error", "Invalid JSON in request body"));
    }

    const req = parseAnthropicMessagesRequest(rawBody);
    if (typeof req === "string") {
      c.status(400);
      return c.json(makeError("invalid_request_error", req));
    }

    const upstreamBody = toOpenRouterChatRequest(req, config);
    const upstream = await fetchImpl(`${config.openRouterBaseUrl.replace(/\/$/, "")}/chat/completions`, {
      method: "POST",
      headers: openRouterHeaders(config),
      body: JSON.stringify(upstreamBody),
    });

    if (!upstream.ok) {
      const message = await upstreamError(upstream);
      c.status(statusCode(upstream.status));
      return c.json(makeError(errorTypeForStatus(upstream.status), message));
    }

    if (req.stream) {
      return openRouterStreamToAnthropic(upstream, req.model);
    }

    const response = await upstream.json() as OpenRouterChatCompletion;
    return c.json(toAnthropicMessagesResponse(response, req.model));
  });

  return app;
}
