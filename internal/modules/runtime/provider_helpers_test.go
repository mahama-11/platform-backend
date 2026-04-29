package runtime

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-service/internal/config"
)

func TestProviderRegistryAndManualProvider(t *testing.T) {
	registry := NewProviderRegistry(config.VolcengineConfig{}, config.ComfyUIBridgeConfig{})
	if _, err := registry.Get("manual"); err != nil {
		t.Fatalf("expected manual provider to be registered: %v", err)
	}
	manual, err := registry.Get("mock")
	if err != nil {
		t.Fatalf("expected mock provider: %v", err)
	}
	submission, err := manual.Submit(context.Background(), ProviderJobRequest{})
	if err != nil || submission == nil || submission.ProviderJobID == "" {
		t.Fatalf("unexpected manual submission: %+v err=%v", submission, err)
	}
	if _, err := manual.Poll(context.Background(), "anything"); err != nil {
		t.Fatalf("manual Poll should not error: %v", err)
	}
	if _, err := registry.Get("missing"); err == nil {
		t.Fatalf("expected missing provider error")
	}
	if !isRetryableProviderError(newRetryableProviderError("retry")) {
		t.Fatalf("expected retryable provider error")
	}
	if isRetryableProviderError(newNonRetryableProviderError("no-retry")) {
		t.Fatalf("expected non-retryable provider error")
	}
	registry = NewProviderRegistry(config.VolcengineConfig{}, config.ComfyUIBridgeConfig{Enabled: true, BaseURL: "http://127.0.0.1:8000"})
	if _, err := registry.Get("comfyui_bridge"); err != nil {
		t.Fatalf("expected comfyui_bridge to be registered when enabled: %v", err)
	}
}

func TestVolcengineHelperFunctions(t *testing.T) {
	sampleBase64 := base64.StdEncoding.EncodeToString([]byte("this-is-a-long-enough-payload-for-base64-check"))
	if buildVolcenginePrompt(ProviderJobRequest{
		Input: RuntimeInputManifest{
			PromptSnapshot: RuntimePromptSnapshot{
				SystemPrompt:   "system prompt",
				StylePrompt:    "style prompt",
				UserPrompt:     "user prompt",
				PromptTemplate: "legacy prompt",
			},
			ParamsSnapshot: map[string]any{"prompt": "actual prompt"},
		},
	}) != "system prompt\n\nstyle prompt\n\nuser prompt\n\nactual prompt" {
		t.Fatalf("unexpected prompt build")
	}
	if _, err := extractDataURLPayload("invalid"); err == nil {
		t.Fatalf("expected invalid data url error")
	}
	if !isBase64Payload(sampleBase64) {
		t.Fatalf("expected valid base64 payload")
	}
	if normalizeImageMimeType("image/jpeg; charset=utf-8") != "image/jpeg" {
		t.Fatalf("expected normalized mime type")
	}
	if got, err := normalizeVolcengineOutputFormat("jpg"); err != nil || got != "jpeg" {
		t.Fatalf("unexpected normalized output format: %s err=%v", got, err)
	}
	if outputFormatToMimeType("", "https://x/test.webp") != "image/webp" {
		t.Fatalf("expected webp mime type")
	}
	if stringMapValue(map[string]any{"steps": 8}, "steps") != "8" {
		t.Fatalf("expected stringMapValue to stringify non-string")
	}
	if classifyProviderErrorClass(newRetryableProviderError("retry")) != "retryable_provider" {
		t.Fatalf("expected retryable_provider classification")
	}
}

func TestVolcengineFetchImageAsDataURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer server.Close()

	provider := newVolcengineImageProvider("volcengine", config.VolcengineConfig{}).(*volcengineImageProvider)
	dataURL, err := provider.fetchImageAsDataURL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetchImageAsDataURL: %v", err)
	}
	if dataURL == "" || dataURL[:14] != "data:image/png"[:14] {
		t.Fatalf("unexpected data url: %s", dataURL)
	}
}
