package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"platform-service/internal/config"
)

type comfyUIBridgeProvider struct {
	name   string
	cfg    config.ComfyUIBridgeConfig
	client *http.Client
}

func newComfyUIBridgeProvider(name string, cfg config.ComfyUIBridgeConfig) GenerationProvider {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &comfyUIBridgeProvider{
		name:   name,
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

func (p *comfyUIBridgeProvider) Name() string { return p.name }

func (p *comfyUIBridgeProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	if strings.TrimSpace(p.cfg.BaseURL) == "" {
		return nil, newNonRetryableProviderError("comfyui bridge base url is not configured")
	}
	prompt := buildVolcenginePrompt(req)
	if prompt == "" && req.TaskType != "image_understanding" {
		return nil, newNonRetryableProviderError("prompt is required for comfyui bridge image generation")
	}
	params := cloneMap(req.Input.ParamsSnapshot)
	if prompt != "" {
		params["prompt"] = prompt
	}
	if req.CallbackURL != "" {
		params["callback_url"] = req.CallbackURL
	}
	if workflowID := firstNonEmpty(stringMapValue(req.Input.ParamsSnapshot, "workflow_id"), p.cfg.DefaultWorkflowID); workflowID != "" {
		params["workflow_id"] = workflowID
	}
	path := "/generate/text"
	switch {
	case req.TaskType == "image_understanding":
		imagePayload, err := p.buildComfyUnderstandingImagePayload(req.Input.SourceAssets)
		if err != nil {
			return nil, err
		}
		params["image"] = imagePayload
		if _, ok := params["max_new_tokens"]; !ok {
			params["max_new_tokens"] = 512
		}
		path = "/generate/understand"
	case req.Input.InputMode == "multi_image":
		images, err := p.buildComfyMultiImagePayload(req.Input.SourceAssets)
		if err != nil {
			return nil, err
		}
		params["images"] = images
		path = "/generate/multi-image"
	case len(req.Input.SourceAssets) > 0 || req.Input.InputMode == "image_to_image":
		imagePayload, err := p.buildComfyImagePayload(req.Input.SourceAssets)
		if err != nil {
			return nil, err
		}
		params["image"] = imagePayload
		path = "/generate/image"
	}
	var resp comfyTaskSubmissionResponse
	if err := p.postJSON(ctx, path, params, &resp); err != nil {
		return nil, err
	}
	taskID := firstNonEmpty(resp.TaskID, resp.Data.TaskID)
	if taskID == "" {
		return nil, newRetryableProviderError("comfyui bridge did not return task_id")
	}
	return &ProviderSubmission{
		ProviderJobID: taskID,
		Stage:         "provider_accepted",
		StageMessage:  defaultString(resp.Message, "Image generation accepted"),
		EtaSeconds:    15,
	}, nil
}

func (p *comfyUIBridgeProvider) Poll(ctx context.Context, providerJobID string) (*ProviderPollResult, error) {
	if strings.TrimSpace(providerJobID) == "" {
		return nil, newNonRetryableProviderError("provider job id is required")
	}
	var resp comfyTaskStatusResponse
	if err := p.getJSON(ctx, "/tasks/"+providerJobID, &resp); err != nil {
		return nil, err
	}
	status := strings.ToLower(firstNonEmpty(resp.Status, resp.Data.Status))
	progress := maxInt(progressToInt(resp.Progress), progressToInt(resp.Data.Progress))
	switch status {
	case "completed", "success":
		images := resp.ResultImages
		if len(images) == 0 {
			images = resp.Data.ResultImages
		}
		variants := make([]ProviderResultVariant, 0, len(images))
		for idx, item := range images {
			dataURL := p.normalizeResultImage(item)
			if dataURL == "" {
				continue
			}
			variants = append(variants, ProviderResultVariant{
				Index:      idx,
				InlineData: dataURL,
				MimeType:   outputFormatToMimeType(firstNonEmpty(resp.OutputFormat, p.cfg.DefaultOutputFormat), ""),
				Metadata: map[string]any{
					"provider":  p.name,
					"task_id":   providerJobID,
					"prompt_id": firstNonEmpty(resp.PromptID, resp.Data.PromptID),
				},
			})
		}
		metadata := map[string]any{
			"provider": p.name,
			"task_id":  providerJobID,
		}
		if promptID := strings.TrimSpace(firstNonEmpty(resp.PromptID, resp.Data.PromptID)); promptID != "" {
			metadata["prompt_id"] = promptID
		}
		if len(variants) == 0 {
			resultText := strings.TrimSpace(firstNonEmpty(resp.ResultText, resp.Data.ResultText))
			if strings.TrimSpace(resultText) == "" {
				return nil, newRetryableProviderError("comfyui bridge completed without result images")
			}
			assetType := "text"
			mimeType := "text/plain"
			if json.Valid([]byte(resultText)) {
				assetType = "json"
				mimeType = "application/json"
			}
			variants = append(variants, ProviderResultVariant{
				Index:      0,
				AssetType:  assetType,
				InlineData: resultText,
				MimeType:   mimeType,
				Metadata: map[string]any{
					"provider": p.name,
					"task_id":  providerJobID,
				},
			})
		}
		return &ProviderPollResult{
			Status:       "completed",
			Stage:        "provider_completed",
			StageMessage: defaultString(resp.Message, "Image generation completed"),
			Progress:     100,
			Completion: &ProviderCompletion{
				Status:       "completed",
				Progress:     100,
				StageMessage: defaultString(resp.Message, "Image generation completed"),
				Variants:     variants,
				Metadata:     metadata,
			},
		}, nil
	case "failed", "error":
		return &ProviderPollResult{
			Status:       "failed",
			Stage:        "provider_failed",
			StageMessage: defaultString(resp.Error, "Image generation failed"),
			ErrorClass:   "non_retryable_provider",
			ErrorCode:    "PROVIDER_TASK_FAILED",
			ErrorMessage: defaultString(resp.Error, "Image generation failed"),
		}, nil
	default:
		if progress <= 0 {
			progress = 10
		}
		return &ProviderPollResult{
			Status:       "processing",
			Stage:        "provider_running",
			StageMessage: defaultString(resp.Message, "Image generation processing"),
			Progress:     progress,
			EtaSeconds:   5,
		}, nil
	}
}

func (p *comfyUIBridgeProvider) Cancel(_ context.Context, _ string) error { return nil }

func (p *comfyUIBridgeProvider) buildComfyImagePayload(assets []ProviderSourceAsset) (string, error) {
	for _, asset := range assets {
		source := firstNonEmpty(strings.TrimSpace(asset.SourceURL), strings.TrimSpace(asset.PreviewURL))
		if source == "" {
			continue
		}
		if strings.HasPrefix(source, "data:") {
			payload, err := extractDataURLPayload(source)
			if err != nil {
				return "", newNonRetryableProviderError(err.Error())
			}
			return payload, nil
		}
		if isBase64Payload(source) {
			return strings.TrimSpace(source), nil
		}
	}
	return "", newNonRetryableProviderError("no usable image payload found for comfyui bridge image generation")
}

func (p *comfyUIBridgeProvider) buildComfyUnderstandingImagePayload(assets []ProviderSourceAsset) (string, error) {
	for _, asset := range assets {
		source := firstNonEmpty(strings.TrimSpace(asset.SourceURL), strings.TrimSpace(asset.PreviewURL))
		if source == "" {
			continue
		}
		if strings.HasPrefix(source, "data:") {
			if _, err := extractDataURLPayload(source); err != nil {
				return "", newNonRetryableProviderError(err.Error())
			}
			return normalizeDataURL(source), nil
		}
		if isBase64Payload(source) {
			mimeType := firstNonEmpty(strings.TrimSpace(asset.MimeType), "image/png")
			return "data:" + mimeType + ";base64," + strings.TrimSpace(source), nil
		}
	}
	return "", newNonRetryableProviderError("no usable image payload found for comfyui bridge image understanding")
}

func (p *comfyUIBridgeProvider) buildComfyMultiImagePayload(assets []ProviderSourceAsset) ([]string, error) {
	images := make([]string, 0, len(assets))
	for _, asset := range assets {
		payload, err := p.buildComfyImagePayload([]ProviderSourceAsset{asset})
		if err != nil || payload == "" {
			continue
		}
		images = append(images, payload)
		if len(images) == 4 {
			return images, nil
		}
	}
	return nil, newNonRetryableProviderError(fmt.Sprintf("comfyui bridge multi-image generation requires exactly 4 usable images, got %d", len(images)))
}

func (p *comfyUIBridgeProvider) normalizeResultImage(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "data:") {
		return normalizeDataURL(trimmed)
	}
	if isBase64Payload(trimmed) {
		mimeType := outputFormatToMimeType(p.cfg.DefaultOutputFormat, "")
		if mimeType == "" {
			mimeType = "image/png"
		}
		return "data:" + mimeType + ";base64," + trimmed
	}
	return ""
}

func (p *comfyUIBridgeProvider) postJSON(ctx context.Context, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return newRetryableProviderError(fmt.Sprintf("encode comfyui bridge request: %v", err))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return newRetryableProviderError(fmt.Sprintf("init comfyui bridge request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(p.cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return newRetryableProviderError(fmt.Sprintf("comfyui bridge request failed: %v", err))
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return newRetryableProviderError(fmt.Sprintf("read comfyui bridge response: %v", err))
	}
	if resp.StatusCode >= 400 {
		return classifyVolcengineError(fmt.Sprintf("comfyui bridge request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody))), resp.StatusCode)
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return newRetryableProviderError(fmt.Sprintf("decode comfyui bridge response: %v body=%s", err, trimBodyForError(respBody)))
		}
	}
	return nil
}

func (p *comfyUIBridgeProvider) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.cfg.BaseURL, "/")+path, nil)
	if err != nil {
		return newRetryableProviderError(fmt.Sprintf("init comfyui bridge poll request: %v", err))
	}
	if strings.TrimSpace(p.cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return newRetryableProviderError(fmt.Sprintf("comfyui bridge poll failed: %v", err))
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return newRetryableProviderError(fmt.Sprintf("read comfyui bridge poll response: %v", err))
	}
	if resp.StatusCode >= 400 {
		return classifyVolcengineError(fmt.Sprintf("comfyui bridge poll failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody))), resp.StatusCode)
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return newRetryableProviderError(fmt.Sprintf("decode comfyui bridge poll response: %v body=%s", err, trimBodyForError(respBody)))
	}
	return nil
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func progressToInt(value float64) int {
	if value <= 0 {
		return 0
	}
	return int(math.Round(value))
}

func trimBodyForError(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= 600 {
		return text
	}
	return text[:600] + "...(truncated)"
}

type comfyTaskSubmissionResponse struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
	Data    struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

type comfyTaskStatusResponse struct {
	Status       string   `json:"status"`
	PromptID     string   `json:"prompt_id"`
	Progress     float64  `json:"progress"`
	Message      string   `json:"message"`
	Error        string   `json:"error"`
	OutputFormat string   `json:"output_format"`
	ResultImages []string `json:"result_images"`
	ResultText   string   `json:"result_text"`
	Data         struct {
		Status       string   `json:"status"`
		PromptID     string   `json:"prompt_id"`
		Progress     float64  `json:"progress"`
		ResultImages []string `json:"result_images"`
		ResultText   string   `json:"result_text"`
	} `json:"data"`
}

var _ = base64.StdEncoding
