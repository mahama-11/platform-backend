package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"platform-service/internal/config"
)

func TestMinimaxTextProviderRequiresAPIKey(t *testing.T) {
	provider := newMinimaxTextProvider("minimax_text", config.MinimaxConfig{})
	_, err := provider.Submit(context.Background(), ProviderJobRequest{RuntimeJobID: "runtime-1", TaskType: RuntimeTaskIntentPlanning, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "plan intent"}}})
	if err == nil || !strings.Contains(err.Error(), "api key is not configured") || isRetryableProviderError(err) {
		t.Fatalf("expected non-retryable missing key error, got %v", err)
	}
}

func TestMinimaxTextProviderReturnsNormalizedJSONVariant(t *testing.T) {
	var observedAuth, observedPath string
	var observed map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedAuth = r.Header.Get("Authorization")
		observedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&observed); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"MiniMax-Test","choices":[{"message":{"role":"assistant","content":"{\"intent\":\"hero\",\"decision\":\"keep\"}"}}]}`))
	}))
	defer server.Close()

	provider := newMinimaxTextProvider("minimax_text", config.MinimaxConfig{BaseURL: server.URL + "/v1", APIKey: "test-key", Model: "MiniMax-Test", RequestTimeout: time.Second})
	result, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "runtime-1",
		TaskType:     RuntimeTaskIntentPlanning,
		Input: RuntimeInputManifest{
			PromptSnapshot: RuntimePromptSnapshot{SystemPrompt: "You plan ecommerce visual intent", UserPrompt: "plan intent"},
			ParamsSnapshot: map[string]any{"response_format": "json"},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if observedPath != "/v1/chat/completions" {
		t.Fatalf("unexpected endpoint path: %s", observedPath)
	}
	if observedAuth != "Bearer test-key" {
		t.Fatalf("missing auth header")
	}
	if observed["model"] != "MiniMax-Test" {
		t.Fatalf("unexpected model: %#v", observed["model"])
	}
	if rf, ok := observed["response_format"].(map[string]any); !ok || rf["type"] != "json_object" {
		t.Fatalf("expected json response_format, got %#v", observed["response_format"])
	}
	if result.Completion == nil || len(result.Completion.Variants) != 1 {
		t.Fatalf("expected one completion variant: %+v", result)
	}
	variant := result.Completion.Variants[0]
	if variant.AssetType != "json" || variant.MimeType != "application/json" || !strings.Contains(variant.InlineData, "hero") {
		t.Fatalf("unexpected variant: %+v", variant)
	}
}
