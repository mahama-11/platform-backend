package runtime

import (
	"context"
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
						{Index: 0, SourceURL: "https://example.com/result.png", MimeType: "image/png"},
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
