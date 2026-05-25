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

func TestMinimaxImageSubmitImageToImageSendsSubjectReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/image_generation" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dummy" {
			t.Fatalf("missing auth header: %s", got)
		}
		var body minimaxImageGenerationRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Model != "image-01" || body.ResponseFormat != "base64" || body.AspectRatio != "16:9" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		if !strings.Contains(body.Prompt, "no clutter") || !strings.Contains(body.Prompt, "Must avoid") {
			t.Fatalf("negative prompt should be applied as labeled minimax prompt text: %q", body.Prompt)
		}
		if len(body.SubjectReference) != 1 {
			t.Fatalf("expected subject reference: %+v", body.SubjectReference)
		}
		if body.SubjectReference[0].Type != "character" || body.SubjectReference[0].ImageFile != "https://example.com/source.jpg" {
			t.Fatalf("unexpected subject reference: %+v", body.SubjectReference[0])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":        "resp-1",
			"data":      map[string]any{"image_base64": []string{"aGVsbG8td29ybGQ="}},
			"base_resp": map[string]any{"status_code": 0, "status_msg": "success"},
		})
	}))
	defer server.Close()

	provider := newMinimaxImageProvider("minimax_image_generation", config.MinimaxImageConfig{
		Enabled:            true,
		BaseURL:            server.URL + "/v1",
		APIKey:             "dummy",
		Model:              "image-01",
		DefaultAspectRatio: "1:1",
	}).(*minimaxImageProvider)
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-1",
		TaskType:     RuntimeTaskImageGeneration,
		Input: RuntimeInputManifest{
			InputMode: "image_to_image",
			ParamsSnapshot: map[string]any{
				"prompt":          "Generate product photo",
				"width":           1280,
				"height":          720,
				"negative_prompt": "no clutter",
			},
			SourceAssets: []ProviderSourceAsset{{SourceURL: "https://example.com/source.jpg", MimeType: "image/jpeg"}},
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
	if variant.Metadata["requested_aspect_ratio"] != "16:9" || variant.Metadata["requested_width"] != 1280 || variant.Metadata["requested_height"] != 720 || variant.Metadata["negative_prompt_applied"] != true {
		t.Fatalf("expected requested dimensions and negative prompt metadata, got %#v", variant.Metadata)
	}
}

func TestMinimaxAspectRatioFromDimensionsMapsNearestSupportedRatio(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params map[string]any
		want   string
	}{
		{name: "exact wide", params: map[string]any{"width": 1280, "height": 720}, want: "16:9"},
		{name: "near portrait", params: map[string]any{"width": 768, "height": 1344}, want: "9:16"},
		{name: "near poster", params: map[string]any{"width": 900, "height": 1200}, want: "3:4"},
		{name: "numeric strings", params: map[string]any{"width": "1024", "height": "1024"}, want: "1:1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := minimaxAspectRatioFromDimensions(tc.params); got != tc.want {
				t.Fatalf("minimaxAspectRatioFromDimensions()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestMinimaxAspectRatioPrecedenceKeepsExplicitRatio(t *testing.T) {
	got := normalizeMinimaxAspectRatio(firstNonEmpty(
		stringMapValue(map[string]any{"aspect_ratio": "3:4", "width": 1280, "height": 720}, "aspect_ratio"),
		minimaxAspectRatioFromDimensions(map[string]any{"width": 1280, "height": 720}),
		"1:1",
	))
	if got != "3:4" {
		t.Fatalf("explicit aspect_ratio should win over dimensions, got %q", got)
	}
}

func TestMinimaxImageSubjectReferenceUsesOnlyFirstUsableAsset(t *testing.T) {
	refs, err := buildMinimaxSubjectReference([]ProviderSourceAsset{
		{SourceURL: "https://example.com/first.jpg"},
		{SourceURL: "https://example.com/second.jpg"},
	}, map[string]any{"minimax_subject_type": "character"})
	if err != nil {
		t.Fatalf("build refs: %v", err)
	}
	if len(refs) != 1 || refs[0].ImageFile != "https://example.com/first.jpg" {
		t.Fatalf("expected only first usable reference, got %+v", refs)
	}
}

func TestMinimaxImageSubmitTreatsBusinessErrorAsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":        "resp-fail",
			"base_resp": map[string]any{"status_code": 2013, "status_msg": "invalid params, subject_reference must be character"},
		})
	}))
	defer server.Close()

	provider := newMinimaxImageProvider("minimax_image_generation", config.MinimaxImageConfig{Enabled: true, BaseURL: server.URL + "/v1", APIKey: "dummy", Model: "image-01"}).(*minimaxImageProvider)
	_, err := provider.Submit(context.Background(), ProviderJobRequest{
		RuntimeJobID: "job-2",
		TaskType:     RuntimeTaskImageGeneration,
		Input: RuntimeInputManifest{
			InputMode:      "image_to_image",
			ParamsSnapshot: map[string]any{"prompt": "test"},
			SourceAssets:   []ProviderSourceAsset{{SourceURL: "https://example.com/source.jpg"}},
		},
	})
	if err == nil || isRetryableProviderError(err) {
		t.Fatalf("expected non-retryable business error, got %v", err)
	}
}
