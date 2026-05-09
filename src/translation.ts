import { resolveModelAlias } from "./config.js";
import type {
  AnthropicMessage,
  AnthropicMessagesRequest,
  AnthropicMessagesResponse,
  GatewayConfig,
  OpenRouterChatCompletion,
  OpenRouterChatRequest,
  OpenRouterMessage,
  OpenRouterTool,
  OpenRouterUsage,
} from "./types.js";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isAnthropicMessage(value: unknown): value is AnthropicMessage {
  if (!isRecord(value)) return false;
  if (value.role !== "user" && value.role !== "assistant") return false;
  return typeof value.content === "string" || Array.isArray(value.content);
}

export function parseAnthropicMessagesRequest(value: unknown): AnthropicMessagesRequest | string {
  if (!isRecord(value)) return "Request body must be a JSON object";
  if (typeof value.model !== "string" || !value.model.trim()) return "model is required";
  if (typeof value.max_tokens !== "number" || !Number.isFinite(value.max_tokens) || value.max_tokens <= 0) {
    return "max_tokens must be a positive number";
  }
  if (!Array.isArray(value.messages) || !value.messages.every(isAnthropicMessage)) {
    return "messages must be an array of user/assistant messages";
  }

  return {
    model: value.model,
    max_tokens: Math.floor(value.max_tokens),
    messages: value.messages,
    system: parseSystem(value.system),
    stream: typeof value.stream === "boolean" ? value.stream : false,
    temperature: typeof value.temperature === "number" ? value.temperature : undefined,
    top_p: typeof value.top_p === "number" ? value.top_p : undefined,
    top_k: typeof value.top_k === "number" ? value.top_k : undefined,
    stop_sequences: Array.isArray(value.stop_sequences)
      ? value.stop_sequences.filter((item): item is string => typeof item === "string")
      : undefined,
    tools: Array.isArray(value.tools) ? value.tools : undefined,
    tool_choice: value.tool_choice,
  };
}

function parseSystem(value: unknown): AnthropicMessagesRequest["system"] {
  if (typeof value === "string") return value;
  if (!Array.isArray(value)) return undefined;
  const blocks = value.filter((item): item is { type: "text"; text: string } =>
    isRecord(item) && item.type === "text" && typeof item.text === "string",
  );
  return blocks.length > 0 ? blocks : undefined;
}

function systemToText(system: AnthropicMessagesRequest["system"]): string | null {
  if (!system) return null;
  if (typeof system === "string") return system;
  const text = system.map((block) => block.text).filter(Boolean).join("\n\n");
  return text || null;
}

function contentToText(content: AnthropicMessage["content"]): string {
  if (typeof content === "string") return content;
  const parts: string[] = [];
  for (const block of content) {
    if (block.type === "text" && typeof block.text === "string") {
      parts.push(block.text);
    } else if (block.type === "tool_result") {
      const result = block.content;
      if (typeof result === "string") parts.push(result);
      if (Array.isArray(result)) {
        parts.push(
          result
            .filter((item): item is { type: "text"; text: string } =>
              isRecord(item) && item.type === "text" && typeof item.text === "string",
            )
            .map((item) => item.text)
            .join("\n"),
        );
      }
    } else if (block.type === "tool_use" && typeof block.name === "string") {
      parts.push(`[tool_use:${block.name}]`);
    }
  }
  return parts.filter(Boolean).join("\n");
}

function anthropicToolsToOpenRouter(tools: unknown[] | undefined): OpenRouterTool[] | undefined {
  if (!tools?.length) return undefined;
  const converted: OpenRouterTool[] = [];
  for (const tool of tools) {
    if (!isRecord(tool)) continue;
    if (typeof tool.name !== "string") continue;
    const inputSchema = isRecord(tool.input_schema) ? tool.input_schema : { type: "object", properties: {} };
    converted.push({
      type: "function",
      function: {
        name: tool.name,
        description: typeof tool.description === "string" ? tool.description : undefined,
        parameters: inputSchema,
      },
    });
  }
  return converted.length > 0 ? converted : undefined;
}

export function toOpenRouterChatRequest(req: AnthropicMessagesRequest, config: GatewayConfig): OpenRouterChatRequest {
  const messages: OpenRouterMessage[] = [];
  const system = systemToText(req.system);
  if (system) messages.push({ role: "system", content: system });

  for (const message of req.messages) {
    messages.push({
      role: message.role,
      content: contentToText(message.content),
    });
  }

  const body: OpenRouterChatRequest = {
    model: resolveModelAlias(req.model, config),
    messages,
    max_tokens: req.max_tokens,
    stream: req.stream ?? false,
  };

  if (typeof req.temperature === "number") body.temperature = req.temperature;
  if (typeof req.top_p === "number") body.top_p = req.top_p;
  if (typeof req.top_k === "number") body.top_k = req.top_k;
  if (req.stop_sequences?.length) body.stop = req.stop_sequences;

  const tools = anthropicToolsToOpenRouter(req.tools);
  if (tools) {
    body.tools = tools;
    if (req.tool_choice !== undefined) body.tool_choice = req.tool_choice;
  }
  if (body.stream) body.stream_options = { include_usage: true };

  return body;
}

function firstChoiceText(response: OpenRouterChatCompletion): string {
  const content = response.choices?.[0]?.message?.content;
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .map((part) => {
        if (isRecord(part) && typeof part.text === "string") return part.text;
        return "";
      })
      .filter(Boolean)
      .join("\n");
  }
  return "";
}

function mapStopReason(finishReason: string | null | undefined): AnthropicMessagesResponse["stop_reason"] {
  switch (finishReason) {
    case "length":
      return "max_tokens";
    case "stop":
      return "end_turn";
    case "tool_calls":
      return "tool_use";
    default:
      return null;
  }
}

function usageToAnthropic(usage: OpenRouterUsage | undefined): AnthropicMessagesResponse["usage"] {
  return {
    input_tokens: usage?.prompt_tokens ?? 0,
    output_tokens: usage?.completion_tokens ?? 0,
  };
}

export function toAnthropicMessagesResponse(
  response: OpenRouterChatCompletion,
  requestedModel: string,
): AnthropicMessagesResponse {
  return {
    id: response.id ?? `msg_${crypto.randomUUID()}`,
    type: "message",
    role: "assistant",
    model: requestedModel,
    content: [{ type: "text", text: firstChoiceText(response) }],
    stop_reason: mapStopReason(response.choices?.[0]?.finish_reason),
    stop_sequence: null,
    usage: usageToAnthropic(response.usage),
  };
}
