package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/models"
)

func TestHandleDispatchTaskAndHandlePollTask(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(&fakeProvider{
		name: "comfyui_bridge",
		submitFn: func(req ProviderJobRequest) (*ProviderSubmission, error) {
			return &ProviderSubmission{ProviderJobID: "provider-job-1", Stage: "provider_accepted"}, nil
		},
		pollFn: func(providerJobID string) (*ProviderPollResult, error) {
			return &ProviderPollResult{
				Status: "completed",
				Completion: &ProviderCompletion{
					Status:       "completed",
					Progress:     100,
					StageMessage: "done",
					Variants: []ProviderResultVariant{
						{Index: 0, SourceURL: "https://example.com/result.png", PreviewURL: "https://example.com/preview.png", MimeType: "image/png", Metadata: map[string]any{"provider": "fake"}},
					},
				},
			}, nil
		},
	})
	service.UseRuntime(queue, registry)
	job := &models.RuntimeJob{
		ID:            "job-handle",
		ProductCode:   "ecommerce",
		TaskType:      "image_generation",
		ProviderCode:  "comfyui_bridge",
		ProviderMode:  "async",
		SourceType:    "ecommerce_job",
		SourceID:      "source-1",
		Status:        "queued",
		InputManifest: `{"input_mode":"text_to_image","params_snapshot":{"prompt":"hello"}}`,
		MaxAttempts:   3,
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	if err := service.HandleDispatchTask(context.Background(), job.ID); err != nil {
		t.Fatalf("HandleDispatchTask: %v", err)
	}
	updated, _ := repo.FindRuntimeJobByID(job.ID)
	if updated.ProviderJobID == "" {
		t.Fatalf("expected provider job id after dispatch")
	}
	if err := service.HandlePollTask(context.Background(), job.ID); err != nil {
		t.Fatalf("HandlePollTask: %v", err)
	}
	updated, _ = repo.FindRuntimeJobByID(job.ID)
	if updated.Status != "completed" {
		t.Fatalf("expected completed job after poll, got %+v", updated)
	}
	var manifest struct {
		Variants []struct {
			SourceURL  string         `json:"source_url"`
			PreviewURL string         `json:"preview_url"`
			MimeType   string         `json:"mime_type"`
			Metadata   map[string]any `json:"metadata"`
			Asset      struct {
				SourceURL  string         `json:"source_url"`
				PreviewURL string         `json:"preview_url"`
				MimeType   string         `json:"mime_type"`
				Metadata   map[string]any `json:"metadata"`
			} `json:"asset"`
		} `json:"variants"`
	}
	if err := json.Unmarshal([]byte(updated.OutputManifest), &manifest); err != nil || len(manifest.Variants) != 1 {
		t.Fatalf("decode output manifest: manifest=%+v err=%v", manifest, err)
	}
	variant := manifest.Variants[0]
	if variant.SourceURL != "https://example.com/result.png" || variant.PreviewURL != "https://example.com/preview.png" || variant.MimeType != "image/png" || variant.Metadata["provider"] != "fake" || variant.SourceURL != variant.Asset.SourceURL || variant.PreviewURL != variant.Asset.PreviewURL || variant.MimeType != variant.Asset.MimeType || variant.Metadata["provider"] != variant.Asset.Metadata["provider"] {
		t.Fatalf("output manifest must preserve flat consumer compatibility: %+v", manifest.Variants[0])
	}
}

func TestHandlePollTaskRetriesPollWithoutResubmittingAcceptedJob(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	provider := &fakeProvider{name: "pai_video", pollFn: func(string) (*ProviderPollResult, error) {
		return nil, newRetryableProviderError("temporary poll failure")
	}}
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(provider)
	service.UseRuntime(queue, registry)
	future := time.Now().Add(time.Minute)
	job := &models.RuntimeJob{ID: "job-poll-retry", ProductCode: "novel_video", TaskType: "video_text_to_video", ProviderCode: "pai_video", ProviderJobID: "provider-job-1", Status: "processing", TimeoutAt: &future, MaxAttempts: 3}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request"), "req-1")
	if err := service.HandlePollTask(ctx, job.ID); err != nil {
		t.Fatalf("HandlePollTask: %v", err)
	}
	updated, _ := repo.FindRuntimeJobByID(job.ID)
	if updated.Status != "processing" || updated.ProviderJobID != "provider-job-1" {
		t.Fatalf("accepted job must stay processing: %+v", updated)
	}
	if len(queue.polls) != 1 || len(queue.dispatches) != 0 {
		t.Fatalf("poll error must requeue poll only: polls=%+v dispatches=%+v", queue.polls, queue.dispatches)
	}
	if provider.pollCtx == nil || provider.pollCtx.Value(contextKey("request")) != "req-1" {
		t.Fatalf("caller context was not propagated to provider poll")
	}
}

func TestHandlePollTaskClearsStalePollRetryStateAfterProviderProgress(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	provider := &fakeProvider{name: "pai_video"}
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(provider)
	service.UseRuntime(queue, registry)
	future := time.Now().Add(time.Minute)
	retryAt := time.Now().Add(-time.Second)
	job := &models.RuntimeJob{
		ID: "job-poll-recovered", ProductCode: "novel_video", TaskType: "video_text_to_video",
		ProviderCode: "pai_video", ProviderJobID: "provider-job-1", Status: "processing",
		TimeoutAt: &future, NextRetryAt: &retryAt, ErrorClass: "retryable_provider",
		ErrorCode: "PROVIDER_POLL_FAILED", ErrorMessage: "temporary poll failure",
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	if err := service.HandlePollTask(context.Background(), job.ID); err != nil {
		t.Fatalf("HandlePollTask: %v", err)
	}
	updated, _ := repo.FindRuntimeJobByID(job.ID)
	if updated.NextRetryAt != nil || updated.ErrorClass != "" || updated.ErrorCode != "" || updated.ErrorMessage != "" {
		t.Fatalf("successful provider progress left stale retry state: %+v", updated)
	}
	if len(queue.polls) != 1 || len(queue.dispatches) != 0 {
		t.Fatalf("provider progress must continue polling only: polls=%+v dispatches=%+v", queue.polls, queue.dispatches)
	}
}

func TestHandlePollTaskFailsExpiredJobWithoutCallingProvider(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	provider := &fakeProvider{name: "pai_video"}
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(provider)
	service.UseRuntime(queue, registry)
	past := time.Now().Add(-time.Second)
	job := &models.RuntimeJob{ID: "job-expired", ProductCode: "novel_video", TaskType: "video_text_to_video", ProviderCode: "pai_video", ProviderJobID: "provider-job-1", Status: "processing", TimeoutAt: &past}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	_ = service.HandlePollTask(context.Background(), job.ID)
	updated, _ := repo.FindRuntimeJobByID(job.ID)
	if updated.Status != "failed" || updated.ErrorClass != "provider_timeout" || provider.pollCtx != nil {
		t.Fatalf("expired job was not failed before polling: job=%+v pollCtx=%v", updated, provider.pollCtx)
	}
}

func TestResolveProviderCodeAndProviderCallbackURL(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	service.comfy.CallbackBaseURL = "http://127.0.0.1:8095"
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{
		ID:           "b1",
		ProductCode:  "ecommerce",
		TaskType:     "image_generation",
		ProviderCode: "volcengine",
		Priority:     100,
		Enabled:      true,
		Metadata:     `{"objective_scores":{"quality":80}}`,
	}); err != nil {
		t.Fatalf("CreateProviderBinding: %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{
		ID:           "b2",
		ProductCode:  "ecommerce",
		TaskType:     "image_generation",
		ProviderCode: "comfyui_bridge",
		Priority:     50,
		Enabled:      true,
		Metadata:     `{"objective_scores":{"quality":92}}`,
	}); err != nil {
		t.Fatalf("CreateProviderBinding: %v", err)
	}
	job := &models.RuntimeJob{
		ID:            "job-route",
		ProductCode:   "ecommerce",
		TaskType:      "image_generation",
		RouteSnapshot: `{"objective":"quality"}`,
	}
	providerCode, err := service.resolveProviderCode(job)
	if err != nil || providerCode != "comfyui_bridge" {
		t.Fatalf("unexpected provider resolution: %s err=%v snapshot=%s", providerCode, err, job.RouteSnapshot)
	}
	callbackURL := service.providerCallbackURL(job.ID)
	if callbackURL == "" || callbackURL[:4] != "http" {
		t.Fatalf("expected absolute callback URL, got %s", callbackURL)
	}
}

func TestNotifyProductResultsAndFailRuntimeJob(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	var runtimePath, resultPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/v1/ecommerce/jobs/source-1/runtime" {
			runtimePath = r.URL.Path
		}
		if r.URL.Path == "/internal/v1/ecommerce/jobs/source-1/results" {
			resultPath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := repo.CreateProductEndpoint(&models.RuntimeProductEndpoint{
		ID:           "ep-1",
		ProductCode:  "ecommerce",
		CallbackKind: "ecommerce_internal",
		BaseURL:      server.URL,
		Secret:       "secret",
		Status:       "active",
	}); err != nil {
		t.Fatalf("CreateProductEndpoint: %v", err)
	}
	job := &models.RuntimeJob{
		ID:          "job-fail",
		ProductCode: "ecommerce",
		TaskType:    "image_generation",
		SourceType:  "ecommerce_job",
		SourceID:    "source-1",
		Status:      "processing",
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	if err := service.notifyProductResults(job, ProductRecordResultsInput{Status: "completed"}); err != nil {
		t.Fatalf("notifyProductResults: %v", err)
	}
	if len(queue.callbacks) != 1 {
		t.Fatalf("expected 1 callback delivery, got %+v", queue.callbacks)
	}
	if err := service.HandleCallbackTask(context.Background(), queue.callbacks[0].RuntimeJobID); err != nil {
		t.Fatalf("HandleCallbackTask: %v", err)
	}
	if err := service.failRuntimeJob(job, "provider_timeout", "TIMEOUT", "timed out", time.Now()); err != nil {
		t.Fatalf("failRuntimeJob: %v", err)
	}
	if len(queue.callbacks) != 2 {
		t.Fatalf("expected 2 callback deliveries after failRuntimeJob, got %+v", queue.callbacks)
	}
	if err := service.HandleCallbackTask(context.Background(), queue.callbacks[1].RuntimeJobID); err != nil {
		t.Fatalf("HandleCallbackTask(runtime update): %v", err)
	}
	if resultPath == "" || runtimePath == "" {
		t.Fatalf("expected both callback paths to be called, runtime=%s results=%s", runtimePath, resultPath)
	}
}
