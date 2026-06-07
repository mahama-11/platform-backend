package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"platform-service/internal/models"

	"github.com/hibiken/asynq"
)

type fakeAsynqClient struct {
	tasks []fakeAsynqTask
}

type fakeAsynqTask struct {
	taskType string
	payload  taskPayload
}

func (c *fakeAsynqClient) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	var payload taskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	c.tasks = append(c.tasks, fakeAsynqTask{taskType: task.Type(), payload: payload})
	return &asynq.TaskInfo{}, nil
}

func (c *fakeAsynqClient) Close() error { return nil }

func TestAsynqRuntimeEnqueuePayloads(t *testing.T) {
	client := &fakeAsynqClient{}
	runtime := &AsynqRuntime{client: client, queueName: "runtime:test"}
	if err := runtime.EnqueueDispatch("runtime-job-1", time.Second); err != nil {
		t.Fatalf("EnqueueDispatch: %v", err)
	}
	if err := runtime.EnqueuePoll("runtime-job-2", 2*time.Second); err != nil {
		t.Fatalf("EnqueuePoll: %v", err)
	}
	if err := runtime.EnqueueCallback("delivery-1", 3*time.Second); err != nil {
		t.Fatalf("EnqueueCallback: %v", err)
	}
	if len(client.tasks) != 3 {
		t.Fatalf("expected 3 enqueued tasks, got %+v", client.tasks)
	}
	assertTaskPayload(t, client.tasks[0], taskTypeDispatch, "runtime-job-1", "")
	assertTaskPayload(t, client.tasks[1], taskTypePoll, "runtime-job-2", "")
	assertTaskPayload(t, client.tasks[2], taskTypeCallback, "", "delivery-1")
}

func TestAsynqWorkerDecodeDispatchPollCallbackAndTerminalNoop(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	now := time.Now()
	if err := repo.CreateProviderDefinition(&models.RuntimeProviderDefinition{ID: "provider-worker", Code: "fake_worker", Name: "Fake Worker", ProviderType: "image_generation", Mode: "async", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateProviderDefinition: %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{ID: "binding-worker", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderCode: "fake_worker", Priority: 1, Enabled: true, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateProviderBinding: %v", err)
	}
	submitted := false
	polled := false
	provider := &fakeProvider{name: "fake_worker", submitFn: func(req ProviderJobRequest) (*ProviderSubmission, error) {
		submitted = true
		if req.RuntimeJobID != "worker-dispatch" || req.ProductCode != "menu" || req.TaskType != RuntimeTaskImageGeneration {
			t.Fatalf("unexpected provider submit request: %+v", req)
		}
		return &ProviderSubmission{ProviderJobID: "provider-worker-dispatch", Stage: "accepted", StageMessage: "accepted", EtaSeconds: 11}, nil
	}, pollFn: func(providerJobID string) (*ProviderPollResult, error) {
		polled = true
		if providerJobID != "provider-worker-poll" {
			t.Fatalf("unexpected provider job id: %s", providerJobID)
		}
		return &ProviderPollResult{Status: "processing", Stage: "provider_running", StageMessage: "running", Progress: 30, EtaSeconds: 9}, nil
	}}
	service.UseRuntime(queue, &ProviderRegistry{providers: map[string]GenerationProvider{"fake_worker": provider}})

	if err := repo.CreateRuntimeJob(&models.RuntimeJob{ID: "worker-dispatch", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderMode: "async", OrganizationID: "org-1", SourceType: "menu_job", SourceID: "menu-job-1", Status: "queued", Stage: "queued", MaxAttempts: 3, InputManifest: `{}`}); err != nil {
		t.Fatalf("Create dispatch job: %v", err)
	}
	if err := handleRuntimeAsynqTask(context.Background(), service, taskTypeDispatch, newRuntimeAsynqTask(t, taskTypeDispatch, taskPayload{RuntimeJobID: "worker-dispatch"})); err != nil {
		t.Fatalf("handle dispatch task: %v", err)
	}
	if !submitted || len(queue.polls) != 1 || queue.polls[0].RuntimeJobID != "worker-dispatch" {
		t.Fatalf("expected dispatch to submit and enqueue poll, submitted=%v polls=%+v", submitted, queue.polls)
	}
	updated, err := repo.FindRuntimeJobByID("worker-dispatch")
	if err != nil || updated.ProviderCode != "fake_worker" || updated.ProviderJobID != "provider-worker-dispatch" || updated.Status != "processing" {
		t.Fatalf("unexpected dispatched job: %+v err=%v", updated, err)
	}

	if err := repo.CreateRuntimeJob(&models.RuntimeJob{ID: "worker-poll", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderCode: "fake_worker", ProviderJobID: "provider-worker-poll", ProviderMode: "async", OrganizationID: "org-1", SourceType: "menu_job", SourceID: "menu-job-2", Status: "processing", Stage: "provider_accepted", MaxAttempts: 3, InputManifest: `{}`}); err != nil {
		t.Fatalf("Create poll job: %v", err)
	}
	if err := handleRuntimeAsynqTask(context.Background(), service, taskTypePoll, newRuntimeAsynqTask(t, taskTypePoll, taskPayload{RuntimeJobID: "worker-poll"})); err != nil {
		t.Fatalf("handle poll task: %v", err)
	}
	if !polled || len(queue.polls) != 2 || queue.polls[1].RuntimeJobID != "worker-poll" {
		t.Fatalf("expected poll to reschedule non-terminal job, polled=%v polls=%+v", polled, queue.polls)
	}

	if err := repo.CreateRuntimeJob(&models.RuntimeJob{ID: "worker-callback-job", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderMode: "async", OrganizationID: "org-1", SourceType: "menu_job", SourceID: "menu-job-3", Status: "processing", Stage: "provider_running", MaxAttempts: 3}); err != nil {
		t.Fatalf("Create callback job: %v", err)
	}
	if err := repo.CreateCallbackDelivery(&models.RuntimeCallbackDelivery{ID: "delivery-worker", RuntimeJobID: "worker-callback-job", ProductCode: "menu", SourceID: "menu-job-3", CallbackType: "runtime_update", Status: "pending", PayloadJSON: `{"status":"processing"}`, MaxAttempts: 2}); err != nil {
		t.Fatalf("CreateCallbackDelivery: %v", err)
	}
	if err := handleRuntimeAsynqTask(context.Background(), service, taskTypeCallback, newRuntimeAsynqTask(t, taskTypeCallback, taskPayload{DeliveryID: "delivery-worker"})); err != nil {
		t.Fatalf("handle callback task: %v", err)
	}
	delivery, err := repo.FindCallbackDeliveryByID("delivery-worker")
	if err != nil || delivery.Status != "dead_letter" || delivery.AttemptCount != 1 {
		t.Fatalf("expected callback without endpoint to dead-letter, delivery=%+v err=%v", delivery, err)
	}

	terminalPollCalled := false
	provider.pollFn = func(string) (*ProviderPollResult, error) {
		terminalPollCalled = true
		return nil, nil
	}
	if err := repo.CreateRuntimeJob(&models.RuntimeJob{ID: "worker-terminal", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderCode: "fake_worker", ProviderJobID: "provider-terminal", ProviderMode: "async", OrganizationID: "org-1", SourceType: "menu_job", SourceID: "menu-job-terminal", Status: "completed", Stage: "completed", MaxAttempts: 3}); err != nil {
		t.Fatalf("Create terminal job: %v", err)
	}
	if err := handleRuntimeAsynqTask(context.Background(), service, taskTypePoll, newRuntimeAsynqTask(t, taskTypePoll, taskPayload{RuntimeJobID: "worker-terminal"})); err != nil {
		t.Fatalf("handle terminal poll task: %v", err)
	}
	if terminalPollCalled {
		t.Fatalf("terminal poll task must be a no-op and not call provider")
	}
}

func TestAsynqWorkerRetryFallbackAndProviderCancel(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	now := time.Now()
	for _, code := range []string{"fake_primary", "fake_fallback"} {
		if err := repo.CreateProviderDefinition(&models.RuntimeProviderDefinition{ID: "provider-" + code, Code: code, Name: code, ProviderType: "image_generation", Mode: "async", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatalf("CreateProviderDefinition %s: %v", code, err)
		}
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{ID: "binding-primary", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderCode: "fake_primary", Priority: 1, Enabled: true, Metadata: `{}`, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("Create primary binding: %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{ID: "binding-fallback", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderCode: "fake_fallback", Priority: 2, Enabled: true, Metadata: `{}`, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("Create fallback binding: %v", err)
	}
	primary := &fakeProvider{name: "fake_primary", submitFn: func(ProviderJobRequest) (*ProviderSubmission, error) {
		return nil, newRetryableProviderError("temporary provider outage")
	}}
	fallback := &fakeProvider{name: "fake_fallback"}
	service.UseRuntime(queue, &ProviderRegistry{providers: map[string]GenerationProvider{"fake_primary": primary, "fake_fallback": fallback}})
	if err := repo.CreateRuntimeJob(&models.RuntimeJob{ID: "worker-fallback", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderCode: "fake_primary", ProviderMode: "async", OrganizationID: "org-1", SourceType: "menu_job", SourceID: "menu-job-fallback", Status: "queued", Stage: "queued", MaxAttempts: 3, InputManifest: `{}`, RouteSnapshot: encodeRouteSnapshot(RuntimeRouteSnapshot{CandidateProviders: []string{"fake_primary", "fake_fallback"}, CurrentProviderIdx: 0})}); err != nil {
		t.Fatalf("Create fallback job: %v", err)
	}
	if err := handleRuntimeAsynqTask(context.Background(), service, taskTypeDispatch, newRuntimeAsynqTask(t, taskTypeDispatch, taskPayload{RuntimeJobID: "worker-fallback"})); err != nil {
		t.Fatalf("handle fallback dispatch task: %v", err)
	}
	updated, err := repo.FindRuntimeJobByID("worker-fallback")
	if err != nil || updated.ProviderCode != "fake_fallback" || updated.Stage != "fallback_scheduled" {
		t.Fatalf("expected fallback provider scheduled, job=%+v err=%v", updated, err)
	}
	if len(queue.dispatches) == 0 || queue.dispatches[len(queue.dispatches)-1].RuntimeJobID != "worker-fallback" {
		t.Fatalf("expected fallback dispatch re-enqueue, got %+v", queue.dispatches)
	}

	if err := repo.CreateRuntimeJob(&models.RuntimeJob{ID: "worker-cancel", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderCode: "fake_fallback", ProviderJobID: "provider-cancel", ProviderMode: "async", OrganizationID: "org-1", SourceType: "menu_job", SourceID: "menu-job-cancel", Status: "processing", Stage: "provider_running", MaxAttempts: 3}); err != nil {
		t.Fatalf("Create cancel job: %v", err)
	}
	canceled, err := service.CancelRuntimeJob("worker-cancel")
	if err != nil || canceled.Status != "canceled" || !fallback.cancelCalled {
		t.Fatalf("expected provider cancel and canceled job, canceled=%+v providerCancel=%v err=%v", canceled, fallback.cancelCalled, err)
	}
}

func assertTaskPayload(t *testing.T, actual fakeAsynqTask, taskType, runtimeJobID, deliveryID string) {
	t.Helper()
	if actual.taskType != taskType || actual.payload.RuntimeJobID != runtimeJobID || actual.payload.DeliveryID != deliveryID {
		t.Fatalf("unexpected task payload: %+v want type=%s runtime_job_id=%s delivery_id=%s", actual, taskType, runtimeJobID, deliveryID)
	}
}

func newRuntimeAsynqTask(t *testing.T, taskType string, payload taskPayload) *asynq.Task {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal task payload: %v", err)
	}
	return asynq.NewTask(taskType, body)
}

func TestDecodeTaskPayloadRejectsInvalidJSON(t *testing.T) {
	if _, err := decodeTaskPayload(asynq.NewTask(taskTypeDispatch, []byte("{bad"))); err == nil || !errors.Is(err, err) {
		t.Fatalf("expected invalid task payload error, got %v", err)
	}
}
