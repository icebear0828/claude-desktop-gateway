package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/local/claude-desktop-gateway/internal/config"
)

type upstreamAdapter interface {
	Complete(ctx context.Context, provider config.Provider, route config.Route, req anthropicRequest) (upstreamMessageResult, error)
}

type upstreamMessageResult interface {
	WriteAnthropic(w http.ResponseWriter)
	StatusCode() int
	Close() error
}

type upstreamRequestError struct {
	Status  int
	Kind    anthropicErrorType
	Message string
}

func (e upstreamRequestError) Error() string {
	return e.Message
}

type openAIAdapter struct {
	client *http.Client
}

type openAIResult struct {
	response       *http.Response
	requestedModel string
	stream         bool
}

func (a openAIAdapter) Complete(ctx context.Context, provider config.Provider, route config.Route, req anthropicRequest) (upstreamMessageResult, error) {
	upstreamBody, err := openAIChatRequest(req, route.Model)
	if err != nil {
		return nil, upstreamRequestError{Status: http.StatusBadRequest, Kind: invalidRequestError, Message: err.Error()}
	}
	payload, err := json.Marshal(upstreamBody)
	if err != nil {
		return nil, upstreamRequestError{Status: http.StatusInternalServerError, Kind: apiError, Message: "Failed to encode upstream request"}
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(provider.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, upstreamRequestError{Status: http.StatusInternalServerError, Kind: apiError, Message: "Failed to create upstream request"}
	}
	setOpenAIHeaders(upstreamReq.Header, provider)
	setRouteCacheHeaders(upstreamReq.Header, route.Cache)

	upstream, err := a.client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("openai-compatible request failed: %w", err)
	}
	return openAIResult{response: upstream, requestedModel: req.Model, stream: req.Stream}, nil
}

func (r openAIResult) WriteAnthropic(w http.ResponseWriter) {
	defer r.Close()

	if r.response.StatusCode < 200 || r.response.StatusCode > 299 {
		writeAnthropicError(w, statusCode(r.response.StatusCode), errorTypeForStatus(r.response.StatusCode), upstreamError(r.response))
		return
	}

	copyOpenRouterCacheHeaders(w.Header(), r.response.Header)
	if r.stream {
		streamOpenAIToAnthropic(w, r.response, r.requestedModel)
		return
	}

	var completion openAICompletion
	if err := json.NewDecoder(r.response.Body).Decode(&completion); err != nil {
		writeAnthropicError(w, http.StatusBadGateway, apiError, "OpenRouter response was not valid JSON")
		return
	}
	writeJSON(w, http.StatusOK, anthropicCompletionResponse(completion, r.requestedModel))
}

func (r openAIResult) StatusCode() int {
	if r.response == nil {
		return 0
	}
	return r.response.StatusCode
}

func (r openAIResult) Close() error {
	if r.response == nil || r.response.Body == nil {
		return nil
	}
	return r.response.Body.Close()
}
