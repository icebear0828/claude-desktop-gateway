package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/local/claude-desktop-gateway/internal/config"
)

type anthropicAdapter struct {
	client *http.Client
}

type anthropicResult struct {
	response       *http.Response
	requestedModel string
	stream         bool
}

func (a anthropicAdapter) Complete(ctx context.Context, provider config.Provider, route config.Route, req anthropicRequest) (upstreamMessageResult, error) {
	payload, err := anthropicMessagesPayload(req, route.Model)
	if err != nil {
		return nil, upstreamRequestError{Status: http.StatusInternalServerError, Kind: apiError, Message: "Failed to encode upstream request"}
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(provider.BaseURL, "/")+"/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, upstreamRequestError{Status: http.StatusInternalServerError, Kind: apiError, Message: "Failed to create upstream request"}
	}
	setOpenAIHeaders(upstreamReq.Header, provider)
	setRouteCacheHeaders(upstreamReq.Header, route.Cache)

	upstream, err := a.client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic-compatible request failed: %w", err)
	}
	return anthropicResult{response: upstream, requestedModel: req.Model, stream: req.Stream}, nil
}

func anthropicMessagesPayload(req anthropicRequest, upstreamModel string) ([]byte, error) {
	body := make(map[string]json.RawMessage, len(req.Raw)+1)
	for key, value := range req.Raw {
		body[key] = value
	}
	model, err := json.Marshal(upstreamModel)
	if err != nil {
		return nil, err
	}
	body["model"] = model
	return json.Marshal(body)
}

func (r anthropicResult) WriteAnthropic(w http.ResponseWriter) {
	defer r.Close()

	if r.response.StatusCode < 200 || r.response.StatusCode > 299 {
		writeAnthropicError(w, statusCode(r.response.StatusCode), errorTypeForStatus(r.response.StatusCode), upstreamError(r.response))
		return
	}

	copyOpenRouterCacheHeaders(w.Header(), r.response.Header)
	if r.stream {
		streamAnthropicMessages(w, r.response, r.requestedModel)
		return
	}

	payload, err := io.ReadAll(r.response.Body)
	if err != nil {
		writeAnthropicError(w, http.StatusBadGateway, apiError, "OpenRouter response was not valid JSON")
		return
	}
	payload = rewriteAnthropicResponseModel(payload, r.requestedModel)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (r anthropicResult) StatusCode() int {
	if r.response == nil {
		return 0
	}
	return r.response.StatusCode
}

func (r anthropicResult) Close() error {
	if r.response == nil || r.response.Body == nil {
		return nil
	}
	return r.response.Body.Close()
}

func rewriteAnthropicResponseModel(payload []byte, requestedModel string) []byte {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(payload, &body); err != nil {
		return payload
	}
	model, err := json.Marshal(requestedModel)
	if err != nil {
		return payload
	}
	body["model"] = model
	rewritten, err := json.Marshal(body)
	if err != nil {
		return payload
	}
	return rewritten
}

func streamAnthropicMessages(w http.ResponseWriter, upstream *http.Response, requestedModel string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(upstream.Body)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			_, _ = io.WriteString(w, rewriteAnthropicStreamLine(line, requestedModel))
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}

func rewriteAnthropicStreamLine(line string, requestedModel string) string {
	trimmedLine := strings.TrimRight(line, "\r\n")
	suffix := line[len(trimmedLine):]
	trimmed := strings.TrimSpace(trimmedLine)
	if !strings.HasPrefix(trimmed, "data:") {
		return line
	}
	data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if data == "" || data == "[DONE]" {
		return line
	}
	rewritten, ok := rewriteAnthropicStreamPayload([]byte(data), requestedModel)
	if !ok {
		return line
	}
	return "data: " + string(rewritten) + suffix
}

func rewriteAnthropicStreamPayload(payload []byte, requestedModel string) ([]byte, bool) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, false
	}
	rawMessage, ok := body["message"]
	if !ok {
		return payload, true
	}
	var message map[string]json.RawMessage
	if err := json.Unmarshal(rawMessage, &message); err != nil {
		return payload, true
	}
	model, err := json.Marshal(requestedModel)
	if err != nil {
		return payload, true
	}
	message["model"] = model
	rewrittenMessage, err := json.Marshal(message)
	if err != nil {
		return payload, true
	}
	body["message"] = rewrittenMessage
	rewritten, err := json.Marshal(body)
	if err != nil {
		return payload, true
	}
	return rewritten, true
}
