package runtime

import (
	"testing"
	"time"

	"platform-service/internal/models"
)

func TestDispatchRuntimeJobAcceptsAsyncSubmissionAndEnqueuesPoll(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(&fakeProvider{
		name: "comfyui_bridge",
		submitFn: func(req ProviderJobRequest) (*ProviderSubmission, error) {
			return &ProviderSubmission{
				ProviderJobID: "provider-job-1",
				Stage:         "provider_accepted",
				StageMessage:  "accepted",
				EtaSeconds:    12,
			}, nil
		},
	})
	service.UseRuntime(queue, registry)

	job := &models.RuntimeJob{
		ID:             "job-1",
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderCode:   "comfyui_bridge",
		ProviderMode:   "async",
		OrganizationID: "org-1",
		SourceType:     "ecommerce_job",
		SourceID:       "source-1",
		Status:         "queued",
		Stage:          "queued",
		InputManifest:  `{"input_mode":"text_to_image","params_snapshot":{"prompt":"hello"}}`,
		MaxAttempts:    3,
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	if err := service.dispatchRuntimeJob(job, time.Now()); err != nil {
		t.Fatalf("dispatchRuntimeJob: %v", err)
	}
	updated, err := repo.FindRuntimeJobByID(job.ID)
	if err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
	if updated.ProviderJobID != "provider-job-1" || updated.Status != "processing" {
		t.Fatalf("unexpected updated job: %+v", updated)
	}
	if len(queue.polls) != 1 || queue.polls[0].RuntimeJobID != job.ID {
		t.Fatalf("expected poll enqueue, got %+v", queue.polls)
	}
	attempts, err := repo.ListRuntimeAttempts(job.ID)
	if err != nil || len(attempts) != 1 || attempts[0].ProviderCode != "comfyui_bridge" {
		t.Fatalf("unexpected attempts: %+v err=%v", attempts, err)
	}
}

func TestHandleDispatchErrorSchedulesFallback(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{
		ID:             "job-fallback",
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderCode:   "comfyui_bridge",
		ProviderMode:   "async",
		OrganizationID: "org-1",
		SourceType:     "ecommerce_job",
		SourceID:       "source-1",
		Status:         "processing",
		RouteSnapshot:  `{"candidate_providers":["comfyui_bridge","volcengine"],"current_provider_idx":0}`,
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{
		ID:           "binding-1",
		ProductCode:  "ecommerce",
		TaskType:     "image_generation",
		ProviderCode: "comfyui_bridge",
		Priority:     50,
		Enabled:      true,
		Metadata:     `{"fallback_on":["retryable_provider"]}`,
	}); err != nil {
		t.Fatalf("CreateProviderBinding: %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{
		ID:           "binding-2",
		ProductCode:  "ecommerce",
		TaskType:     "image_generation",
		ProviderCode: "volcengine",
		Priority:     100,
		Enabled:      true,
		Metadata:     `{}`,
	}); err != nil {
		t.Fatalf("CreateProviderBinding: %v", err)
	}

	err := service.handleDispatchError(job, newRetryableProviderError("temporary outage"), time.Now())
	if err != nil {
		t.Fatalf("handleDispatchError: %v", err)
	}
	updated, err := repo.FindRuntimeJobByID(job.ID)
	if err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
	if updated.ProviderCode != "volcengine" || updated.Stage != "fallback_scheduled" {
		t.Fatalf("unexpected updated job after fallback: %+v", updated)
	}
	if len(queue.dispatches) != 1 || queue.dispatches[0].RuntimeJobID != job.ID {
		t.Fatalf("expected dispatch enqueue for fallback, got %+v", queue.dispatches)
	}
}
