package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-service/internal/config"
)

func TestGeminiVisualProviderUsesPromptSnapshotBeforeParamsFallback(t *testing.T) {
	var submittedPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body geminiChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Messages) != 1 || len(body.Messages[0].Content) < 2 {
			t.Fatalf("expected text and image content, got %+v", body.Messages)
		}
		submittedPrompt = body.Messages[0].Content[0].Text
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gemini-visual-test",
			"choices": []map[string]any{{
				"message": map[string]any{"content": `{"elements":[{"element_type":"product_fact","element_key":"product_info","label":"产品信息","value":{"description":"杯子"},"confidence":0.9,"readiness":"ready"}]}`},
			}},
		})
	}))
	defer server.Close()

	provider := newGeminiVisualProvider("gemini_visual_understanding", config.OpenAICompatibleVisionConfig{
		Enabled: true,
		BaseURL: server.URL,
		APIKey:  "dummy",
		Model:   "gemini-visual-test",
	}).(*geminiVisualProvider)

	_, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-visual-1",
		TaskType:     RuntimeTaskImageUnderstanding,
		Input: RuntimeInputManifest{
			PromptSnapshot: RuntimePromptSnapshot{UserPrompt: "shared fixed product/background prompt"},
			ParamsSnapshot: map[string]any{"understanding_prompt": "legacy params prompt"},
			SourceAssets: []ProviderSourceAsset{{
				ID:        "asset-1",
				SourceURL: "data:image/png;base64,aGVsbG8td29ybGQ=",
				MimeType:  "image/png",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submittedPrompt != "shared fixed product/background prompt" {
		t.Fatalf("expected prompt_snapshot.user_prompt to win, got %q", submittedPrompt)
	}
}

func TestGeminiVisualProviderFallsBackToParamsUnderstandingPrompt(t *testing.T) {
	var submittedPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body geminiChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		submittedPrompt = body.Messages[0].Content[0].Text
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gemini-visual-test",
			"choices": []map[string]any{{
				"message": map[string]any{"content": `{"elements":[{"element_type":"background","element_key":"background_info","label":"背景信息","value":{"description":"白底"},"confidence":0.9,"readiness":"ready"}]}`},
			}},
		})
	}))
	defer server.Close()

	provider := newGeminiVisualProvider("gemini_visual_understanding", config.OpenAICompatibleVisionConfig{Enabled: true, BaseURL: server.URL, APIKey: "dummy", Model: "gemini-visual-test"}).(*geminiVisualProvider)
	_, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-visual-2",
		TaskType:     RuntimeTaskImageUnderstanding,
		Input: RuntimeInputManifest{
			ParamsSnapshot: map[string]any{"understanding_prompt": "params understanding prompt"},
			SourceAssets:   []ProviderSourceAsset{{ID: "asset-1", SourceURL: "data:image/png;base64,aGVsbG8td29ybGQ=", MimeType: "image/png"}},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !strings.Contains(submittedPrompt, "params understanding prompt") {
		t.Fatalf("expected params understanding prompt fallback, got %q", submittedPrompt)
	}
}
