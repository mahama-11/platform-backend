package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-service/internal/config"
)

func TestGeminiImageSubmitTextToImageExtractsMarkdownDataURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body geminiChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != "gemini-3.1-flash-image-preview-token" {
			t.Fatalf("unexpected model: %s", body.Model)
		}
		if len(body.Messages) != 1 || len(body.Messages[0].Content) != 1 || body.Messages[0].Content[0].Text == "" {
			t.Fatalf("expected text-only chat content, got %+v", body.Messages)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gemini-3.1-flash-image-preview-token",
			"choices": []map[string]any{{
				"message": map[string]any{"content": "![image](data:image/jpeg;base64,aGVsbG8td29ybGQ=)"},
			}},
		})
	}))
	defer server.Close()

	provider := newGeminiImageProvider("gemini_image_generation", config.OpenAICompatibleVisionConfig{
		Enabled: true,
		BaseURL: server.URL,
		APIKey:  "dummy",
		Model:   "gemini-3.1-flash-image-preview-token",
	}).(*geminiImageProvider)
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-1",
		TaskType:     RuntimeTaskImageGeneration,
		Input: RuntimeInputManifest{
			InputMode:         "text_to_image",
			RequestedVariants: 1,
			ParamsSnapshot:    map[string]any{"prompt": "生成一张蓝色圆形商品图"},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submission == nil || submission.Completion == nil || len(submission.Completion.Variants) != 1 {
		t.Fatalf("unexpected submission: %+v", submission)
	}
	variant := submission.Completion.Variants[0]
	if variant.MimeType != "image/jpeg" || variant.InlineData != "data:image/jpeg;base64,aGVsbG8td29ybGQ=" {
		t.Fatalf("unexpected variant: %+v", variant)
	}
}

func TestGeminiImageSubmitImageToImageSendsSourceImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body geminiChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Messages) != 1 || len(body.Messages[0].Content) != 2 || body.Messages[0].Content[1].ImageURL == nil {
			t.Fatalf("expected text + image_url content, got %+v", body.Messages)
		}
		if body.Messages[0].Content[1].ImageURL.URL != "data:image/png;base64,aGVsbG8td29ybGQ=" {
			t.Fatalf("unexpected image url: %s", body.Messages[0].Content[1].ImageURL.URL)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gemini-3-pro-image-preview",
			"choices": []map[string]any{{
				"message": map[string]any{"content": "result: data:image/png;base64,aGVsbG8td29ybGQ="},
			}},
		})
	}))
	defer server.Close()

	provider := newGeminiImageProvider("gemini_image_generation", config.OpenAICompatibleVisionConfig{
		Enabled: true,
		BaseURL: server.URL,
		APIKey:  "dummy",
		Model:   "gemini-3-pro-image-preview",
	}).(*geminiImageProvider)
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-2",
		TaskType:     RuntimeTaskImageGeneration,
		Input: RuntimeInputManifest{
			InputMode:      "image_to_image",
			ParamsSnapshot: map[string]any{"prompt": "把红色改成绿色"},
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
	if submission.Completion.Variants[0].MimeType != "image/png" {
		t.Fatalf("expected png result, got %+v", submission.Completion.Variants[0])
	}
}
