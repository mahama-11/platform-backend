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

const defaultGeminiVisualPrompt = `你是电商商品图像理解引擎。只返回合法 JSON，不要 markdown，不要解释。
请从图片中抽取可供用户选择的关键视觉属性，覆盖：主体/商品事实、光影、材质、背景、构图、色彩、风格/效果、可裁剪区域。
返回格式：{"summary":"...","confidence":0.0,"elements":[{"element_type":"product_fact|reference_strategy|lighting|material|background|composition|color|effect|crop_region","element_key":"...","label":"用户可读中文标签","value":{"description":"...","options":[{"id":"keep","label":"保留...","description":"..."},{"id":"replace","label":"替换/调整...","description":"..."},{"id":"drop","label":"不采用","description":"..."}],"bbox":{"x":0,"y":0,"width":1,"height":1}},"confidence":0.0,"readiness":"ready|needs_review"}]}`

type geminiVisualProvider struct {
	name   string
	cfg    config.OpenAICompatibleVisionConfig
	client *http.Client
}

func newGeminiVisualProvider(name string, cfg config.OpenAICompatibleVisionConfig) GenerationProvider {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &geminiVisualProvider{name: name, cfg: cfg, client: &http.Client{Timeout: timeout}}
}

func (p *geminiVisualProvider) Name() string { return p.name }

func (p *geminiVisualProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	if req.TaskType != RuntimeTaskImageUnderstanding {
		return nil, newNonRetryableProviderError(fmt.Sprintf("%s only supports image_understanding", p.name))
	}
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return nil, newNonRetryableProviderError("gemini visual api key is not configured")
	}
	imageURL, err := p.firstUsableImage(ctx, req.Input.SourceAssets)
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(req.Input.PromptSnapshot.UserPrompt)
	if prompt == "" {
		prompt = strings.TrimSpace(stringMapValue(req.Input.ParamsSnapshot, "understanding_prompt"))
	}
	if prompt == "" {
		prompt = defaultGeminiVisualPrompt
	}
	model := firstNonEmpty(stringMapValue(req.Input.ParamsSnapshot, "model"), p.cfg.Model, "gemini-3-flash-preview")
	maxTokens := p.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2500
	}
	payload := geminiChatCompletionRequest{
		Model:       model,
		Temperature: p.cfg.Temperature,
		MaxTokens:   maxTokens,
		ResponseFormat: map[string]string{
			"type": "json_object",
		},
		Messages: []geminiChatMessage{{
			Role: "user",
			Content: []geminiChatContentPart{
				{Type: "text", Text: prompt},
				{Type: "image_url", ImageURL: &geminiImageURL{URL: imageURL}},
			},
		}},
	}
	resp, err := p.chatCompletions(ctx, payload)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(resp.firstContent())
	if content == "" {
		return nil, newRetryableProviderError("gemini visual returned empty content")
	}
	if !json.Valid([]byte(content)) {
		return nil, newNonRetryableProviderError("gemini visual returned non-json content")
	}
	stageMessage := "Image understanding completed by Gemini visual provider"
	return &ProviderSubmission{
		ProviderJobID: fmt.Sprintf("%s-%s-%d", p.name, req.RuntimeJobID, time.Now().UnixNano()),
		Stage:         "provider_completed",
		StageMessage:  stageMessage,
		Completion: &ProviderCompletion{
			Status:       "completed",
			Progress:     100,
			StageMessage: stageMessage,
			Variants: []ProviderResultVariant{{
				Index:      0,
				AssetType:  "json",
				InlineData: content,
				MimeType:   "application/json",
				Metadata: map[string]any{
					"provider":       p.name,
					"provider_model": resp.Model,
					"output_shape":   "json_object",
				},
			}},
			Metadata: map[string]any{
				"provider":      p.name,
				"provider_mode": "sync_chat_completion",
				"task_type":     req.TaskType,
				"model":         resp.Model,
			},
		},
	}, nil
}

func (p *geminiVisualProvider) Poll(_ context.Context, _ string) (*ProviderPollResult, error) {
	return nil, newNonRetryableProviderError("gemini visual sync provider does not support polling")
}

func (p *geminiVisualProvider) Cancel(_ context.Context, _ string) error { return nil }

func (p *geminiVisualProvider) firstUsableImage(ctx context.Context, assets []ProviderSourceAsset) (string, error) {
	if len(assets) == 0 {
		return "", newNonRetryableProviderError("source image is required for gemini visual understanding")
	}
	volcCompat := &volcengineImageProvider{client: p.client}
	for _, asset := range assets {
		image, err := volcCompat.buildVolcengineSingleImage(ctx, asset)
		if err != nil {
			continue
		}
		if strings.TrimSpace(image) != "" {
			return image, nil
		}
	}
	return "", newNonRetryableProviderError("no usable source image found for gemini visual understanding")
}

func (p *geminiVisualProvider) chatCompletions(ctx context.Context, payload geminiChatCompletionRequest) (*geminiChatCompletionResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("gemini visual request encode failed: %v", err))
	}
	endpoint := strings.TrimRight(firstNonEmpty(p.cfg.BaseURL, "https://xingjiabiapi.org"), "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("gemini visual request init failed: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("gemini visual request failed: %v", err))
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("gemini visual response read failed: %v", err))
	}
	var out geminiChatCompletionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("decode gemini visual response: %v", err))
	}
	if resp.StatusCode >= 400 {
		msg := out.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("status=%d", resp.StatusCode)
		}
		return nil, classifyGeminiVisualError(msg, resp.StatusCode)
	}
	return &out, nil
}

func classifyGeminiVisualError(message string, status int) error {
	prefix := "gemini visual understanding: "
	lower := strings.ToLower(strings.TrimSpace(message))
	if status == http.StatusTooManyRequests || status >= 500 || strings.Contains(lower, "timeout") || strings.Contains(lower, "temporarily") {
		return newRetryableProviderError(prefix + message)
	}
	return newNonRetryableProviderError(prefix + message)
}

type geminiChatCompletionRequest struct {
	Model          string              `json:"model"`
	Temperature    float64             `json:"temperature"`
	MaxTokens      int                 `json:"max_tokens"`
	ResponseFormat map[string]string   `json:"response_format,omitempty"`
	Messages       []geminiChatMessage `json:"messages"`
}

type geminiChatMessage struct {
	Role    string                  `json:"role"`
	Content []geminiChatContentPart `json:"content"`
}

type geminiChatContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *geminiImageURL `json:"image_url,omitempty"`
}

type geminiImageURL struct {
	URL string `json:"url"`
}

type geminiChatCompletionResponse struct {
	Model   string              `json:"model"`
	Choices []geminiChoice      `json:"choices"`
	Error   geminiErrorResponse `json:"error,omitempty"`
}

type geminiChoice struct {
	Message geminiChoiceMessage `json:"message"`
}

type geminiChoiceMessage struct {
	Content string `json:"content"`
}

type geminiErrorResponse struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func (r geminiChatCompletionResponse) firstContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}
