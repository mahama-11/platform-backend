package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-service/internal/config"
)

func TestComfyTransparentPNGBase64IsTransparent(t *testing.T) {
	payload, err := base64.StdEncoding.DecodeString(comfyTransparentPNGBase64)
	if err != nil {
		t.Fatalf("decode placeholder base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("decode placeholder PNG: %v", err)
	}
	if got := img.Bounds().Dx(); got != 1 {
		t.Fatalf("placeholder width = %d, want 1", got)
	}
	if got := img.Bounds().Dy(); got != 1 {
		t.Fatalf("placeholder height = %d, want 1", got)
	}
	_, _, _, alpha := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	if alpha != 0 {
		t.Fatalf("placeholder pixel alpha = %d, want 0", alpha)
	}
}

func TestComfyUIBridgeSubmitMultiImagePadsToExactlyFourImages(t *testing.T) {
	realImages := []string{
		base64.StdEncoding.EncodeToString([]byte("real-image-payload-00000000000000000001")),
		base64.StdEncoding.EncodeToString([]byte("real-image-payload-00000000000000000002")),
		base64.StdEncoding.EncodeToString([]byte("real-image-payload-00000000000000000003")),
		base64.StdEncoding.EncodeToString([]byte("real-image-payload-00000000000000000004")),
		base64.StdEncoding.EncodeToString([]byte("real-image-payload-00000000000000000005")),
	}

	tests := []struct {
		name       string
		realCount  int
		wantImages []string
	}{
		{
			name:       "one real image gets three placeholders",
			realCount:  1,
			wantImages: []string{realImages[0], comfyTransparentPNGBase64, comfyTransparentPNGBase64, comfyTransparentPNGBase64},
		},
		{
			name:       "two real images get two placeholders",
			realCount:  2,
			wantImages: []string{realImages[0], realImages[1], comfyTransparentPNGBase64, comfyTransparentPNGBase64},
		},
		{
			name:       "three real images get one placeholder",
			realCount:  3,
			wantImages: []string{realImages[0], realImages[1], realImages[2], comfyTransparentPNGBase64},
		},
		{
			name:       "four real images are unchanged",
			realCount:  4,
			wantImages: []string{realImages[0], realImages[1], realImages[2], realImages[3]},
		},
		{
			name:       "five real images use first four only",
			realCount:  5,
			wantImages: []string{realImages[0], realImages[1], realImages[2], realImages[3]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/generate/multi-image" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"task_id": "task-multi",
					"status":  "pending",
				})
			}))
			defer server.Close()

			provider := newComfyUIBridgeProvider("comfyui_bridge", config.ComfyUIBridgeConfig{
				BaseURL:             server.URL,
				DefaultOutputFormat: "png",
			})
			_, err := provider.Submit(context.Background(), ProviderJobRequest{
				RuntimeJobID: "job-multi",
				Input: RuntimeInputManifest{
					InputMode: "multi_image",
					ParamsSnapshot: map[string]any{
						"prompt": "compose these images",
					},
					SourceAssets: sourceAssetsForPayloads(realImages[:tt.realCount]),
				},
			})
			if err != nil {
				t.Fatalf("submit multi_image: %v", err)
			}

			images, ok := captured["images"].([]any)
			if !ok {
				t.Fatalf("expected images array in captured body, got %#v", captured["images"])
			}
			if len(images) != 4 {
				t.Fatalf("expected exactly 4 images, got %d: %#v", len(images), images)
			}
			for i, want := range tt.wantImages {
				if got := images[i]; got != want {
					t.Fatalf("image[%d] mismatch: got %#v want %#v", i, got, want)
				}
			}
		})
	}
}

func TestComfyUIBridgeSubmitMultiImageRejectsZeroUsableImages(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newComfyUIBridgeProvider("comfyui_bridge", config.ComfyUIBridgeConfig{BaseURL: server.URL})
	_, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-zero-usable",
		Input: RuntimeInputManifest{
			InputMode: "multi_image",
			ParamsSnapshot: map[string]any{
				"prompt": "compose these images",
			},
			SourceAssets: []ProviderSourceAsset{
				{ID: "empty"},
				{ID: "url", SourceURL: "https://example.com/not-inline.png"},
				{ID: "short-base64", SourceURL: "aGVsbG8="},
			},
		},
	})
	if err == nil {
		t.Fatal("expected zero usable images error")
	}
	if called {
		t.Fatal("provider should not be called when no usable images are present")
	}
	if !strings.Contains(err.Error(), "no usable image payload found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func sourceAssetsForPayloads(payloads []string) []ProviderSourceAsset {
	assets := make([]ProviderSourceAsset, 0, len(payloads))
	for _, payload := range payloads {
		assets = append(assets, ProviderSourceAsset{
			ID:        "asset",
			MimeType:  "image/png",
			SourceURL: payload,
		})
	}
	return assets
}
