export interface GatewayConfig {
  host: string;
  port: number;
  openRouterApiKey: string;
  gatewayApiKey: string | null;
  openRouterBaseUrl: string;
  defaultModel: string;
  aliases: Record<string, string>;
  referrer: string | null;
  title: string | null;
}

export type AnthropicErrorType =
  | "invalid_request_error"
  | "authentication_error"
  | "permission_error"
  | "not_found_error"
  | "rate_limit_error"
  | "api_error"
  | "overloaded_error";

export interface AnthropicErrorBody {
  type: "error";
  error: {
    type: AnthropicErrorType;
    message: string;
  };
}

export interface AnthropicTextBlock {
  type: "text";
  text: string;
}

export interface AnthropicMessage {
  role: "user" | "assistant";
  content: string | Array<Record<string, unknown>>;
}

export interface AnthropicMessagesRequest {
  model: string;
  max_tokens: number;
  messages: AnthropicMessage[];
  system?: string | AnthropicTextBlock[];
  stream?: boolean;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  stop_sequences?: string[];
  tools?: unknown[];
  tool_choice?: unknown;
}

export interface AnthropicMessagesResponse {
  id: string;
  type: "message";
  role: "assistant";
  model: string;
  content: AnthropicTextBlock[];
  stop_reason: "end_turn" | "max_tokens" | "stop_sequence" | "tool_use" | null;
  stop_sequence: string | null;
  usage: {
    input_tokens: number;
    output_tokens: number;
  };
}

export interface OpenRouterMessage {
  role: "system" | "user" | "assistant" | "tool";
  content: string;
}

export interface OpenRouterTool {
  type: "function";
  function: {
    name: string;
    description?: string;
    parameters: Record<string, unknown>;
  };
}

export interface OpenRouterChatRequest {
  model: string;
  messages: OpenRouterMessage[];
  max_tokens: number;
  stream: boolean;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  stop?: string[];
  tools?: OpenRouterTool[];
  tool_choice?: unknown;
  stream_options?: {
    include_usage: boolean;
  };
}

export interface OpenRouterUsage {
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
}

export interface OpenRouterChatCompletion {
  id?: string;
  choices?: Array<{
    finish_reason?: string | null;
    message?: {
      content?: unknown;
    };
  }>;
  usage?: OpenRouterUsage;
}

export interface OpenRouterChatChunk {
  id?: string;
  error?: {
    code?: string | number;
    message?: string;
  };
  choices?: Array<{
    finish_reason?: string | null;
    delta?: {
      content?: unknown;
    };
  }>;
  usage?: OpenRouterUsage;
}
