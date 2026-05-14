package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"platform-service/internal/config"
)

type minimaxTextProvider struct {
	name   string
	cfg    config.MinimaxConfig
	client *http.Client
}

func newMinimaxTextProvider(name string, cfg config.MinimaxConfig) GenerationProvider {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &minimaxTextProvider{name: name, cfg: cfg, client: &http.Client{Timeout: timeout}}
}

func (p *minimaxTextProvider) Name() string { return p.name }

func (p *minimaxTextProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return nil, newNonRetryableProviderError("minimax api key is not configured")
	}
	messages := buildMinimaxMessages(req)
	if len(messages) == 0 {
		return nil, newNonRetryableProviderError("prompt is required for minimax text runtime")
	}
	model := firstNonEmpty(
		stringMapValue(req.Input.ParamsSnapshot, "model"),
		req.Input.PromptSnapshot.Model,
		p.cfg.Model,
		"MiniMax-M2",
	)
	maxTokens := p.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	temperature := p.cfg.Temperature
	payload := minimaxChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
	if strings.EqualFold(stringMapValue(req.Input.ParamsSnapshot, "response_format"), "json") || req.TaskType == RuntimeTaskIntentPlanning || req.TaskType == RuntimeTaskPromptPlanning {
		payload.ResponseFormat = map[string]string{"type": "json_object"}
	}
	resp, err := p.chatCompletions(ctx, payload)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(resp.FirstContent())
	if content == "" {
		return nil, newRetryableProviderError("minimax returned empty content")
	}
	mimeType := "text/plain"
	assetType := "text"
	if looksLikeJSON(content) || payload.ResponseFormat != nil {
		mimeType = "application/json"
		assetType = "json"
	}
	stageMessage := "Text runtime completed"
	return &ProviderSubmission{
		ProviderJobID: fmt.Sprintf("minimax-%s-%d", req.RuntimeJobID, time.Now().UnixNano()),
		Stage:         "provider_completed",
		StageMessage:  stageMessage,
		Completion: &ProviderCompletion{
			Status:       "completed",
			Progress:     100,
			StageMessage: stageMessage,
			Variants: []ProviderResultVariant{{
				Index:      0,
				AssetType:  assetType,
				InlineData: content,
				MimeType:   mimeType,
				Metadata: map[string]any{
					"provider":       p.name,
					"provider_model": resp.Model,
					"runtime_task":   req.TaskType,
				},
			}},
			Metadata: map[string]any{
				"provider":      p.name,
				"provider_mode": "sync_chat_completion",
				"runtime_task":  req.TaskType,
				"model":         resp.Model,
			},
		},
	}, nil
}

func (p *minimaxTextProvider) Poll(_ context.Context, _ string) (*ProviderPollResult, error) {
	return nil, newNonRetryableProviderError("minimax sync provider does not support polling")
}

func (p *minimaxTextProvider) Cancel(_ context.Context, _ string) error { return nil }

func (p *minimaxTextProvider) chatCompletions(ctx context.Context, payload minimaxChatCompletionRequest) (*minimaxChatCompletionResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("minimax request encode failed: %v", err))
	}
	endpoint := strings.TrimRight(firstNonEmpty(strings.TrimSpace(p.cfg.BaseURL), "https://api.minimax.chat/v1"), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("minimax request init failed: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("minimax request failed: %v", err))
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("minimax response read failed: %v", err))
	}
	var out minimaxChatCompletionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("decode minimax response: %v", err))
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(out.Error.Message)
		if message == "" {
			message = fmt.Sprintf("minimax request failed: status=%d", resp.StatusCode)
		}
		return nil, classifyMinimaxError(message, resp.StatusCode)
	}
	return &out, nil
}

func buildMinimaxMessages(req ProviderJobRequest) []minimaxChatMessage {
	messages := make([]minimaxChatMessage, 0, 4)
	add := func(role, content string) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		messages = append(messages, minimaxChatMessage{Role: role, Content: content})
	}
	add("system", req.Input.PromptSnapshot.SystemPrompt)
	add("system", req.Input.PromptSnapshot.StylePrompt)
	add("user", req.Input.PromptSnapshot.UserPrompt)
	add("user", stringMapValue(req.Input.ParamsSnapshot, "prompt"))
	return messages
}

func classifyMinimaxError(message string, status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return newNonRetryableProviderError("minimax authentication failed")
	case http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity:
		return newNonRetryableProviderError(message)
	case http.StatusTooManyRequests:
		return newRetryableProviderError(message)
	default:
		if status >= 500 {
			return newRetryableProviderError(message)
		}
		return newNonRetryableProviderError(message)
	}
}

func looksLikeJSON(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

type minimaxChatCompletionRequest struct {
	Model          string               `json:"model"`
	Messages       []minimaxChatMessage `json:"messages"`
	MaxTokens      int                  `json:"max_tokens,omitempty"`
	Temperature    float64              `json:"temperature,omitempty"`
	ResponseFormat map[string]string    `json:"response_format,omitempty"`
}

type minimaxChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type minimaxChatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message minimaxChatMessage `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (r minimaxChatCompletionResponse) FirstContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}
