package runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"platform-service/internal/config"
)

func TestMinimaxTextProviderErrorAndPlainTextBranches(t *testing.T) {
	plainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"MiniMax-Plain","choices":[{"message":{"role":"assistant","content":"plain answer"}}]}`))
	}))
	defer plainServer.Close()

	provider := newMinimaxTextProvider("minimax_text", config.MinimaxConfig{BaseURL: plainServer.URL, APIKey: "test-key", RequestTimeout: time.Second})
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "runtime-plain",
		TaskType:     RuntimeTaskTextReasoning,
		Input:        RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{StylePrompt: "short", UserPrompt: "answer"}},
	})
	if err != nil {
		t.Fatalf("Submit plain text: %v", err)
	}
	if got := submission.Completion.Variants[0]; got.AssetType != "text" || got.MimeType != "text/plain" || got.InlineData != "plain answer" {
		t.Fatalf("unexpected plain variant: %+v", got)
	}
	if _, err := provider.Poll(context.Background(), "provider-job"); err == nil || isRetryableProviderError(err) {
		t.Fatalf("expected non-retryable poll unsupported error, got %v", err)
	}
	if err := provider.Cancel(context.Background(), "provider-job"); err != nil {
		t.Fatalf("Cancel should be noop: %v", err)
	}

	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"quota temporarily exceeded"}}`))
	}))
	defer errorServer.Close()
	provider = newMinimaxTextProvider("minimax_text", config.MinimaxConfig{BaseURL: errorServer.URL, APIKey: "test-key"})
	_, err = provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "runtime-error", TaskType: RuntimeTaskTextReasoning, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "answer"}}})
	if err == nil || !isRetryableProviderError(err) || !strings.Contains(err.Error(), "quota") {
		t.Fatalf("expected retryable minimax error, got %v", err)
	}

	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model":"MiniMax-Empty","choices":[{"message":{"role":"assistant","content":"   "}}]}`))
	}))
	defer emptyServer.Close()
	provider = newMinimaxTextProvider("minimax_text", config.MinimaxConfig{BaseURL: emptyServer.URL, APIKey: "test-key"})
	_, err = provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "runtime-empty", TaskType: RuntimeTaskTextReasoning, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "answer"}}})
	if err == nil || !isRetryableProviderError(err) || !strings.Contains(err.Error(), "empty content") {
		t.Fatalf("expected retryable empty content error, got %v", err)
	}
}

func TestMinimaxImageProviderSuccessAndBranchHelpers(t *testing.T) {
	var observed minimaxImageGenerationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/image_generation" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&observed); err != nil {
			t.Fatalf("decode minimax image request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"img-resp-1","data":{"image_base64":["aW1hZ2UtMQ==",""]},"base_resp":{"status_code":0}}`))
	}))
	defer server.Close()

	provider := newMinimaxImageProvider("minimax_image", config.MinimaxImageConfig{BaseURL: server.URL, APIKey: "test-key", Model: "image-test", RequestTimeout: time.Second})
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "runtime-image",
		TaskType:     RuntimeTaskImageGeneration,
		Input: RuntimeInputManifest{
			InputMode:      "image_to_image",
			PromptSnapshot: RuntimePromptSnapshot{SystemPrompt: "system", UserPrompt: "make image"},
			ParamsSnapshot: map[string]any{"width": 1920, "height": 1080, "negative_prompt": "blur", "negative_prompt_text": "blur", "subject_type": "product"},
			SourceAssets:   []ProviderSourceAsset{{ID: "asset-1", SourceURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("source-image"))}},
		},
	})
	if err != nil {
		t.Fatalf("Submit minimax image: %v", err)
	}
	if observed.AspectRatio != "16:9" || len(observed.SubjectReference) != 1 || observed.SubjectReference[0].Type != "product" {
		t.Fatalf("unexpected minimax image request: %+v", observed)
	}
	if len(submission.Completion.Variants) != 1 || !strings.HasPrefix(submission.Completion.Variants[0].InlineData, "data:image/") {
		t.Fatalf("unexpected minimax image completion: %+v", submission.Completion)
	}
	if _, err := provider.Poll(context.Background(), "provider-job"); err == nil || isRetryableProviderError(err) {
		t.Fatalf("expected non-retryable poll unsupported error, got %v", err)
	}
	if err := provider.Cancel(context.Background(), "provider-job"); err != nil {
		t.Fatalf("Cancel should be noop: %v", err)
	}
	if normalizeMinimaxAspectRatio("bad") != "1:1" || minimaxAspectRatioFromDimensions(map[string]any{"width": json.Number("300"), "height": json.Number("200")}) != "3:2" || nearestMinimaxAspectRatio(1000, 300) != "21:9" {
		t.Fatalf("unexpected aspect ratio helper behavior")
	}
	if got := intGCD(-12, 8); got != 4 {
		t.Fatalf("unexpected gcd: %d", got)
	}
}

func TestMinimaxImageProviderErrorBranches(t *testing.T) {
	provider := newMinimaxImageProvider("minimax_image", config.MinimaxImageConfig{})
	_, err := provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "bad-task", TaskType: RuntimeTaskTextReasoning, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "x"}}})
	if err == nil || isRetryableProviderError(err) || !strings.Contains(err.Error(), "only supports") {
		t.Fatalf("expected unsupported task error, got %v", err)
	}
	provider = newMinimaxImageProvider("minimax_image", config.MinimaxImageConfig{APIKey: "test-key"})
	_, err = provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "missing-ref", TaskType: RuntimeTaskImageInpainting, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "edit"}, SourceAssets: []ProviderSourceAsset{{ID: "asset-1", SourceURL: "not-a-url"}}}})
	if err == nil || isRetryableProviderError(err) || !strings.Contains(err.Error(), "no usable image reference") {
		t.Fatalf("expected no usable image reference error, got %v", err)
	}

	businessServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":1002,"status_msg":"bad request"}}`))
	}))
	defer businessServer.Close()
	provider = newMinimaxImageProvider("minimax_image", config.MinimaxImageConfig{BaseURL: businessServer.URL, APIKey: "test-key"})
	_, err = provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "biz", TaskType: RuntimeTaskImageGeneration, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "draw"}}})
	if err == nil || isRetryableProviderError(err) {
		t.Fatalf("expected non-retryable minimax business error, got %v", err)
	}
}

func TestGeminiVisualProviderSuccessAndErrors(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("image-bytes"))
	}))
	defer imageServer.Close()

	var observed geminiChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&observed); err != nil {
			t.Fatalf("decode gemini request: %v", err)
		}
		_, _ = w.Write([]byte(`{"model":"gemini-test","choices":[{"message":{"content":"{\"summary\":\"ok\",\"confidence\":0.9,\"elements\":[]}"}}]}`))
	}))
	defer server.Close()

	provider := newGeminiVisualProvider("gemini_visual", config.OpenAICompatibleVisionConfig{BaseURL: server.URL, APIKey: "test-key", Model: "gemini-test", RequestTimeout: time.Second})
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "visual-1", TaskType: RuntimeTaskImageUnderstanding, Input: RuntimeInputManifest{SourceAssets: []ProviderSourceAsset{{ID: "asset-1", SourceURL: imageServer.URL}}}})
	if err != nil {
		t.Fatalf("Submit gemini visual: %v", err)
	}
	if observed.Model != "gemini-test" || len(observed.Messages) != 1 || len(observed.Messages[0].Content) != 2 {
		t.Fatalf("unexpected gemini request: %+v", observed)
	}
	if got := submission.Completion.Variants[0]; got.AssetType != "json" || got.MimeType != "application/json" || !strings.Contains(got.InlineData, "summary") {
		t.Fatalf("unexpected gemini variant: %+v", got)
	}
	if _, err := provider.Poll(context.Background(), "provider-job"); err == nil || isRetryableProviderError(err) {
		t.Fatalf("expected non-retryable poll unsupported error, got %v", err)
	}
	if err := provider.Cancel(context.Background(), "provider-job"); err != nil {
		t.Fatalf("Cancel should be noop: %v", err)
	}

	provider = newGeminiVisualProvider("gemini_visual", config.OpenAICompatibleVisionConfig{APIKey: "test-key"})
	_, err = provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "visual-bad-task", TaskType: RuntimeTaskImageGeneration})
	if err == nil || isRetryableProviderError(err) || !strings.Contains(err.Error(), "only supports") {
		t.Fatalf("expected unsupported task error, got %v", err)
	}
	_, err = provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "visual-no-image", TaskType: RuntimeTaskImageUnderstanding})
	if err == nil || isRetryableProviderError(err) || !strings.Contains(err.Error(), "source image is required") {
		t.Fatalf("expected missing source error, got %v", err)
	}
	if !isRetryableProviderError(classifyGeminiVisualError("temporarily down", http.StatusInternalServerError)) || isRetryableProviderError(classifyGeminiVisualError("bad prompt", http.StatusBadRequest)) {
		t.Fatalf("unexpected gemini error classification")
	}
}

func TestVolcengineProviderSubmitAndErrorBranches(t *testing.T) {
	var observed volcengineGenerateImagesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&observed); err != nil {
			t.Fatalf("decode volcengine request: %v", err)
		}
		url := "https://cdn.example.com/result.jpeg"
		_, _ = w.Write([]byte(`{"model":"volc-test","data":[{"url":"` + url + `"},{"url":"https://cdn.example.com/ignored.jpeg"}]}`))
	}))
	defer server.Close()
	provider := newVolcengineImageProvider("volcengine", config.VolcengineConfig{BaseURL: server.URL, APIKey: "test-key", ImageModel: "volc-test", ImageSize: "1K", Watermark: true})
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "volc-1", TaskType: RuntimeTaskImageGeneration, Input: RuntimeInputManifest{RequestedVariants: 1, PromptSnapshot: RuntimePromptSnapshot{Provider: "volcengine", Model: "snapshot-model", UserPrompt: "draw"}, ParamsSnapshot: map[string]any{"output_format": "jpg"}}})
	if err != nil {
		t.Fatalf("Submit volcengine: %v", err)
	}
	if observed.Model != "snapshot-model" || observed.OutputFormat != "jpeg" || !observed.Watermark {
		t.Fatalf("unexpected volcengine request: %+v", observed)
	}
	if len(submission.Completion.Variants) != 1 || submission.Completion.Variants[0].MimeType != "image/jpeg" {
		t.Fatalf("unexpected volcengine variants: %+v", submission.Completion.Variants)
	}
	if _, err := provider.Poll(context.Background(), "provider-job"); err == nil || isRetryableProviderError(err) {
		t.Fatalf("expected non-retryable poll unsupported error, got %v", err)
	}
	if err := provider.Cancel(context.Background(), "provider-job"); err != nil {
		t.Fatalf("Cancel should be noop: %v", err)
	}

	provider = newVolcengineImageProvider("volcengine", config.VolcengineConfig{APIKey: "test-key"})
	_, err = provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "bad-format", Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "draw"}, ParamsSnapshot: map[string]any{"output_format": "gif"}}})
	if err == nil || isRetryableProviderError(err) || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported output format, got %v", err)
	}
	if !isRetryableProviderError(classifyVolcengineError("server down", http.StatusInternalServerError)) || isRetryableProviderError(classifyVolcengineError("parameter `x` bad", http.StatusBadRequest)) {
		t.Fatalf("unexpected volcengine error classification")
	}
}

func TestAdditionalProviderNoopAndClassificationHelpers(t *testing.T) {
	gemini := newGeminiImageProvider("gemini_image_generation", config.OpenAICompatibleVisionConfig{}).(*geminiImageProvider)
	if gemini.Name() != "gemini_image_generation" {
		t.Fatalf("unexpected gemini name")
	}
	if _, err := gemini.Poll(context.Background(), "job"); err == nil {
		t.Fatalf("expected gemini image poll unsupported")
	}
	if err := gemini.Cancel(context.Background(), "job"); err != nil {
		t.Fatalf("gemini image cancel should noop: %v", err)
	}
	if got := extractGeminiImageDataURL("prefix data:image/png;base64,QUJD suffix"); got == "" || mimeTypeFromDataURL(got) != "image/png" {
		t.Fatalf("unexpected gemini data url extraction: %s", got)
	}
	if mimeTypeFromDataURL("data:image/webp;base64,QUJD") != "image/webp" || mimeTypeFromDataURL("data:image/jpeg;base64,QUJD") != "image/jpeg" {
		t.Fatalf("unexpected data url mime type")
	}
	kimi := newKimiCodingTextProvider("kimi_coding_text", config.KimiCodingConfig{}).(*kimiCodingTextProvider)
	if kimi.Name() != "kimi_coding_text" {
		t.Fatalf("unexpected kimi name")
	}
	if _, err := kimi.Poll(context.Background(), "job"); err == nil {
		t.Fatalf("expected kimi poll unsupported")
	}
	if err := kimi.Cancel(context.Background(), "job"); err != nil {
		t.Fatalf("kimi cancel should noop: %v", err)
	}
	for _, tc := range []struct {
		text   string
		status int
	}{
		{"rate limit", http.StatusTooManyRequests},
		{"unauthorized", http.StatusUnauthorized},
		{"bad request", http.StatusBadRequest},
		{"connection reset", http.StatusInternalServerError},
	} {
		if classifyKimiCodingError(tc.text, tc.status) == nil || classifyMinimaxError(tc.text, tc.status) == nil || classifyVolcengineError(tc.text, tc.status) == nil {
			t.Fatalf("expected provider classification for %+v", tc)
		}
	}
	longBody := []byte(strings.Repeat("x", 610))
	if len(trimBodyForError(longBody)) != len(strings.Repeat("x", 600)+"...(truncated)") || trimBodyForError([]byte(" short ")) != "short" {
		t.Fatalf("unexpected trimBodyForError")
	}
	if classifyComfyUIBridgePollError(404, []byte("missing")) == nil || classifyComfyUIBridgePollError(500, []byte("server")) == nil {
		t.Fatalf("expected comfy poll classification")
	}
}
