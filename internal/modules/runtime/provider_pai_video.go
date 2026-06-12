package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"platform-service/internal/config"
)

type VideoProviderCapabilities interface {
	Balance(ctx context.Context) (map[string]any, error)
	TTSVoices(ctx context.Context) (map[string]any, error)
	UploadFile(ctx context.Context, kind, filename string, reader io.Reader) (map[string]any, error)
	UploadURL(ctx context.Context, fileURL string) (map[string]any, error)
	ProviderAction(ctx context.Context, action string, payload map[string]any) (map[string]any, error)
}

type paiVideoProvider struct {
	name   string
	cfg    config.PaiVideoConfig
	client *http.Client
}

type paiVideoAPIError struct {
	message   string
	errCode   any
	response  any
	code      string
	retryable bool
}

func (e *paiVideoAPIError) Error() string   { return e.message }
func (e *paiVideoAPIError) Retryable() bool { return e.retryable }
func (e *paiVideoAPIError) Code() string    { return e.code }

func newPaiVideoProvider(name string, cfg config.PaiVideoConfig) GenerationProvider {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &paiVideoProvider{name: name, cfg: cfg, client: &http.Client{Timeout: timeout}}
}

func (p *paiVideoProvider) Name() string { return p.name }

func (p *paiVideoProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	endpoint, err := p.endpointForTask(req.TaskType, req.Input.InputMode)
	if err != nil {
		return nil, err
	}
	payload := cloneMap(req.Input.ParamsSnapshot)
	if endpointSupportsPromptDefault(endpoint) && strings.TrimSpace(stringMapValue(payload, "prompt")) == "" {
		payload["prompt"] = buildVolcenginePrompt(req)
	}
	if endpointSupportsDefaultModel(endpoint) {
		if _, ok := payload["model"]; !ok && strings.TrimSpace(p.cfg.DefaultModel) != "" {
			payload["model"] = p.cfg.DefaultModel
		}
	}
	translateNormalizedPaiVideoPayload(endpoint, payload)
	cleanPaiVideoPayload(endpoint, payload)
	resp, err := p.requestJSON(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, err
	}
	videoID := stringFromAny(firstAny(resp["video_id"], resp["videoId"], resp["id"]))
	if videoID == "" {
		return nil, newRetryableProviderError("pai_video did not return video_id")
	}
	return &ProviderSubmission{
		ProviderJobID: videoID,
		Stage:         "provider_accepted",
		StageMessage:  "Video generation accepted",
		EtaSeconds:    15,
	}, nil
}

func (p *paiVideoProvider) ProviderAction(ctx context.Context, action string, payload map[string]any) (map[string]any, error) {
	endpoint, method, err := p.endpointForProviderAction(action)
	if err != nil {
		return nil, err
	}
	if method == http.MethodGet {
		return p.requestJSON(ctx, method, endpoint, nil)
	}
	requestPayload := cloneMap(payload)
	cleanPaiVideoPayload(endpoint, requestPayload)
	return p.requestJSON(ctx, method, endpoint, requestPayload)
}

func (p *paiVideoProvider) Poll(ctx context.Context, providerJobID string) (*ProviderPollResult, error) {
	if strings.TrimSpace(providerJobID) == "" {
		return nil, newNonRetryableProviderError("provider job id is required")
	}
	resp, err := p.requestJSON(ctx, http.MethodGet, "/video/result/"+url.PathEscape(providerJobID), nil)
	if err != nil {
		var apiErr *paiVideoAPIError
		if strings.Contains(strings.ToLower(err.Error()), "record not found") || errors.As(err, &apiErr) && strings.Contains(strings.ToLower(apiErr.message), "record not found") {
			return &ProviderPollResult{
				Status:       "processing",
				Stage:        "provider_pending_record",
				StageMessage: "Video generation record is not visible yet",
				Progress:     45,
				EtaSeconds:   5,
			}, nil
		}
		return nil, err
	}
	statusCode := intFromAny(resp["status"])
	switch statusCode {
	case 1:
		sourceURL := firstNonEmpty(
			stringFromAny(resp["url"]),
			stringFromAny(resp["video_url"]),
			stringFromAny(resp["output_url"]),
			stringFromAny(resp["result_url"]),
		)
		previewURL := firstNonEmpty(
			stringFromAny(resp["cover_url"]),
			stringFromAny(resp["poster"]),
			stringFromAny(resp["thumbnail_url"]),
			sourceURL,
		)
		if sourceURL == "" {
			return nil, newRetryableProviderError("pai_video completed without result url")
		}
		return &ProviderPollResult{
			Status:       "completed",
			Stage:        "provider_completed",
			StageMessage: "Video generation completed",
			Progress:     100,
			Completion: &ProviderCompletion{
				Status:       "completed",
				Progress:     100,
				StageMessage: "Video generation completed",
				Variants: []ProviderResultVariant{{
					Index:      0,
					SourceURL:  sourceURL,
					PreviewURL: previewURL,
					MimeType:   "video/mp4",
					Metadata: map[string]any{
						"provider":     p.name,
						"provider_job": providerJobID,
						"raw":          resp,
					},
				}},
				Metadata: map[string]any{
					"provider":     p.name,
					"provider_job": providerJobID,
					"raw":          resp,
				},
			},
		}, nil
	case 7, 8:
		message := firstNonEmpty(stringFromAny(resp["error"]), stringFromAny(resp["message"]), "Video generation failed")
		if strings.Contains(strings.ToLower(message), "record not found") {
			return &ProviderPollResult{
				Status:       "processing",
				Stage:        "provider_pending_record",
				StageMessage: "Video generation record is not visible yet",
				Progress:     45,
				EtaSeconds:   5,
			}, nil
		}
		return &ProviderPollResult{
			Status:       "failed",
			Stage:        "provider_failed",
			StageMessage: message,
			ErrorClass:   "non_retryable_provider",
			ErrorCode:    "PROVIDER_TASK_FAILED",
			ErrorMessage: message,
		}, nil
	default:
		progress := progressForPaiVideoStatus(statusCode)
		return &ProviderPollResult{
			Status:       "processing",
			Stage:        "provider_running",
			StageMessage: "Video generation processing",
			Progress:     progress,
			EtaSeconds:   5,
		}, nil
	}
}

func (p *paiVideoProvider) Cancel(_ context.Context, _ string) error { return nil }

func (p *paiVideoProvider) Balance(ctx context.Context) (map[string]any, error) {
	return p.requestJSON(ctx, http.MethodGet, "/account/balance", nil)
}

func (p *paiVideoProvider) TTSVoices(ctx context.Context) (map[string]any, error) {
	return p.requestJSON(ctx, http.MethodGet, "/video/lip_sync/tts_list?page_num=1&page_size=50", nil)
}

func (p *paiVideoProvider) UploadURL(ctx context.Context, fileURL string) (map[string]any, error) {
	return p.requestForm(ctx, http.MethodPost, "/media/upload", map[string]string{"file_url": fileURL}, nil)
}

func (p *paiVideoProvider) UploadFile(ctx context.Context, kind, filename string, reader io.Reader) (map[string]any, error) {
	field := "image"
	path := "/image/upload"
	fields := map[string]string{"image_url": ""}
	if kind == "media" {
		field = "file"
		path = "/media/upload"
		fields = map[string]string{"file_url": ""}
	}
	return p.requestForm(ctx, http.MethodPost, path, fields, map[string]paiVideoFilePart{field: {Filename: filename, Reader: reader}})
}

func (p *paiVideoProvider) endpointForTask(taskType, inputMode string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(firstNonEmpty(taskType, inputMode)))
	switch value {
	case "video_text_to_video", "text", "text_to_video":
		return "/video/text/generate", nil
	case "video_image_to_video", "image", "image_to_video":
		return "/video/img/generate", nil
	case "video_template", "template", "template_video":
		return "/video/img/generate", nil
	case "video_fusion", "reference", "fusion":
		return "/video/fusion/generate", nil
	case "video_transition", "frames", "transition":
		return "/video/transition/generate", nil
	case "video_extend", "extend", "video_extension":
		return "/video/extend/generate", nil
	case "video_mimic", "motion", "mimic":
		return "/video/mimic/generate", nil
	case "video_sound_effect", "sound", "sound_effect":
		return "/video/sound_effect/generate", nil
	case "video_lip_sync", "lip-sync", "lip_sync":
		return "/video/lip_sync/generate", nil
	case "video_restyle", "restyle", "redraw":
		return "/video/restyle/generate", nil
	case "video_swap", "swap", "subject_swap":
		return "/video/swap/generate", nil
	case "video_multi_transition", "multi_transition", "multi-transition":
		return "/video/multi_transition/generate", nil
	case "video_modify", "modify", "video_edit":
		return "/video/modify/generate", nil
	default:
		return "", newNonRetryableProviderError(fmt.Sprintf("unsupported pai_video task_type: %s", taskType))
	}
}

func (p *paiVideoProvider) endpointForProviderAction(action string) (string, string, error) {
	value := strings.ToLower(strings.TrimSpace(action))
	switch value {
	case "restyle_list", "video_restyle_list":
		return "/video/restyle/list", http.MethodGet, nil
	case "swap_mask_selection", "video_swap_mask_selection", "mask_selection":
		return "/video/mask/selection", http.MethodPost, nil
	default:
		return "", "", newNonRetryableProviderError(fmt.Sprintf("unsupported pai_video provider action: %s", action))
	}
}

func (p *paiVideoProvider) requestJSON(ctx context.Context, method, path string, payload map[string]any) (map[string]any, error) {
	var body io.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(p.cfg.BaseURL, "/")+path, body)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("pai_video request init failed: %v", err))
	}
	p.setHeaders(req, ctx)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return p.do(req)
}

type paiVideoFilePart struct {
	Filename string
	Reader   io.Reader
}

func (p *paiVideoProvider) requestForm(ctx context.Context, method, path string, fields map[string]string, files map[string]paiVideoFilePart) (map[string]any, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	for key, file := range files {
		part, err := writer.CreateFormFile(key, filepath.Base(file.Filename))
		if err != nil {
			return nil, newRetryableProviderError(fmt.Sprintf("pai_video form init failed: %v", err))
		}
		if _, err := io.Copy(part, file.Reader); err != nil {
			return nil, newRetryableProviderError(fmt.Sprintf("pai_video form copy failed: %v", err))
		}
	}
	_ = writer.Close()
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(p.cfg.BaseURL, "/")+path, &body)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("pai_video request init failed: %v", err))
	}
	p.setHeaders(req, ctx)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return p.do(req)
}

func (p *paiVideoProvider) setHeaders(req *http.Request, ctx context.Context) {
	req.Header.Set("API-KEY", strings.TrimSpace(p.cfg.APIKey))
	req.Header.Set("Ai-trace-id", paiVideoTraceID(ctx))
	req.Header.Set("Accept", "application/json")
}

func paiVideoTraceID(ctx context.Context) string {
	parts := []string{"platform-runtime"}
	if requestID, ok := ctx.Value("request_id").(string); ok && strings.TrimSpace(requestID) != "" {
		parts = append(parts, "req", sanitizeTracePart(requestID))
	}
	if traceID, ok := ctx.Value("trace_id").(string); ok && strings.TrimSpace(traceID) != "" {
		parts = append(parts, "trace", sanitizeTracePart(traceID))
	}
	parts = append(parts, strconv.FormatInt(time.Now().UnixNano(), 10))
	return strings.Join(parts, "-")
}

func sanitizeTracePart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, value)
	if len(value) > 96 {
		value = value[:96]
	}
	return value
}

func (p *paiVideoProvider) do(req *http.Request) (map[string]any, error) {
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return nil, newNonRetryableProviderError("pai_video api key is not configured")
	}
	if strings.TrimSpace(p.cfg.BaseURL) == "" {
		return nil, newNonRetryableProviderError("pai_video base url is not configured")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("pai_video request failed: %v", err))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, newRetryableProviderError(fmt.Sprintf("decode pai_video response: %v", err))
	}
	if _, ok := payload["ErrCode"]; !ok {
		return nil, &paiVideoAPIError{message: "pai_video response envelope missing ErrCode", code: "PAI_VIDEO_ENVELOPE_INVALID", retryable: false, response: payload}
	}
	if resp.StatusCode >= 400 || !providerCodeOK(payload["ErrCode"]) {
		message := stringFromAny(payload["ErrMsg"])
		if message == "" {
			message = fmt.Sprintf("pai_video request failed: status=%d", resp.StatusCode)
		}
		code := normalizePaiVideoErrorCode(payload["ErrCode"])
		return nil, &paiVideoAPIError{message: message, errCode: payload["ErrCode"], response: payload, code: code, retryable: paiVideoErrorRetryable(payload["ErrCode"], resp.StatusCode)}
	}
	return normalizePaiVideoPayload(payload), nil
}

func normalizePaiVideoPayload(payload map[string]any) map[string]any {
	resp, _ := payload["Resp"].(map[string]any)
	out := map[string]any{"raw": payload}
	for key, value := range resp {
		out[key] = value
	}
	if id := firstAny(resp["video_id"], resp["videoId"], resp["id"]); id != nil {
		out["video_id"] = id
	}
	if credits := firstAny(resp["credits"], resp["credit"], resp["consume_credit"]); credits != nil {
		out["credits"] = credits
	}
	return out
}

func endpointSupportsPromptDefault(endpoint string) bool {
	switch endpoint {
	case "/video/text/generate", "/video/img/generate", "/video/fusion/generate", "/video/transition/generate", "/video/extend/generate", "/video/modify/generate":
		return true
	default:
		return false
	}
}

func endpointSupportsDefaultModel(endpoint string) bool {
	switch endpoint {
	case "/video/text/generate", "/video/img/generate", "/video/fusion/generate", "/video/transition/generate", "/video/extend/generate", "/video/multi_transition/generate":
		return true
	default:
		return false
	}
}

func translateNormalizedPaiVideoPayload(endpoint string, out map[string]any) {
	if options, ok := out["options"].(map[string]any); ok {
		for key, value := range options {
			if _, exists := out[key]; !exists {
				out[key] = value
			}
		}
		delete(out, "options")
	}
	assets, _ := out["source_assets"].([]any)
	for _, raw := range assets {
		asset, _ := raw.(map[string]any)
		role := strings.ToLower(strings.TrimSpace(stringFromAny(asset["role"])))
		assetID := stringFromAny(firstAny(asset["asset_id"], asset["id"]))
		if assetID == "" {
			continue
		}
		switch role {
		case "image", "source_image":
			if endpoint == "/video/img/generate" {
				out["img_id"] = assetID
			}
		case "reference_image":
			refs, _ := out["image_references"].([]any)
			refs = append(refs, map[string]any{"img_id": assetID})
			out["image_references"] = refs
		case "first_frame":
			out["first_frame_img"] = assetID
		case "last_frame":
			out["last_frame_img"] = assetID
		case "character_image":
			out["img_id"] = assetID
		case "motion_video":
			out["video_media_id"] = assetID
		case "video":
			out["video_id"] = assetID
		}
	}
	delete(out, "source_assets")
}

func cleanPaiVideoPayload(endpoint string, out map[string]any) {
	delete(out, "batch_count")
	switch endpoint {
	case "/video/text/generate", "/video/img/generate":
		// Official text/image generation supports optional sound_effect_* and lip_sync_tts_* fields.
		return
	case "/video/sound_effect/generate":
		delete(out, "lip_sync_tts_switch")
		delete(out, "lip_sync_tts_content")
		delete(out, "lip_sync_tts_speaker_id")
		return
	case "/video/lip_sync/generate":
		delete(out, "sound_effect_switch")
		delete(out, "sound_effect_content")
		delete(out, "original_sound_switch")
		return
	}
	delete(out, "sound_effect_switch")
	delete(out, "sound_effect_content")
	delete(out, "original_sound_switch")
	delete(out, "lip_sync_tts_switch")
	delete(out, "lip_sync_tts_content")
	delete(out, "lip_sync_tts_speaker_id")
}

func providerCodeOK(value any) bool {
	switch v := value.(type) {
	case float64:
		return int(v) == 0
	case int:
		return v == 0
	case string:
		return strings.TrimSpace(v) == "0"
	default:
		return false
	}
}

func normalizePaiVideoErrorCode(value any) string {
	code := stringFromAny(value)
	if code == "" {
		return "PAI_VIDEO_UPSTREAM_ERROR"
	}
	return "PAI_VIDEO_" + strings.ToUpper(strings.ReplaceAll(code, " ", "_"))
}

func paiVideoErrorRetryable(value any, statusCode int) bool {
	if statusCode >= 500 {
		return true
	}
	n := intFromAny(value)
	if n >= 500000 {
		return true
	}
	code := strings.ToLower(stringFromAny(value))
	return strings.Contains(code, "timeout") || strings.Contains(code, "rate") || strings.HasPrefix(code, "5")
}

func firstAny(values ...any) any {
	for _, value := range values {
		if value != nil && value != "" {
			return value
		}
	}
	return nil
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case float32:
		return strings.TrimSpace(strconv.FormatFloat(float64(v), 'f', -1, 32))
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var out int
		_, _ = fmt.Sscanf(v, "%d", &out)
		return out
	default:
		return 0
	}
}

func progressForPaiVideoStatus(status int) int {
	switch status {
	case 0:
		return 10
	case 5:
		return 45
	default:
		return 20
	}
}
