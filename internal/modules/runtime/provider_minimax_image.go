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

type minimaxImageProvider struct {
	name   string
	cfg    config.MinimaxImageConfig
	client *http.Client
}

func newMinimaxImageProvider(name string, cfg config.MinimaxImageConfig) GenerationProvider {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &minimaxImageProvider{name: name, cfg: cfg, client: &http.Client{Timeout: timeout}}
}

func (p *minimaxImageProvider) Name() string { return p.name }

func (p *minimaxImageProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	if req.TaskType != "" && req.TaskType != RuntimeTaskImageGeneration && req.TaskType != RuntimeTaskImageInpainting {
		return nil, newNonRetryableProviderError(fmt.Sprintf("%s only supports image_generation/image_inpainting", p.name))
	}
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return nil, newNonRetryableProviderError("minimax image api key is not configured")
	}
	prompt := buildVolcenginePrompt(req)
	if prompt == "" {
		return nil, newNonRetryableProviderError("prompt is required for minimax image generation")
	}
	model := firstNonEmpty(stringMapValue(req.Input.ParamsSnapshot, "model"), req.Input.PromptSnapshot.Model, p.cfg.Model, "image-01")
	aspectRatio := normalizeMinimaxAspectRatio(firstNonEmpty(
		stringMapValue(req.Input.ParamsSnapshot, "aspect_ratio"),
		stringMapValue(req.Input.ParamsSnapshot, "aspectRatio"),
		stringMapValue(req.Input.ParamsSnapshot, "ratio"),
		p.cfg.DefaultAspectRatio,
		"1:1",
	))
	payload := minimaxImageGenerationRequest{
		Model:          model,
		Prompt:         prompt,
		AspectRatio:    aspectRatio,
		ResponseFormat: "base64",
	}
	if len(req.Input.SourceAssets) > 0 || req.Input.InputMode == "image_to_image" || req.Input.InputMode == "image_edit" || req.TaskType == RuntimeTaskImageInpainting {
		reference, err := buildMinimaxSubjectReference(req.Input.SourceAssets, req.Input.ParamsSnapshot)
		if err != nil {
			return nil, err
		}
		payload.SubjectReference = reference
	}
	resp, err := p.generate(ctx, payload)
	if err != nil {
		return nil, err
	}
	if resp.BaseResp.StatusCode != 0 {
		message := strings.TrimSpace(resp.BaseResp.StatusMsg)
		if message == "" {
			message = fmt.Sprintf("minimax image provider returned status_code=%d", resp.BaseResp.StatusCode)
		}
		return nil, classifyMinimaxBusinessError(message, resp.BaseResp.StatusCode)
	}
	if len(resp.Data.ImageBase64) == 0 {
		return nil, newRetryableProviderError("minimax image provider returned no images")
	}
	mimeType := firstNonEmpty(stringMapValue(req.Input.ParamsSnapshot, "output_mime_type"), "image/jpeg")
	variants := make([]ProviderResultVariant, 0, len(resp.Data.ImageBase64))
	for idx, item := range resp.Data.ImageBase64 {
		payload := strings.TrimSpace(item)
		if payload == "" {
			continue
		}
		dataURL := normalizeDataURL(payload)
		if !strings.HasPrefix(strings.ToLower(dataURL), "data:image/") {
			dataURL = "data:" + mimeType + ";base64," + payload
		}
		variants = append(variants, ProviderResultVariant{
			Index:      idx,
			InlineData: dataURL,
			MimeType:   mimeTypeFromDataURL(dataURL),
			Metadata: map[string]any{
				"provider":       p.name,
				"provider_model": model,
				"input_mode":     req.Input.InputMode,
				"response_id":    resp.ID,
			},
		})
	}
	if len(variants) == 0 {
		return nil, newRetryableProviderError("minimax image provider returned blank images")
	}
	stageMessage := "Image generation completed by MiniMax image provider"
	return &ProviderSubmission{
		ProviderJobID: fmt.Sprintf("%s-%s-%d", p.name, req.RuntimeJobID, time.Now().UnixNano()),
		Stage:         "provider_completed",
		StageMessage:  stageMessage,
		Completion: &ProviderCompletion{
			Status:       "completed",
			Progress:     100,
			StageMessage: stageMessage,
			Variants:     variants,
			Metadata: map[string]any{
				"provider":      p.name,
				"provider_mode": "sync_image_generation",
				"task_type":     req.TaskType,
				"model":         model,
				"response_id":   resp.ID,
			},
		},
	}, nil
}

func (p *minimaxImageProvider) Poll(_ context.Context, _ string) (*ProviderPollResult, error) {
	return nil, newNonRetryableProviderError("minimax image sync provider does not support polling")
}

func (p *minimaxImageProvider) Cancel(_ context.Context, _ string) error { return nil }

func (p *minimaxImageProvider) generate(ctx context.Context, payload minimaxImageGenerationRequest) (*minimaxImageGenerationResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("minimax image request encode failed: %v", err))
	}
	endpoint := strings.TrimRight(firstNonEmpty(strings.TrimSpace(p.cfg.BaseURL), "https://api.minimaxi.com/v1"), "/") + "/image_generation"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("minimax image request init failed: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("minimax image request failed: %v", err))
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("minimax image response read failed: %v", err))
	}
	var out minimaxImageGenerationResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("decode minimax image response: %v", err))
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(out.BaseResp.StatusMsg)
		if message == "" {
			message = fmt.Sprintf("minimax image request failed: status=%d", resp.StatusCode)
		}
		return nil, classifyMinimaxError(message, resp.StatusCode)
	}
	return &out, nil
}

type minimaxImageGenerationRequest struct {
	Model            string                    `json:"model"`
	Prompt           string                    `json:"prompt"`
	AspectRatio      string                    `json:"aspect_ratio,omitempty"`
	SubjectReference []minimaxSubjectReference `json:"subject_reference,omitempty"`
	ResponseFormat   string                    `json:"response_format,omitempty"`
}

type minimaxSubjectReference struct {
	Type      string `json:"type"`
	ImageFile string `json:"image_file"`
}

type minimaxImageGenerationResponse struct {
	ID   string `json:"id"`
	Data struct {
		ImageBase64 []string `json:"image_base64"`
	} `json:"data"`
	Metadata map[string]any `json:"metadata"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func buildMinimaxSubjectReference(assets []ProviderSourceAsset, params map[string]any) ([]minimaxSubjectReference, error) {
	refs := make([]minimaxSubjectReference, 0, 1)
	subjectType := firstNonEmpty(stringMapValue(params, "minimax_subject_type"), stringMapValue(params, "subject_type"), "character")
	for _, asset := range assets {
		source := strings.TrimSpace(firstNonEmpty(asset.SourceURL, asset.PreviewURL))
		if source == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(source), "http://") || strings.HasPrefix(strings.ToLower(source), "https://") || strings.HasPrefix(strings.ToLower(source), "data:image/") {
			refs = append(refs, minimaxSubjectReference{Type: subjectType, ImageFile: source})
			break
		}
	}
	if len(refs) == 0 {
		return nil, newNonRetryableProviderError("no usable image reference found for minimax image generation")
	}
	return refs, nil
}

func normalizeMinimaxAspectRatio(value string) string {
	switch strings.TrimSpace(value) {
	case "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "21:9":
		return strings.TrimSpace(value)
	default:
		return "1:1"
	}
}

func classifyMinimaxBusinessError(message string, code int) error {
	if code == 1002 || code == 1004 || code == 1013 || code == 2013 {
		return newNonRetryableProviderError(message)
	}
	return newRetryableProviderError(message)
}
