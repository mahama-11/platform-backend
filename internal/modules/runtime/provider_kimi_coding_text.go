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

type kimiCodingTextProvider struct {
	name   string
	cfg    config.KimiCodingConfig
	client *http.Client
}

func newKimiCodingTextProvider(name string, cfg config.KimiCodingConfig) GenerationProvider {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &kimiCodingTextProvider{name: name, cfg: cfg, client: &http.Client{Timeout: timeout}}
}

func (p *kimiCodingTextProvider) Name() string { return p.name }

func (p *kimiCodingTextProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return nil, newNonRetryableProviderError("kimi coding api key is not configured")
	}
	messages := buildKimiCodingMessages(req)
	if len(messages) == 0 {
		return nil, newNonRetryableProviderError("prompt is required for kimi coding text runtime")
	}
	model := firstNonEmpty(
		stringMapValue(req.Input.ParamsSnapshot, "model"),
		req.Input.PromptSnapshot.Model,
		p.cfg.Model,
		"kimi-k2.6",
	)
	maxTokens := p.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	payload := kimiCodingMessagesRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: p.cfg.Temperature,
	}
	resp, err := p.messages(ctx, payload)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(resp.FirstText())
	if content == "" {
		return nil, newRetryableProviderError("kimi coding returned empty content")
	}
	mimeType := "text/plain"
	assetType := "text"
	if looksLikeJSON(content) {
		mimeType = "application/json"
		assetType = "json"
	}
	stageMessage := "Text runtime completed"
	return &ProviderSubmission{
		ProviderJobID: fmt.Sprintf("kimi-coding-%s-%d", req.RuntimeJobID, time.Now().UnixNano()),
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
					"provider_model": firstNonEmpty(resp.Model, model),
					"runtime_task":   req.TaskType,
				},
			}},
			Metadata: map[string]any{
				"provider":      p.name,
				"provider_mode": "sync_anthropic_messages",
				"runtime_task":  req.TaskType,
				"model":         firstNonEmpty(resp.Model, model),
			},
		},
	}, nil
}

func (p *kimiCodingTextProvider) Poll(_ context.Context, _ string) (*ProviderPollResult, error) {
	return nil, newNonRetryableProviderError("kimi coding sync provider does not support polling")
}

func (p *kimiCodingTextProvider) Cancel(_ context.Context, _ string) error { return nil }

func (p *kimiCodingTextProvider) messages(ctx context.Context, payload kimiCodingMessagesRequest) (*kimiCodingMessagesResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("kimi coding request encode failed: %v", err))
	}
	endpoint := strings.TrimRight(firstNonEmpty(strings.TrimSpace(p.cfg.BaseURL), "https://api.kimi.com/coding"), "/") + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("kimi coding request init failed: %v", err))
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("kimi coding request failed: %v", err))
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("kimi coding response read failed: %v", err))
	}
	var out kimiCodingMessagesResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("decode kimi coding response: %v", err))
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(out.Error.Message)
		if message == "" {
			message = fmt.Sprintf("kimi coding request failed: status=%d", resp.StatusCode)
		}
		return nil, classifyKimiCodingError(message, resp.StatusCode)
	}
	return &out, nil
}

func buildKimiCodingMessages(req ProviderJobRequest) []kimiCodingMessage {
	messages := make([]kimiCodingMessage, 0, 4)
	add := func(role, content string) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		messages = append(messages, kimiCodingMessage{Role: role, Content: content})
	}
	add("user", combineNonEmpty("\n\n", req.Input.PromptSnapshot.SystemPrompt, req.Input.PromptSnapshot.StylePrompt, req.Input.PromptSnapshot.UserPrompt, stringMapValue(req.Input.ParamsSnapshot, "prompt")))
	return messages
}

func classifyKimiCodingError(message string, status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return newNonRetryableProviderError("kimi coding authentication failed")
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

func combineNonEmpty(sep string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, sep)
}

type kimiCodingMessagesRequest struct {
	Model       string              `json:"model"`
	Messages    []kimiCodingMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float64             `json:"temperature,omitempty"`
}

type kimiCodingMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type kimiCodingMessagesResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (r kimiCodingMessagesResponse) FirstText() string {
	for _, item := range r.Content {
		if strings.TrimSpace(item.Text) != "" {
			return item.Text
		}
	}
	return ""
}
