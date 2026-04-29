package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"platform-service/internal/config"
)

type volcengineImageProvider struct {
	name   string
	cfg    config.VolcengineConfig
	client *http.Client
}

func newVolcengineImageProvider(name string, cfg config.VolcengineConfig) GenerationProvider {
	return &volcengineImageProvider{
		name:   name,
		cfg:    cfg,
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

func (p *volcengineImageProvider) Name() string { return p.name }

func (p *volcengineImageProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return nil, newNonRetryableProviderError("volcengine api key is not configured")
	}

	prompt := buildVolcenginePrompt(req)
	if prompt == "" {
		return nil, newNonRetryableProviderError("prompt is required for volcengine image generation")
	}

	outputFormat, err := normalizeVolcengineOutputFormat(stringMapValue(req.Input.ParamsSnapshot, "output_format"))
	if err != nil {
		return nil, err
	}

	size := firstNonEmpty(
		stringMapValue(req.Input.ParamsSnapshot, "size"),
		p.cfg.ImageSize,
		"2K",
	)
	modelID := firstNonEmpty(
		stringMapValue(req.Input.ParamsSnapshot, "model"),
		p.cfg.ImageModel,
		"doubao-seedream-5-0-260128",
	)
	if req.Input.PromptSnapshot.Provider == p.name && strings.TrimSpace(req.Input.PromptSnapshot.Model) != "" {
		modelID = req.Input.PromptSnapshot.Model
	}

	requestedCount := req.Input.RequestedVariants
	if requestedCount <= 0 {
		requestedCount = 1
	}
	if requestedCount > 4 {
		requestedCount = 4
	}

	generateReq := volcengineGenerateImagesRequest{
		Model:          modelID,
		Prompt:         prompt,
		Size:           size,
		ResponseFormat: "url",
		Watermark:      p.cfg.Watermark,
	}
	if outputFormat != "" {
		generateReq.OutputFormat = outputFormat
	}
	if len(req.Input.SourceAssets) > 0 {
		imageInput, imageErr := p.buildVolcengineImageInput(ctx, req.Input.SourceAssets)
		if imageErr != nil {
			return nil, imageErr
		}
		generateReq.Image = imageInput
	}

	resp, callErr := p.generateImages(ctx, generateReq)
	if callErr != nil {
		return nil, callErr
	}
	if resp.Error != nil {
		return nil, classifyVolcengineError(resp.Error.Message, 400)
	}

	variants := make([]ProviderResultVariant, 0, minInt(requestedCount, len(resp.Data)))
	for _, item := range resp.Data {
		if item.URL == nil || strings.TrimSpace(*item.URL) == "" {
			continue
		}
		sourceURL := strings.TrimSpace(*item.URL)
		variants = append(variants, ProviderResultVariant{
			Index:      len(variants),
			SourceURL:  sourceURL,
			PreviewURL: sourceURL,
			MimeType:   outputFormatToMimeType(outputFormat, sourceURL),
			Metadata: map[string]any{
				"provider":        p.name,
				"provider_model":  resp.Model,
				"requested_size":  size,
				"requested_count": requestedCount,
			},
		})
		if len(variants) >= requestedCount {
			break
		}
	}

	if len(variants) == 0 {
		return nil, newRetryableProviderError("volcengine returned no images")
	}

	stageMessage := fmt.Sprintf("Image generation completed (%d/%d)", len(variants), requestedCount)
	return &ProviderSubmission{
		ProviderJobID: fmt.Sprintf("volcengine-%s-%d", req.RuntimeJobID, time.Now().UnixNano()),
		Stage:         "provider_completed",
		StageMessage:  stageMessage,
		Completion: &ProviderCompletion{
			Status:       "completed",
			Progress:     100,
			StageMessage: stageMessage,
			Variants:     variants,
			Metadata: map[string]any{
				"provider":        p.name,
				"provider_mode":   "sync_generate",
				"input_mode":      req.Input.InputMode,
				"requested_count": requestedCount,
				"actual_count":    len(variants),
			},
		},
	}, nil
}

func (p *volcengineImageProvider) Poll(_ context.Context, _ string) (*ProviderPollResult, error) {
	return nil, newNonRetryableProviderError("volcengine sync provider does not support polling")
}

func (p *volcengineImageProvider) Cancel(_ context.Context, _ string) error { return nil }

func (p *volcengineImageProvider) generateImages(ctx context.Context, payload volcengineGenerateImagesRequest) (*volcengineGenerateImagesResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("volcengine request encode failed: %v", err))
	}
	endpoint := strings.TrimRight(firstNonEmpty(strings.TrimSpace(p.cfg.BaseURL), "https://ark.cn-beijing.volces.com/api/v3"), "/") + "/images/generations"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("volcengine request init failed: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("volcengine request failed: %v", err))
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 50MB max API response
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("volcengine response read failed: %v", err))
	}
	var out volcengineGenerateImagesResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("decode volcengine response: %v", err))
	}
	if resp.StatusCode >= 400 {
		if out.Error != nil && out.Error.Message != "" {
			return nil, classifyVolcengineError(out.Error.Message, resp.StatusCode)
		}
		return nil, classifyVolcengineError(fmt.Sprintf("volcengine request failed: status=%d", resp.StatusCode), resp.StatusCode)
	}
	return &out, nil
}

func buildVolcenginePrompt(req ProviderJobRequest) string {
	parts := make([]string, 0, 4)
	appendUnique := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		for _, existing := range parts {
			if existing == trimmed {
				return
			}
		}
		parts = append(parts, trimmed)
	}
	appendUnique(req.Input.PromptSnapshot.SystemPrompt)
	appendUnique(req.Input.PromptSnapshot.StylePrompt)
	appendUnique(req.Input.PromptSnapshot.UserPrompt)
	appendUnique(stringMapValue(req.Input.ParamsSnapshot, "prompt"))
	return strings.Join(parts, "\n\n")
}

func (p *volcengineImageProvider) buildVolcengineImageInput(ctx context.Context, assets []ProviderSourceAsset) (string, error) {
	for _, asset := range assets {
		image, err := p.buildVolcengineSingleImage(ctx, asset)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(image) != "" {
			return image, nil
		}
	}
	return "", newNonRetryableProviderError("no usable source image found for image-to-image generation")
}

func (p *volcengineImageProvider) buildVolcengineSingleImage(ctx context.Context, asset ProviderSourceAsset) (string, error) {
	source := firstNonEmpty(strings.TrimSpace(asset.SourceURL), strings.TrimSpace(asset.PreviewURL))
	if source == "" {
		return "", newNonRetryableProviderError(fmt.Sprintf("source asset %s has no usable image payload", asset.ID))
	}
	if strings.HasPrefix(source, "data:") {
		if _, err := extractDataURLPayload(source); err != nil {
			return "", err
		}
		return normalizeDataURL(source), nil
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return p.fetchImageAsDataURL(ctx, source)
	}
	if isBase64Payload(source) {
		mimeType := normalizeImageMimeType(asset.MimeType)
		if mimeType == "" {
			mimeType = "image/png"
		}
		return "data:" + mimeType + ";base64," + strings.TrimSpace(source), nil
	}
	return "", newNonRetryableProviderError(fmt.Sprintf("unsupported image source format for asset %s", asset.ID))
}

func extractDataURLPayload(value string) (string, error) {
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid data url payload")
	}
	payload := strings.TrimSpace(parts[1])
	if payload == "" {
		return "", fmt.Errorf("empty data url payload")
	}
	if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
		return "", fmt.Errorf("invalid base64 image payload: %w", err)
	}
	return payload, nil
}

func (p *volcengineImageProvider) fetchImageAsDataURL(ctx context.Context, sourceURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", newRetryableProviderError(fmt.Sprintf("init source image request failed: %v", err))
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", newRetryableProviderError(fmt.Sprintf("fetch source image failed: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", classifyVolcengineError(fmt.Sprintf("fetch source image failed: status=%d", resp.StatusCode), resp.StatusCode)
	}
	// 限制下载大小为 20MB，防止恶意或异常大图 OOM
	const maxImageSize = 20 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize+1))
	if err != nil {
		return "", newRetryableProviderError(fmt.Sprintf("read source image failed: %v", err))
	}
	if len(body) > maxImageSize {
		return "", newNonRetryableProviderError("source image exceeds 20MB size limit")
	}
	if len(body) == 0 {
		return "", newNonRetryableProviderError("source image response is empty")
	}
	mimeType := normalizeImageMimeType(resp.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = normalizeImageMimeType(http.DetectContentType(body))
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func normalizeDataURL(value string) string {
	parts := strings.SplitN(strings.TrimSpace(value), ",", 2)
	if len(parts) != 2 {
		return strings.TrimSpace(value)
	}
	header := strings.ToLower(strings.TrimSpace(parts[0]))
	payload := strings.TrimSpace(parts[1])
	return header + "," + payload
}

func isBase64Payload(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.Contains(trimmed, ",") || strings.Contains(trimmed, "://") {
		return false
	}
	if len(trimmed) < 32 {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(trimmed)
	return err == nil
}

func normalizeImageMimeType(value string) string {
	mimeType := strings.ToLower(strings.TrimSpace(value))
	if mimeType == "" {
		return ""
	}
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp", "image/gif", "image/bmp":
		return mimeType
	default:
		return ""
	}
}

func normalizeVolcengineOutputFormat(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "png":
		return "png", nil
	case "jpeg", "jpg":
		return "jpeg", nil
	default:
		return "", newNonRetryableProviderError(fmt.Sprintf("unsupported volcengine output format: %s", value))
	}
}

func outputFormatToMimeType(format, sourceURL string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	}
	lower := strings.ToLower(sourceURL)
	switch {
	case strings.Contains(lower, ".jpg"), strings.Contains(lower, ".jpeg"):
		return "image/jpeg"
	case strings.Contains(lower, ".webp"):
		return "image/webp"
	default:
		return "image/png"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringMapValue(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func classifyVolcengineError(message string, status int) error {
	lower := strings.ToLower(strings.TrimSpace(message))
	if status == http.StatusTooManyRequests || status >= 500 {
		return newRetryableProviderError("volcengine generate images: " + message)
	}
	if strings.Contains(lower, "parameter `") ||
		strings.Contains(lower, "not valid") ||
		strings.Contains(lower, "not supported by the current model") ||
		strings.Contains(lower, "has not activated the model") ||
		strings.Contains(lower, "api key") {
		return newNonRetryableProviderError("volcengine generate images: " + message)
	}
	if status >= 400 {
		return newNonRetryableProviderError("volcengine generate images: " + message)
	}
	return newRetryableProviderError("volcengine generate images: " + message)
}

type volcengineGenerateImagesRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Image          string `json:"image,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Size           string `json:"size,omitempty"`
	Watermark      bool   `json:"watermark,omitempty"`
	OutputFormat   string `json:"output_format,omitempty"`
}

type volcengineGenerateImagesResponse struct {
	Model string                        `json:"model"`
	Data  []volcengineGenerateImageData `json:"data"`
	Error *volcengineGenerateImageError `json:"error,omitempty"`
}

type volcengineGenerateImageData struct {
	URL *string `json:"url,omitempty"`
}

type volcengineGenerateImageError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
