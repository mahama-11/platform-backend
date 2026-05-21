package runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-service/internal/config"
)

func TestComfyUIBridgeSubmitTextToImage(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/generate/text" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"task_id": "task-123",
			"status":  "pending",
		})
	}))
	defer server.Close()

	provider := newComfyUIBridgeProvider("comfyui_bridge", config.ComfyUIBridgeConfig{
		BaseURL:             server.URL,
		DefaultOutputFormat: "png",
	})
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-1",
		Input: RuntimeInputManifest{
			InputMode: "text_to_image",
			ParamsSnapshot: map[string]any{
				"prompt": "sunset over the sea",
				"steps":  8,
			},
		},
		CallbackURL: "http://callback.local/cb",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submission.ProviderJobID != "task-123" {
		t.Fatalf("unexpected provider job id: %+v", submission)
	}
	if captured["callback_url"] != "http://callback.local/cb" {
		t.Fatalf("expected callback_url to be forwarded, got %#v", captured["callback_url"])
	}
}

func TestComfyUIBridgeSubmitImageToImageUsesImageEndpoint(t *testing.T) {
	sampleBase64 := base64.StdEncoding.EncodeToString([]byte("this-is-a-long-enough-image-payload-for-validation"))
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/generate/image" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"task_id": "task-img",
			"status":  "pending",
		})
	}))
	defer server.Close()

	provider := newComfyUIBridgeProvider("comfyui_bridge", config.ComfyUIBridgeConfig{
		BaseURL:             server.URL,
		DefaultOutputFormat: "png",
	})
	_, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-2",
		Input: RuntimeInputManifest{
			InputMode: "image_to_image",
			ParamsSnapshot: map[string]any{
				"prompt":  "make it cinematic",
				"denoise": 0.7,
			},
			SourceAssets: []ProviderSourceAsset{
				{
					ID:        "asset-1",
					MimeType:  "image/png",
					SourceURL: "data:image/png;base64," + sampleBase64,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("submit image_to_image: %v", err)
	}
	if captured["image"] != sampleBase64 {
		t.Fatalf("expected raw base64 image payload, got %#v", captured["image"])
	}
}

func TestComfyUIBridgePollCompletedBuildsInlineData(t *testing.T) {
	sampleBase64 := base64.StdEncoding.EncodeToString([]byte("this-is-a-long-enough-result-image-payload-for-validation"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/task-done" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"task_id":       "task-done",
			"status":        "completed",
			"result_images": []string{sampleBase64},
			"progress":      100,
		})
	}))
	defer server.Close()

	provider := newComfyUIBridgeProvider("comfyui_bridge", config.ComfyUIBridgeConfig{
		BaseURL:             server.URL,
		DefaultOutputFormat: "png",
	})
	result, err := provider.Poll(context.Background(), "task-done")
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Status != "completed" || result.Completion == nil || len(result.Completion.Variants) != 1 {
		t.Fatalf("unexpected poll result: %+v", result)
	}
	if result.Completion.Variants[0].InlineData != "data:image/png;base64,"+sampleBase64 {
		t.Fatalf("unexpected inline data: %+v", result.Completion.Variants[0])
	}
}

func TestComfyUIBridgeSubmitSanitizesPythonTraceback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "name 'traceback' is not defined", http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := newComfyUIBridgeProvider("comfyui_bridge", config.ComfyUIBridgeConfig{BaseURL: server.URL})
	_, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-understand",
		TaskType:     RuntimeTaskImageUnderstanding,
		Input: RuntimeInputManifest{SourceAssets: []ProviderSourceAsset{{
			ID:        "asset-1",
			MimeType:  "image/png",
			SourceURL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=",
		}}},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if strings.Contains(err.Error(), "traceback") {
		t.Fatalf("raw traceback leaked: %v", err)
	}
	if !strings.Contains(err.Error(), "image understanding provider failed internally") {
		t.Fatalf("expected sanitized provider message, got %v", err)
	}
}

func TestComfyUIBridgeRejectsUnreadableUnderstandingImageBeforeProvider(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newComfyUIBridgeProvider("comfyui_bridge", config.ComfyUIBridgeConfig{BaseURL: server.URL})
	_, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-invalid-image",
		TaskType:     RuntimeTaskImageUnderstanding,
		Input: RuntimeInputManifest{SourceAssets: []ProviderSourceAsset{{
			ID:        "asset-1",
			MimeType:  "image/jpeg",
			SourceURL: "data:image/jpeg;base64,dGVzdA==",
		}}},
	})
	if err == nil {
		t.Fatal("expected invalid image error")
	}
	if called {
		t.Fatal("provider should not be called for an unreadable source image")
	}
	if !strings.Contains(err.Error(), "source image file is invalid or unreadable") && !strings.Contains(err.Error(), "source JPEG image file is invalid or unreadable") {
		t.Fatalf("unexpected invalid image error: %v", err)
	}
}
