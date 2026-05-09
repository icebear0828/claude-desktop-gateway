package gateway

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/local/claude-desktop-gateway/internal/config"
)

type Gateway struct {
	cfg    config.Config
	openAI upstreamAdapter
}

type anthropicErrorType string

const (
	invalidRequestError  anthropicErrorType = "invalid_request_error"
	authenticationError  anthropicErrorType = "authentication_error"
	permissionError      anthropicErrorType = "permission_error"
	notFoundError        anthropicErrorType = "not_found_error"
	rateLimitError       anthropicErrorType = "rate_limit_error"
	apiError             anthropicErrorType = "api_error"
	overloadedError      anthropicErrorType = "overloaded_error"
	defaultRequestTimout                    = 120 * time.Second
)

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	Messages      []anthropicMessage `json:"messages"`
	System        json.RawMessage    `json:"system"`
	Stream        bool               `json:"stream"`
	Temperature   *float64           `json:"temperature"`
	TopP          *float64           `json:"top_p"`
	TopK          *float64           `json:"top_k"`
	StopSequences []string           `json:"stop_sequences"`
	Tools         []json.RawMessage  `json:"tools"`
	ToolChoice    json.RawMessage    `json:"tool_choice"`
}

func New(cfg config.Config, client *http.Client) http.Handler {
	if client == nil {
		client = &http.Client{Timeout: defaultRequestTimout}
	}
	return &Gateway{cfg: cfg, openAI: openAIAdapter{client: client}}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/health":
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case strings.HasPrefix(r.URL.Path, "/v1/"):
		if !g.hasValidGatewayAuth(r.Header) {
			writeAnthropicError(w, http.StatusUnauthorized, authenticationError, "Invalid API key")
			return
		}
		g.serveV1(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (g *Gateway) serveV1(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/v1/models" && r.Method == http.MethodGet:
		g.handleModels(w)
	case r.URL.Path == "/v1/messages" && r.Method == http.MethodPost:
		g.handleMessages(w, r)
	default:
		writeAnthropicError(w, http.StatusNotFound, notFoundError, "Endpoint not found")
	}
}

func (g *Gateway) handleModels(w http.ResponseWriter) {
	type model struct {
		ID          string `json:"id"`
		Object      string `json:"object"`
		Created     int64  `json:"created"`
		OwnedBy     string `json:"owned_by"`
		DisplayName string `json:"display_name"`
	}
	desktopModels := g.cfg.DesktopModels()
	models := make([]model, 0, len(desktopModels))
	for _, desktopModel := range desktopModels {
		models = append(models, model{
			ID:          desktopModel.ID,
			Object:      "model",
			Created:     1700000000,
			OwnedBy:     desktopModel.Provider,
			DisplayName: desktopModel.DisplayName,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": models})
}

func (g *Gateway) handleMessages(w http.ResponseWriter, r *http.Request) {
	req, errMessage := parseAnthropicRequest(r.Body)
	if errMessage != "" {
		writeAnthropicError(w, http.StatusBadRequest, invalidRequestError, errMessage)
		return
	}

	route, ok := g.cfg.ResolveRoute(req.Model)
	if !ok {
		writeAnthropicError(w, http.StatusBadRequest, invalidRequestError, "model is not configured")
		return
	}
	provider, ok := g.cfg.ProviderFor(route)
	if !ok {
		writeAnthropicError(w, http.StatusBadRequest, invalidRequestError, "provider is not configured")
		return
	}
	adapter, ok := g.adapterFor(provider)
	if !ok {
		writeAnthropicError(w, http.StatusBadRequest, invalidRequestError, "provider profile is not supported")
		return
	}
	if message := unsupportedProviderCapability(provider, req); message != "" {
		writeAnthropicError(w, http.StatusBadRequest, invalidRequestError, message)
		return
	}

	ctx := r.Context()
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRequestTimout)
		defer cancel()
	}

	result, err := adapter.Complete(ctx, provider, route, req)
	if err != nil {
		var requestErr upstreamRequestError
		if errors.As(err, &requestErr) {
			writeAnthropicError(w, requestErr.Status, requestErr.Kind, requestErr.Message)
			return
		}
		writeAnthropicError(w, http.StatusBadGateway, apiError, "OpenRouter request failed")
		return
	}
	result.WriteAnthropic(w)
}

func (g *Gateway) adapterFor(provider config.Provider) (upstreamAdapter, bool) {
	switch provider.Profile {
	case "", "openai-chat":
		return g.openAI, true
	default:
		return nil, false
	}
}

func unsupportedProviderCapability(provider config.Provider, req anthropicRequest) string {
	capabilities := provider.Capabilities.Effective()
	if req.Stream && !capabilities.Streaming {
		return "provider does not support streaming"
	}
	if len(req.Tools) > 0 && !capabilities.Tools {
		return "provider does not support tools"
	}
	return ""
}

func parseAnthropicRequest(body io.Reader) (anthropicRequest, string) {
	var req anthropicRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&req); err != nil {
		return anthropicRequest{}, "Invalid JSON in request body"
	}
	if strings.TrimSpace(req.Model) == "" {
		return anthropicRequest{}, "model is required"
	}
	if req.MaxTokens <= 0 {
		return anthropicRequest{}, "max_tokens must be a positive number"
	}
	if len(req.Messages) == 0 {
		return anthropicRequest{}, "messages must be an array of user/assistant messages"
	}
	for _, message := range req.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return anthropicRequest{}, "messages must be an array of user/assistant messages"
		}
		if !isStringJSON(message.Content) && !isArrayJSON(message.Content) {
			return anthropicRequest{}, "messages must be an array of user/assistant messages"
		}
	}
	return req, ""
}

func openAIChatRequest(req anthropicRequest, model string) (map[string]any, error) {
	messages := make([]map[string]string, 0, len(req.Messages)+1)
	if system := systemToText(req.System); system != "" {
		messages = append(messages, map[string]string{"role": "system", "content": system})
	}
	for _, message := range req.Messages {
		text, err := contentToText(message.Content)
		if err != nil {
			return nil, err
		}
		messages = append(messages, map[string]string{"role": message.Role, "content": text})
	}

	body := map[string]any{
		"model":      model,
		"messages":   messages,
		"max_tokens": req.MaxTokens,
		"stream":     req.Stream,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.TopK != nil {
		body["top_k"] = *req.TopK
	}
	if len(req.StopSequences) > 0 {
		body["stop"] = req.StopSequences
	}
	if tools := anthropicToolsToOpenAI(req.Tools); len(tools) > 0 {
		body["tools"] = tools
		if len(req.ToolChoice) > 0 && string(req.ToolChoice) != "null" {
			var toolChoice any
			if err := json.Unmarshal(req.ToolChoice, &toolChoice); err == nil {
				body["tool_choice"] = toolChoice
			}
		}
	}
	if req.Stream {
		body["stream_options"] = map[string]bool{"include_usage": true}
	}
	return body, nil
}

func anthropicToolsToOpenAI(tools []json.RawMessage) []map[string]any {
	converted := make([]map[string]any, 0, len(tools))
	for _, raw := range tools {
		var tool map[string]json.RawMessage
		if err := json.Unmarshal(raw, &tool); err != nil {
			continue
		}
		var name string
		if err := json.Unmarshal(tool["name"], &name); err != nil || name == "" {
			continue
		}
		var description string
		_ = json.Unmarshal(tool["description"], &description)
		parameters := map[string]any{"type": "object", "properties": map[string]any{}}
		if rawSchema, ok := tool["input_schema"]; ok && len(rawSchema) > 0 {
			var schema map[string]any
			if err := json.Unmarshal(rawSchema, &schema); err == nil && schema != nil {
				parameters = schema
			}
		}
		function := map[string]any{"name": name, "parameters": parameters}
		if description != "" {
			function["description"] = description
		}
		converted = append(converted, map[string]any{"type": "function", "function": function})
	}
	return converted
}

func systemToText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return textBlocks(raw)
}

func contentToText(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	if !isArrayJSON(raw) {
		return "", fmt.Errorf("message content must be a string or array")
	}

	var blocks []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", fmt.Errorf("message content must be a string or array")
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		var kind string
		_ = json.Unmarshal(block["type"], &kind)
		switch kind {
		case "text":
			var value string
			if err := json.Unmarshal(block["text"], &value); err == nil && value != "" {
				parts = append(parts, value)
			}
		case "tool_result":
			if value := toolResultText(block["content"]); value != "" {
				parts = append(parts, value)
			}
		case "tool_use":
			var name string
			if err := json.Unmarshal(block["name"], &name); err == nil && name != "" {
				parts = append(parts, "[tool_use:"+name+"]")
			}
		}
	}
	return strings.Join(parts, "\n"), nil
}

func textBlocks(raw json.RawMessage) string {
	var blocks []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		var kind string
		var text string
		_ = json.Unmarshal(block["type"], &kind)
		_ = json.Unmarshal(block["text"], &text)
		if kind == "text" && text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func toolResultText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return textBlocks(raw)
}

func isStringJSON(raw json.RawMessage) bool {
	var value string
	return json.Unmarshal(raw, &value) == nil
}

func isArrayJSON(raw json.RawMessage) bool {
	var value []json.RawMessage
	return json.Unmarshal(raw, &value) == nil
}

func setOpenAIHeaders(headers http.Header, provider config.Provider) {
	headers.Set("Authorization", "Bearer "+provider.APIKey)
	headers.Set("Content-Type", "application/json")
	if provider.Referrer != "" {
		headers.Set("HTTP-Referer", provider.Referrer)
	}
	if provider.Title != "" {
		headers.Set("X-Title", provider.Title)
	}
}

func (g *Gateway) hasValidGatewayAuth(headers http.Header) bool {
	if g.cfg.GatewayAPIKey == "" {
		return true
	}
	if headers.Get("x-api-key") == g.cfg.GatewayAPIKey {
		return true
	}
	auth := headers.Get("authorization")
	token, ok := strings.CutPrefix(auth, "Bearer ")
	return ok && token == g.cfg.GatewayAPIKey
}

func writeAnthropicError(w http.ResponseWriter, status int, kind anthropicErrorType, message string) {
	writeJSON(w, status, map[string]any{
		"type": "error",
		"error": map[string]string{
			"type":    string(kind),
			"message": message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func statusCode(status int) int {
	if status >= 400 && status <= 599 {
		return status
	}
	return http.StatusInternalServerError
}

func errorTypeForStatus(status int) anthropicErrorType {
	switch status {
	case http.StatusUnauthorized:
		return authenticationError
	case http.StatusForbidden:
		return permissionError
	case http.StatusNotFound:
		return notFoundError
	case http.StatusTooManyRequests:
		return rateLimitError
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return invalidRequestError
	case 529:
		return overloadedError
	default:
		return apiError
	}
}

func upstreamError(response *http.Response) string {
	var body struct {
		Error any `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err == nil {
		switch value := body.Error.(type) {
		case string:
			if value != "" {
				return value
			}
		case map[string]any:
			if message, ok := value["message"].(string); ok && message != "" {
				return message
			}
		}
	}
	if http.StatusText(response.StatusCode) != "" {
		return http.StatusText(response.StatusCode)
	}
	return "OpenRouter request failed"
}

type openAICompletion struct {
	ID      string `json:"id"`
	Choices []struct {
		FinishReason *string `json:"finish_reason"`
		Message      struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage openAIUsage `json:"usage"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func anthropicCompletionResponse(response openAICompletion, requestedModel string) map[string]any {
	id := response.ID
	if id == "" {
		id = "msg_" + randomHex(12)
	}
	finishReason := ""
	if len(response.Choices) > 0 && response.Choices[0].FinishReason != nil {
		finishReason = *response.Choices[0].FinishReason
	}
	return map[string]any{
		"id":            id,
		"type":          "message",
		"role":          "assistant",
		"model":         requestedModel,
		"content":       []map[string]string{{"type": "text", "text": firstChoiceText(response)}},
		"stop_reason":   stopReason(finishReason),
		"stop_sequence": nil,
		"usage":         map[string]int{"input_tokens": response.Usage.PromptTokens, "output_tokens": response.Usage.CompletionTokens},
	}
}

func firstChoiceText(response openAICompletion) string {
	if len(response.Choices) == 0 {
		return ""
	}
	raw := response.Choices[0].Message.Content
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return textBlocks(raw)
}

func stopReason(finishReason string) any {
	switch finishReason {
	case "length":
		return "max_tokens"
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	default:
		return nil
	}
}

func randomHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func streamOpenAIToAnthropic(w http.ResponseWriter, upstream *http.Response, requestedModel string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(upstream.Body)
	started := false
	stopped := false
	messageID := upstream.Header.Get("x-generation-id")
	if messageID == "" {
		messageID = "msg_" + randomHex(12)
	}
	usage := openAIUsage{}
	finishReason := "stop"

	startMessage := func() {
		if started {
			return
		}
		started = true
		writeEvent(w, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":            messageID,
				"type":          "message",
				"role":          "assistant",
				"model":         requestedModel,
				"content":       []any{},
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage":         map[string]int{"input_tokens": usage.PromptTokens, "output_tokens": 0},
			},
		})
		writeEvent(w, "content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         0,
			"content_block": map[string]string{"type": "text", "text": ""},
		})
		if flusher != nil {
			flusher.Flush()
		}
	}

	stopMessage := func() {
		if stopped {
			return
		}
		startMessage()
		stopped = true
		writeEvent(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
		writeEvent(w, "message_delta", map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   streamStopReason(finishReason),
				"stop_sequence": nil,
			},
			"usage": map[string]int{"output_tokens": usage.CompletionTokens},
		})
		writeEvent(w, "message_stop", map[string]string{"type": "message_stop"})
		if flusher != nil {
			flusher.Flush()
		}
	}

	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			processStreamLine(strings.TrimRight(line, "\r\n"), &messageID, &usage, &finishReason, startMessage, stopMessage, w, flusher)
		}
		if err != nil {
			break
		}
	}
	stopMessage()
}

func processStreamLine(
	line string,
	messageID *string,
	usage *openAIUsage,
	finishReason *string,
	startMessage func(),
	stopMessage func(),
	w http.ResponseWriter,
	flusher http.Flusher,
) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, ":") || !strings.HasPrefix(trimmed, "data:") {
		return
	}
	data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if data == "[DONE]" {
		stopMessage()
		return
	}

	var chunk struct {
		ID    string `json:"id"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Choices []struct {
			FinishReason *string `json:"finish_reason"`
			Delta        struct {
				Content json.RawMessage `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *openAIUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return
	}
	if chunk.ID != "" {
		*messageID = chunk.ID
	}
	if chunk.Usage != nil {
		*usage = *chunk.Usage
	}
	if chunk.Error != nil {
		startMessage()
		writeEvent(w, "error", map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    string(apiError),
				"message": valueOrDefault(chunk.Error.Message, "OpenRouter stream error"),
			},
		})
		*finishReason = "error"
		stopMessage()
		return
	}
	if len(chunk.Choices) == 0 {
		return
	}
	if chunk.Choices[0].FinishReason != nil {
		*finishReason = *chunk.Choices[0].FinishReason
	}
	var content string
	if err := json.Unmarshal(chunk.Choices[0].Delta.Content, &content); err == nil && content != "" {
		startMessage()
		writeEvent(w, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]string{"type": "text_delta", "text": content},
		})
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func writeEvent(w io.Writer, event string, data any) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, encoded)
}

func streamStopReason(finishReason string) string {
	switch finishReason {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

func valueOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
