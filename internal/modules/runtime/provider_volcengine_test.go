package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-service/internal/config"
)

func TestVolcengineSubmitTextToImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["model"] != "model-x" {
			t.Fatalf("unexpected model: %#v", body["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "model-x",
			"data": []map[string]any{
				{"url": "https://example.com/image.png"},
			},
		})
	}))
	defer server.Close()

	provider := newVolcengineImageProvider("volcengine", config.VolcengineConfig{
		BaseURL:    server.URL,
		APIKey:     "secret",
		ImageModel: "model-x",
		ImageSize:  "2K",
	}).(*volcengineImageProvider)
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-1",
		Input: RuntimeInputManifest{
			InputMode:         "text_to_image",
			RequestedVariants: 1,
			ParamsSnapshot:    map[string]any{"prompt": "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submission == nil || submission.Completion == nil || len(submission.Completion.Variants) != 1 {
		t.Fatalf("unexpected submission: %+v", submission)
	}
}

func TestVolcengineSubmitImageToImageAndHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "model-x",
			"data": []map[string]any{
				{"url": "https://example.com/image.webp"},
			},
		})
	}))
	defer server.Close()

	provider := newVolcengineImageProvider("volcengine", config.VolcengineConfig{
		BaseURL:    server.URL,
		APIKey:     "secret",
		ImageModel: "model-x",
		ImageSize:  "2K",
	}).(*volcengineImageProvider)
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-2",
		Input: RuntimeInputManifest{
			InputMode:         "image_to_image",
			RequestedVariants: 1,
			ParamsSnapshot: map[string]any{
				"prompt":        "hello",
				"output_format": "png",
			},
			SourceAssets: []ProviderSourceAsset{
				{ID: "asset-1", SourceURL: "data:image/png;base64,aGVsbG8td29ybGQ="},
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit image_to_image: %v", err)
	}
	if submission.Completion.Variants[0].MimeType != "image/png" {
		t.Fatalf("expected png mime type, got %+v", submission.Completion.Variants[0])
	}
	if normalizeDataURL("data:image/png;base64,abcd") != "data:image/png;base64,abcd" {
		t.Fatalf("expected normalizeDataURL passthrough")
	}
	if minInt(2, 5) != 2 {
		t.Fatalf("expected minInt result")
	}
	if classifyVolcengineError("timeout", 503) == nil {
		t.Fatalf("expected classified error")
	}
	if _, err := normalizeVolcengineOutputFormat("webp"); err == nil {
		t.Fatalf("expected unsupported output format error")
	}
}
