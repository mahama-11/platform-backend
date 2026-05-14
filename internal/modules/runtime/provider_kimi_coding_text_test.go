package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/config"
)

func TestKimiCodingTextProviderRequiresAPIKey(t *testing.T) {
	provider := newKimiCodingTextProvider("kimi_coding_text", config.KimiCodingConfig{})
	_, err := provider.Submit(t.Context(), ProviderJobRequest{RuntimeJobID: "rt_1", TaskType: RuntimeTaskIntentPlanning, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "hello"}}})
	if err == nil || isRetryableProviderError(err) {
		t.Fatalf("expected non-retryable missing key error, got %v", err)
	}
}

func TestKimiCodingTextProviderAnthropicMessagesSuccess(t *testing.T) {
	var gotAuth, gotVersion, gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/coding/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("anthropic-version")
		var payload kimiCodingMessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotModel = payload.Model
		if len(payload.Messages) != 1 || payload.Messages[0].Role != "user" || payload.Messages[0].Content == "" {
			t.Fatalf("unexpected messages: %+v", payload.Messages)
		}
		_ = json.NewEncoder(w).Encode(kimiCodingMessagesResponse{ID: "msg_1", Type: "message", Role: "assistant", Model: "kimi-for-coding", Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{{Type: "text", Text: `{"ok":true}`}}})
	}))
	defer server.Close()
	provider := newKimiCodingTextProvider("kimi_coding_text", config.KimiCodingConfig{BaseURL: server.URL + "/coding", APIKey: "test-key", Model: "kimi-k2.6", RequestTimeout: time.Second})
	out, err := provider.Submit(t.Context(), ProviderJobRequest{RuntimeJobID: "rt_1", TaskType: RuntimeTaskPromptPlanning, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{SystemPrompt: "system", UserPrompt: "user"}}})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if gotAuth != "Bearer test-key" || gotVersion == "" || gotModel != "kimi-k2.6" {
		t.Fatalf("unexpected provider request auth/version/model auth=%q version=%q model=%q", gotAuth, gotVersion, gotModel)
	}
	if out.Completion == nil || len(out.Completion.Variants) != 1 || out.Completion.Variants[0].MimeType != "application/json" {
		t.Fatalf("unexpected completion: %+v", out)
	}
}

func TestKimiCodingTextProviderRateLimitIsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(kimiCodingMessagesResponse{})
	}))
	defer server.Close()
	provider := newKimiCodingTextProvider("kimi_coding_text", config.KimiCodingConfig{BaseURL: server.URL, APIKey: "test-key", Model: "kimi-k2.6", RequestTimeout: time.Second})
	_, err := provider.Submit(t.Context(), ProviderJobRequest{RuntimeJobID: "rt_1", TaskType: RuntimeTaskTextReasoning, Input: RuntimeInputManifest{PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "hello"}}})
	if err == nil || !isRetryableProviderError(err) {
		t.Fatalf("expected retryable rate-limit error, got %v", err)
	}
}
