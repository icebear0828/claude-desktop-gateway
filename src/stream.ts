import type { AnthropicMessagesResponse, OpenRouterChatChunk, OpenRouterUsage } from "./types.js";

const encoder = new TextEncoder();

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parseJsonObject(value: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(value) as unknown;
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function toChunk(value: Record<string, unknown>): OpenRouterChatChunk {
  const choices = Array.isArray(value.choices)
    ? value.choices
        .filter(isRecord)
        .map((choice) => {
          const rawFinishReason = choice.finish_reason;
          return {
            finish_reason: typeof rawFinishReason === "string" || rawFinishReason === null
              ? rawFinishReason
              : undefined,
            delta: isRecord(choice.delta)
              ? { content: choice.delta.content }
              : undefined,
          };
        })
    : undefined;
  const usage = isRecord(value.usage)
    ? {
        prompt_tokens: typeof value.usage.prompt_tokens === "number" ? value.usage.prompt_tokens : undefined,
        completion_tokens: typeof value.usage.completion_tokens === "number" ? value.usage.completion_tokens : undefined,
        total_tokens: typeof value.usage.total_tokens === "number" ? value.usage.total_tokens : undefined,
      }
    : undefined;
  const error = isRecord(value.error)
    ? {
        code: typeof value.error.code === "string" || typeof value.error.code === "number"
          ? value.error.code
          : undefined,
        message: typeof value.error.message === "string" ? value.error.message : undefined,
      }
    : undefined;

  return {
    id: typeof value.id === "string" ? value.id : undefined,
    choices,
    usage,
    error,
  };
}

function writeEvent(controller: ReadableStreamDefaultController<Uint8Array>, event: string, data: unknown): void {
  controller.enqueue(encoder.encode(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`));
}

function stopReason(finishReason: string | null | undefined): AnthropicMessagesResponse["stop_reason"] {
  if (finishReason === "length") return "max_tokens";
  if (finishReason === "tool_calls") return "tool_use";
  if (finishReason === "stop") return "end_turn";
  if (finishReason === "error") return "end_turn";
  return "end_turn";
}

function usageDelta(usage: OpenRouterUsage): { output_tokens: number } {
  return { output_tokens: usage.completion_tokens ?? 0 };
}

export function openRouterStreamToAnthropic(upstream: Response, requestedModel: string): Response {
  const reader = upstream.body?.getReader();
  if (!reader) {
    return new Response("Upstream response is not streamable", { status: 502 });
  }

  const decoder = new TextDecoder();
  const stream = new ReadableStream<Uint8Array>({
    async start(controller) {
      let buffer = "";
      let started = false;
      let stopped = false;
      let messageId = upstream.headers.get("x-generation-id") ?? `msg_${crypto.randomUUID()}`;
      let usage: OpenRouterUsage = {};
      let finishReason: string | null | undefined = "stop";

      const startMessage = (): void => {
        if (started) return;
        started = true;
        writeEvent(controller, "message_start", {
          type: "message_start",
          message: {
            id: messageId,
            type: "message",
            role: "assistant",
            model: requestedModel,
            content: [],
            stop_reason: null,
            stop_sequence: null,
            usage: {
              input_tokens: usage.prompt_tokens ?? 0,
              output_tokens: 0,
            },
          },
        });
        writeEvent(controller, "content_block_start", {
          type: "content_block_start",
          index: 0,
          content_block: { type: "text", text: "" },
        });
      };

      const stopMessage = (): void => {
        if (stopped) return;
        startMessage();
        stopped = true;
        writeEvent(controller, "content_block_stop", {
          type: "content_block_stop",
          index: 0,
        });
        writeEvent(controller, "message_delta", {
          type: "message_delta",
          delta: {
            stop_reason: stopReason(finishReason),
            stop_sequence: null,
          },
          usage: usageDelta(usage),
        });
        writeEvent(controller, "message_stop", { type: "message_stop" });
      };

      const handleData = (data: string): void => {
        if (data === "[DONE]") {
          stopMessage();
          return;
        }
        const parsed = parseJsonObject(data);
        if (!parsed) return;
        const chunk = toChunk(parsed);
        if (chunk.id) messageId = chunk.id;
        if (chunk.usage) usage = { ...usage, ...chunk.usage };
        if (chunk.error) {
          startMessage();
          writeEvent(controller, "error", {
            type: "error",
            error: {
              type: "api_error",
              message: chunk.error.message ?? "OpenRouter stream error",
            },
          });
          finishReason = "error";
          stopMessage();
          return;
        }

        const choice = chunk.choices?.[0];
        if (choice?.finish_reason !== undefined) finishReason = choice.finish_reason;
        const content = choice?.delta?.content;
        if (typeof content === "string" && content.length > 0) {
          startMessage();
          writeEvent(controller, "content_block_delta", {
            type: "content_block_delta",
            index: 0,
            delta: { type: "text_delta", text: content },
          });
        }
      };

      const processLine = (line: string): void => {
        const trimmed = line.trimEnd();
        if (!trimmed || trimmed.startsWith(":")) return;
        if (!trimmed.startsWith("data:")) return;
        handleData(trimmed.slice("data:".length).trim());
      };

      try {
        while (true) {
          const result = await reader.read();
          if (result.done) break;
          buffer += decoder.decode(result.value, { stream: true });
          let newlineIndex = buffer.indexOf("\n");
          while (newlineIndex >= 0) {
            const line = buffer.slice(0, newlineIndex);
            buffer = buffer.slice(newlineIndex + 1);
            processLine(line);
            newlineIndex = buffer.indexOf("\n");
          }
        }
        buffer += decoder.decode();
        if (buffer.trim()) processLine(buffer);
        stopMessage();
        controller.close();
      } catch (error) {
        controller.error(error);
      } finally {
        reader.releaseLock();
      }
    },
    cancel() {
      void reader.cancel();
    },
  });

  return new Response(stream, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream; charset=utf-8",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
