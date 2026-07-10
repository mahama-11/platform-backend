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

func TestPaiVideoEndpointCoverageMatchesOfficialVideoAPIs(t *testing.T) {
	provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{}).(*paiVideoProvider)
	cases := map[string]string{
		"video_text_to_video":    "/video/text/generate",
		"video_image_to_video":   "/video/img/generate",
		"video_template":         "/video/img/generate",
		"video_transition":       "/video/transition/generate",
		"video_extend":           "/video/extend/generate",
		"video_fusion":           "/video/fusion/generate",
		"video_lip_sync":         "/video/lip_sync/generate",
		"video_sound_effect":     "/video/sound_effect/generate",
		"video_restyle":          "/video/restyle/generate",
		"video_swap":             "/video/swap/generate",
		"video_multi_transition": "/video/multi_transition/generate",
		"video_mimic":            "/video/mimic/generate",
		"video_modify":           "/video/modify/generate",
	}
	for taskType, want := range cases {
		t.Run(taskType, func(t *testing.T) {
			got, err := provider.endpointForTask(taskType, "")
			if err != nil {
				t.Fatalf("endpointForTask(%s): %v", taskType, err)
			}
			if got != want {
				t.Fatalf("endpointForTask(%s)=%s, want %s", taskType, got, want)
			}
		})
	}

	actionCases := map[string]struct {
		path   string
		method string
	}{
		"restyle_list":        {path: "/video/restyle/list", method: http.MethodGet},
		"swap_mask_selection": {path: "/video/mask/selection", method: http.MethodPost},
	}
	for action, want := range actionCases {
		t.Run(action, func(t *testing.T) {
			path, method, err := provider.endpointForProviderAction(action)
			if err != nil {
				t.Fatalf("endpointForProviderAction(%s): %v", action, err)
			}
			if path != want.path || method != want.method {
				t.Fatalf("endpointForProviderAction(%s)=(%s,%s), want (%s,%s)", action, path, method, want.path, want.method)
			}
		})
	}
}

func TestPaiVideoSubmitUsesOfficialEnvelopeAndPreservesOfficialTextOptions(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/video/text/generate" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if r.Header.Get("API-KEY") != "test-key" {
			t.Fatalf("missing API-KEY header: %s", r.Header.Get("API-KEY"))
		}
		if !strings.HasPrefix(r.Header.Get("Ai-trace-id"), "platform-runtime-") {
			t.Fatalf("missing Ai-trace-id header: %s", r.Header.Get("Ai-trace-id"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ErrCode": 0,
			"ErrMsg":  "Success",
			"Resp": map[string]any{
				"video_id": 12345,
				"credits":  8,
			},
		})
	}))
	defer server.Close()

	provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key", DefaultModel: "v6"}).(*paiVideoProvider)
	submission, err := provider.Submit(context.Background(), ProviderJobRequest{
		TaskType: "video_text_to_video",
		Input: RuntimeInputManifest{
			ParamsSnapshot: map[string]any{
				"prompt":                  "cat running",
				"batch_count":             4,
				"sound_effect_switch":     true,
				"sound_effect_content":    "wind",
				"lip_sync_tts_switch":     true,
				"lip_sync_tts_content":    "hello",
				"lip_sync_tts_speaker_id": "auto",
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submission.ProviderJobID != "12345" {
		t.Fatalf("ProviderJobID=%s, want 12345", submission.ProviderJobID)
	}
	if _, ok := received["batch_count"]; ok {
		t.Fatalf("batch_count should not be forwarded to upstream: %+v", received)
	}
	for _, key := range []string{"sound_effect_switch", "sound_effect_content", "lip_sync_tts_switch", "lip_sync_tts_content", "lip_sync_tts_speaker_id"} {
		if _, ok := received[key]; !ok {
			t.Fatalf("official text/img option %s should be preserved: %+v", key, received)
		}
	}
	if received["model"] != "v6" {
		t.Fatalf("default model not applied: %+v", received)
	}
}

func TestPaiVideoProviderActionProxiesOfficialUtilityAPIs(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/video/mask/selection" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode action request: %v", err)
			}
			if body["source_video_id"] == nil || body["keyframe_id"] == nil {
				t.Fatalf("mask action payload not forwarded: %+v", body)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ErrCode": 0,
			"ErrMsg":  "Success",
			"Resp": map[string]any{
				"ok": true,
			},
		})
	}))
	defer server.Close()

	provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key"}).(*paiVideoProvider)
	if _, err := provider.ProviderAction(context.Background(), "restyle_list", nil); err != nil {
		t.Fatalf("restyle_list action: %v", err)
	}
	if _, err := provider.ProviderAction(context.Background(), "swap_mask_selection", map[string]any{"source_video_id": 1, "keyframe_id": 1}); err != nil {
		t.Fatalf("swap_mask_selection action: %v", err)
	}
	want := []string{"GET /video/restyle/list", "POST /video/mask/selection"}
	if len(paths) != len(want) {
		t.Fatalf("paths=%v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths=%v, want %v", paths, want)
		}
	}
}

func TestPaiVideoRejectsMalformedOfficialEnvelopeAndClassifiesUpstreamErrors(t *testing.T) {
	t.Run("missing ErrCode is non retryable malformed envelope", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"Resp": map[string]any{"video_id": "v1"}})
		}))
		defer server.Close()
		provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key"}).(*paiVideoProvider)
		_, err := provider.Submit(context.Background(), ProviderJobRequest{TaskType: "video_text_to_video", Input: RuntimeInputManifest{ParamsSnapshot: map[string]any{"prompt": "cat"}}})
		if err == nil {
			t.Fatalf("expected malformed envelope error")
		}
		if isRetryableProviderError(err) {
			t.Fatalf("malformed official envelope should be non-retryable, got %v", err)
		}
		if providerErrorCode(err) != "PAI_VIDEO_ENVELOPE_INVALID" {
			t.Fatalf("unexpected provider error code: %s err=%v", providerErrorCode(err), err)
		}
	})

	t.Run("non zero ErrCode is normalized with upstream code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"ErrCode": 400011, "ErrMsg": "invalid model", "Resp": map[string]any{}})
		}))
		defer server.Close()
		provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key"}).(*paiVideoProvider)
		_, err := provider.Submit(context.Background(), ProviderJobRequest{TaskType: "video_text_to_video", Input: RuntimeInputManifest{ParamsSnapshot: map[string]any{"prompt": "cat"}}})
		if err == nil {
			t.Fatalf("expected upstream error")
		}
		if isRetryableProviderError(err) {
			t.Fatalf("4xx-like ErrCode should be non-retryable, got %v", err)
		}
		if providerErrorCode(err) != "PAI_VIDEO_400011" {
			t.Fatalf("unexpected provider error code: %s err=%v", providerErrorCode(err), err)
		}
	})
}

func TestPaiVideoSubmitDoesNotBlindlyRetryAmbiguousAcceptance(t *testing.T) {
	t.Run("success envelope without video id", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"ErrCode": 0, "ErrMsg": "Success", "Resp": map[string]any{}})
		}))
		defer server.Close()
		provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key"}).(*paiVideoProvider)
		_, err := provider.Submit(context.Background(), ProviderJobRequest{TaskType: "video_text_to_video", Input: RuntimeInputManifest{ParamsSnapshot: map[string]any{"prompt": "cat"}}})
		if err == nil || isRetryableProviderError(err) {
			t.Fatalf("missing video id must be non-retryable, got %v", err)
		}
	})
	t.Run("timed out submit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(50 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(map[string]any{"ErrCode": 0, "Resp": map[string]any{"video_id": "late"}})
		}))
		defer server.Close()
		provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key", RequestTimeout: 10 * time.Millisecond}).(*paiVideoProvider)
		_, err := provider.Submit(context.Background(), ProviderJobRequest{TaskType: "video_text_to_video", Input: RuntimeInputManifest{ParamsSnapshot: map[string]any{"prompt": "cat"}}})
		if err == nil || isRetryableProviderError(err) {
			t.Fatalf("unknown submit outcome must be non-retryable, got %v", err)
		}
	})
}

func TestPaiVideoRequestTraceHeaderUsesContextCorrelation(t *testing.T) {
	var traceHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceHeader = r.Header.Get("Ai-trace-id")
		_ = json.NewEncoder(w).Encode(map[string]any{"ErrCode": 0, "ErrMsg": "Success", "Resp": map[string]any{"credit_package": 100}})
	}))
	defer server.Close()
	provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key"}).(*paiVideoProvider)
	ctx := context.WithValue(context.Background(), "request_id", "req-123")
	ctx = context.WithValue(ctx, "trace_id", "trace-abc")
	if _, err := provider.Balance(ctx); err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if !strings.Contains(traceHeader, "req-123") || !strings.Contains(traceHeader, "trace-abc") {
		t.Fatalf("Ai-trace-id should include request and trace ids, got %q", traceHeader)
	}
}

func TestPaiVideoNoGenerationFakeUpstreamContracts(t *testing.T) {
	paths := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths[r.Method+" "+r.URL.Path]++
		resp := map[string]any{"ok": true}
		switch r.URL.Path {
		case "/account/balance":
			resp = map[string]any{"credit_package": 100}
		case "/video/lip_sync/tts_list":
			resp = map[string]any{"voices": []any{map[string]any{"speaker_id": "auto"}}}
		case "/image/upload":
			resp = map[string]any{"img_id": 101}
		case "/media/upload":
			resp = map[string]any{"media_id": 202}
		case "/video/restyle/list":
			resp = map[string]any{"styles": []any{"anime"}}
		case "/video/mask/selection":
			resp = map[string]any{"mask_id": "m1"}
		case "/video/result/job-1":
			resp = map[string]any{"status": 5}
		default:
			t.Fatalf("unexpected upstream no-generation path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ErrCode": 0, "ErrMsg": "Success", "Resp": resp})
	}))
	defer server.Close()
	provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key"}).(*paiVideoProvider)
	if _, err := provider.Balance(context.Background()); err != nil {
		t.Fatalf("balance: %v", err)
	}
	if _, err := provider.TTSVoices(context.Background()); err != nil {
		t.Fatalf("tts: %v", err)
	}
	if _, err := provider.UploadFile(context.Background(), "image", "a.png", strings.NewReader("png")); err != nil {
		t.Fatalf("image upload: %v", err)
	}
	if _, err := provider.UploadURL(context.Background(), "https://example.com/a.mp4"); err != nil {
		t.Fatalf("upload url: %v", err)
	}
	if _, err := provider.ProviderAction(context.Background(), "restyle_list", nil); err != nil {
		t.Fatalf("restyle list: %v", err)
	}
	if _, err := provider.ProviderAction(context.Background(), "swap_mask_selection", map[string]any{"source_video_id": "v1", "keyframe_id": 1}); err != nil {
		t.Fatalf("mask selection: %v", err)
	}
	poll, err := provider.Poll(context.Background(), "job-1")
	if err != nil || poll.Status != "processing" {
		t.Fatalf("poll=%+v err=%v", poll, err)
	}
	for _, key := range []string{"GET /account/balance", "GET /video/lip_sync/tts_list", "POST /image/upload", "POST /media/upload", "GET /video/restyle/list", "POST /video/mask/selection", "GET /video/result/job-1"} {
		if paths[key] == 0 {
			t.Fatalf("missing no-generation path %s, paths=%v", key, paths)
		}
	}
}

func TestPaiVideoPollNormalizesCompletedAndFailedResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"status": 5}
		switch r.URL.Path {
		case "/video/result/completed":
			resp = map[string]any{"status": 1, "url": "https://cdn.example.com/video.mp4", "cover_url": "https://cdn.example.com/cover.jpg"}
		case "/video/result/failed":
			resp = map[string]any{"status": 7, "message": "upstream failed"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ErrCode": 0, "ErrMsg": "Success", "Resp": resp})
	}))
	defer server.Close()
	provider := newPaiVideoProvider("pai_video", config.PaiVideoConfig{Enabled: true, BaseURL: server.URL, APIKey: "test-key"}).(*paiVideoProvider)

	completed, err := provider.Poll(context.Background(), "completed")
	if err != nil || completed.Status != "completed" || completed.Completion == nil || len(completed.Completion.Variants) != 1 || completed.Completion.Variants[0].SourceURL == "" {
		t.Fatalf("completed poll=%+v err=%v", completed, err)
	}
	failed, err := provider.Poll(context.Background(), "failed")
	if err != nil || failed.Status != "failed" || failed.ErrorMessage != "upstream failed" {
		t.Fatalf("failed poll=%+v err=%v", failed, err)
	}
	if _, err := provider.Poll(context.Background(), ""); err == nil || isRetryableProviderError(err) {
		t.Fatalf("empty provider job id should be non-retryable: %v", err)
	}
	if provider.Name() != "pai_video" {
		t.Fatalf("provider name=%s", provider.Name())
	}
	if err := provider.Cancel(context.Background(), "completed"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
}
