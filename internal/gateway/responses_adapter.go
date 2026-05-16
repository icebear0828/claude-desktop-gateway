package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/local/claude-desktop-gateway/internal/config"
)

type responsesRequest struct {
	Model  string
	Stream bool
	Raw    map[string]json.RawMessage
}

type responsesAdapter struct {
	client *http.Client
}

type responsesResult struct {
	response       *http.Response
	requestedModel string
	stream         bool
}

func (g *Gateway) handleResponses(w http.ResponseWriter, r *http.Request) {
	req, errMessage := parseResponsesRequest(r.Body)
	if errMessage != "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", errMessage)
		return
	}

	routes, ok := g.cfg.ResolveRoutes(req.Model)
	if !ok {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "model is not configured")
		return
	}

	ctx := r.Context()
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRequestTimout)
		defer cancel()
	}

	for i := 0; i < len(routes); i++ {
		route := routes[i]
		provider, ok := g.cfg.ProviderFor(route)
		if !ok {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "provider is not configured")
			return
		}
		if provider.Profile != "responses" {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "provider profile is not supported for Responses API")
			return
		}
		if message := unsupportedResponsesCapability(provider, req); message != "" {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", message)
			return
		}

		result, err := g.responses.Complete(ctx, provider, route, req)
		if err != nil {
			if canFallbackError(err) && hasNextRoute(routes, i) {
				continue
			}
			var requestErr upstreamRequestError
			if errors.As(err, &requestErr) {
				writeOpenAIError(w, requestErr.Status, openAIErrorTypeForStatus(requestErr.Status), requestErr.Message)
				return
			}
			writeOpenAIError(w, http.StatusBadGateway, "server_error", "OpenRouter request failed")
			return
		}
		if canFallbackStatus(result.StatusCode()) && hasNextRoute(routes, i) {
			_ = result.Close()
			continue
		}
		result.WriteResponses(w)
		return
	}

	writeOpenAIError(w, http.StatusBadGateway, "server_error", "OpenRouter request failed")
}

func (a responsesAdapter) Complete(ctx context.Context, provider config.Provider, route config.Route, req responsesRequest) (responsesResult, error) {
	payload, err := responsesPayload(req, route.Model)
	if err != nil {
		return responsesResult{}, upstreamRequestError{Status: http.StatusInternalServerError, Kind: apiError, Message: "Failed to encode upstream request"}
	}
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(provider.BaseURL, "/")+"/responses", bytes.NewReader(payload))
	if err != nil {
		return responsesResult{}, upstreamRequestError{Status: http.StatusInternalServerError, Kind: apiError, Message: "Failed to create upstream request"}
	}
	setOpenAIHeaders(upstreamReq.Header, provider)
	setRouteCacheHeaders(upstreamReq.Header, route.Cache)

	upstream, err := a.client.Do(upstreamReq)
	if err != nil {
		return responsesResult{}, fmt.Errorf("responses-compatible request failed: %w", err)
	}
	return responsesResult{response: upstream, requestedModel: req.Model, stream: req.Stream}, nil
}

func parseResponsesRequest(body io.Reader) (responsesRequest, string) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&raw); err != nil {
		return responsesRequest{}, "Invalid JSON in request body"
	}
	var model string
	if err := json.Unmarshal(raw["model"], &model); err != nil || strings.TrimSpace(model) == "" {
		return responsesRequest{}, "model is required"
	}
	var stream bool
	_ = json.Unmarshal(raw["stream"], &stream)
	return responsesRequest{Model: strings.TrimSpace(model), Stream: stream, Raw: raw}, ""
}

func responsesPayload(req responsesRequest, upstreamModel string) ([]byte, error) {
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

func unsupportedResponsesCapability(provider config.Provider, req responsesRequest) string {
	capabilities := provider.Capabilities.Effective()
	if req.Stream && !capabilities.Streaming {
		return "provider does not support streaming"
	}
	if hasResponsesTools(req.Raw) && !capabilities.Tools {
		return "provider does not support tools"
	}
	return ""
}

func hasResponsesTools(raw map[string]json.RawMessage) bool {
	value, ok := raw["tools"]
	if !ok {
		return false
	}
	trimmed := strings.TrimSpace(string(value))
	return trimmed != "" && trimmed != "null" && trimmed != "[]"
}

func (r responsesResult) WriteResponses(w http.ResponseWriter) {
	defer r.Close()

	if r.response.StatusCode < 200 || r.response.StatusCode > 299 {
		writeOpenAIError(w, statusCode(r.response.StatusCode), openAIErrorTypeForStatus(r.response.StatusCode), upstreamError(r.response))
		return
	}

	copyOpenRouterCacheHeaders(w.Header(), r.response.Header)
	if r.stream {
		streamResponses(w, r.response, r.requestedModel)
		return
	}

	payload, err := io.ReadAll(r.response.Body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "server_error", "OpenRouter response was not valid JSON")
		return
	}
	payload = rewriteResponsesPayloadModel(payload, r.requestedModel)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (r responsesResult) StatusCode() int {
	if r.response == nil {
		return 0
	}
	return r.response.StatusCode
}

func (r responsesResult) Close() error {
	if r.response == nil || r.response.Body == nil {
		return nil
	}
	return r.response.Body.Close()
}

func streamResponses(w http.ResponseWriter, upstream *http.Response, requestedModel string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(upstream.Body)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			_, _ = io.WriteString(w, rewriteResponsesStreamLine(line, requestedModel))
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}

func rewriteResponsesStreamLine(line string, requestedModel string) string {
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
	rewritten := rewriteResponsesPayloadModel([]byte(data), requestedModel)
	return "data: " + string(rewritten) + suffix
}

func rewriteResponsesPayloadModel(payload []byte, requestedModel string) []byte {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(payload, &body); err != nil {
		return payload
	}
	model, err := json.Marshal(requestedModel)
	if err != nil {
		return payload
	}
	if _, ok := body["model"]; ok {
		body["model"] = model
	}
	if rawResponse, ok := body["response"]; ok {
		var response map[string]json.RawMessage
		if err := json.Unmarshal(rawResponse, &response); err == nil {
			if _, ok := response["model"]; ok {
				response["model"] = model
				if rewrittenResponse, err := json.Marshal(response); err == nil {
					body["response"] = rewrittenResponse
				}
			}
		}
	}
	rewritten, err := json.Marshal(body)
	if err != nil {
		return payload
	}
	return rewritten
}

func openAIErrorTypeForStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "authentication_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "invalid_request_error"
	default:
		return "server_error"
	}
}
