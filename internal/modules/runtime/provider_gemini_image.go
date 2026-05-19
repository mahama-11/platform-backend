package runtime

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"platform-service/internal/config"
)

var geminiImageDataURLPattern = regexp.MustCompile(`data:image/(png|jpeg|jpg|webp);base64,[A-Za-z0-9+/=]+`)

type geminiImageProvider struct {
	name   string
	cfg    config.OpenAICompatibleVisionConfig
	visual *geminiVisualProvider
}

func newGeminiImageProvider(name string, cfg config.OpenAICompatibleVisionConfig) GenerationProvider {
	return &geminiImageProvider{
		name:   name,
		cfg:    cfg,
		visual: newGeminiVisualProvider(name, cfg).(*geminiVisualProvider),
	}
}

func (p *geminiImageProvider) Name() string { return p.name }

func (p *geminiImageProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	if req.TaskType != "" && req.TaskType != RuntimeTaskImageGeneration && req.TaskType != RuntimeTaskImageInpainting {
		return nil, newNonRetryableProviderError(fmt.Sprintf("%s only supports image_generation/image_inpainting", p.name))
	}
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return nil, newNonRetryableProviderError("gemini image api key is not configured")
	}
	prompt := buildVolcenginePrompt(req)
	if prompt == "" {
		return nil, newNonRetryableProviderError("prompt is required for gemini image generation")
	}
	model := firstNonEmpty(stringMapValue(req.Input.ParamsSnapshot, "model"), p.cfg.Model, "gemini-3-pro-image-preview")
	maxTokens := p.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1500
	}
	content := []geminiChatContentPart{{Type: "text", Text: prompt}}
	if len(req.Input.SourceAssets) > 0 || req.Input.InputMode == "image_to_image" || req.Input.InputMode == "image_edit" || req.TaskType == RuntimeTaskImageInpainting {
		imageURL, err := p.visual.firstUsableImage(ctx, req.Input.SourceAssets)
		if err != nil {
			return nil, err
		}
		content = append(content, geminiChatContentPart{Type: "image_url", ImageURL: &geminiImageURL{URL: imageURL}})
	}
	payload := geminiChatCompletionRequest{
		Model:       model,
		Temperature: p.cfg.Temperature,
		MaxTokens:   maxTokens,
		Messages: []geminiChatMessage{{
			Role:    "user",
			Content: content,
		}},
	}
	resp, err := p.visual.chatCompletions(ctx, payload)
	if err != nil {
		return nil, err
	}
	contentText := strings.TrimSpace(resp.firstContent())
	dataURL := extractGeminiImageDataURL(contentText)
	if dataURL == "" {
		return nil, newRetryableProviderError("gemini image provider returned no image data")
	}
	mimeType := mimeTypeFromDataURL(dataURL)
	stageMessage := "Image generation completed by Gemini image provider"
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
				InlineData: dataURL,
				MimeType:   mimeType,
				Metadata: map[string]any{
					"provider":       p.name,
					"provider_model": resp.Model,
					"input_mode":     req.Input.InputMode,
				},
			}},
			Metadata: map[string]any{
				"provider":      p.name,
				"provider_mode": "sync_chat_image_generation",
				"task_type":     req.TaskType,
				"model":         resp.Model,
			},
		},
	}, nil
}

func (p *geminiImageProvider) Poll(_ context.Context, _ string) (*ProviderPollResult, error) {
	return nil, newNonRetryableProviderError("gemini image sync provider does not support polling")
}

func (p *geminiImageProvider) Cancel(_ context.Context, _ string) error { return nil }

func extractGeminiImageDataURL(content string) string {
	matched := geminiImageDataURLPattern.FindString(strings.TrimSpace(content))
	if matched == "" {
		return ""
	}
	return normalizeDataURL(matched)
}

func mimeTypeFromDataURL(dataURL string) string {
	trimmed := strings.ToLower(strings.TrimSpace(dataURL))
	if strings.HasPrefix(trimmed, "data:image/jpeg") || strings.HasPrefix(trimmed, "data:image/jpg") {
		return "image/jpeg"
	}
	if strings.HasPrefix(trimmed, "data:image/webp") {
		return "image/webp"
	}
	if strings.HasPrefix(trimmed, "data:image/png") {
		return "image/png"
	}
	return outputFormatToMimeType("", dataURL)
}
