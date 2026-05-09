import type { GatewayConfig } from "./types.js";

export type { GatewayConfig } from "./types.js";

export const DEFAULT_ALIAS_MODEL = "inclusionai/ring-2.6-1t:free";

export const DEFAULT_MODEL_ALIASES: Readonly<Record<string, string>> = Object.freeze({
  "claude-opus-4-7": DEFAULT_ALIAS_MODEL,
  "claude-sonnet-4-6": DEFAULT_ALIAS_MODEL,
  "claude-haiku-4-5": DEFAULT_ALIAS_MODEL,
});

const DEFAULT_HOST = "127.0.0.1";
const DEFAULT_PORT = 8787;
const DEFAULT_BASE_URL = "https://openrouter.ai/api/v1";
const DEFAULT_TITLE = "Codex Proxy Claude Gateway";

function parsePort(value: string | undefined): number {
  if (!value) return DEFAULT_PORT;
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : DEFAULT_PORT;
}

function isStringRecord(value: unknown): value is Record<string, string> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
  return Object.values(value).every((item) => typeof item === "string");
}

export function parseAliasEnv(value: string | undefined, defaultModel: string): Record<string, string> {
  const base = {
    "claude-opus-4-7": defaultModel,
    "claude-sonnet-4-6": defaultModel,
    "claude-haiku-4-5": defaultModel,
  };
  if (!value?.trim()) return base;

  try {
    const parsed = JSON.parse(value) as unknown;
    if (isStringRecord(parsed)) return { ...base, ...parsed };
  } catch {
    // Fall through to comma-separated KEY=VALUE parsing.
  }

  const aliases: Record<string, string> = { ...base };
  for (const pair of value.split(",")) {
    const [rawKey, ...rawValueParts] = pair.split("=");
    const key = rawKey?.trim();
    const mapped = rawValueParts.join("=").trim();
    if (key && mapped) aliases[key] = mapped;
  }
  return aliases;
}

export function resolveModelAlias(model: string, config: GatewayConfig): string {
  const trimmed = model.trim();
  return config.aliases[trimmed] ?? trimmed;
}

export function loadConfigFromEnv(env: NodeJS.ProcessEnv = process.env): GatewayConfig {
  const defaultModel = env.CLAUDE_GATEWAY_DEFAULT_MODEL?.trim() || DEFAULT_ALIAS_MODEL;
  const openRouterApiKey = env.OPENROUTER_API_KEY?.trim() ?? "";
  return {
    host: env.HOST?.trim() || DEFAULT_HOST,
    port: parsePort(env.PORT),
    openRouterApiKey,
    gatewayApiKey: env.CLAUDE_GATEWAY_API_KEY?.trim() || null,
    openRouterBaseUrl: env.OPENROUTER_BASE_URL?.trim() || DEFAULT_BASE_URL,
    defaultModel,
    aliases: parseAliasEnv(env.CLAUDE_MODEL_ALIASES, defaultModel),
    referrer: env.OPENROUTER_REFERRER?.trim() || null,
    title: env.OPENROUTER_TITLE?.trim() || DEFAULT_TITLE,
  };
}
