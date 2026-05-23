package runtime

import (
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
)

func TestApplyRuntimeJobTransitionDispatchStarted(t *testing.T) {
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	timeoutAt := now.Add(5 * time.Minute)
	job := &models.RuntimeJob{ID: "job-1", Status: platformconst.StatusQueued, Stage: platformconst.StatusQueued, AttemptCount: 1}

	result, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{
		Event:            RuntimeJobEventDispatchStarted,
		Now:              now,
		ProviderCode:     "comfyui_bridge",
		Stage:            "dispatching",
		StageMessage:     "Dispatching to provider",
		IncrementAttempt: true,
		TimeoutAt:        &timeoutAt,
	})
	if err != nil {
		t.Fatalf("ApplyRuntimeJobTransition: %v", err)
	}
	if result.Noop {
		t.Fatalf("dispatch_started should not no-op")
	}
	if job.Status != platformconst.StatusProcessing || job.ProviderCode != "comfyui_bridge" || job.Stage != "dispatching" || job.StageMessage != "Dispatching to provider" || job.AttemptCount != 2 || job.TimeoutAt == nil || !job.TimeoutAt.Equal(timeoutAt) {
		t.Fatalf("unexpected dispatch transition result: %+v", job)
	}
}

func TestApplyRuntimeJobTransitionRetryAndFallbackAllowProcessingToQueued(t *testing.T) {
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	retryAt := now.Add(time.Minute)
	for _, tc := range []struct {
		name  string
		from  string
		event RuntimeJobTransitionEvent
		stage string
	}{
		{"processing_retry", platformconst.StatusProcessing, RuntimeJobEventRetryScheduled, "retry_scheduled"},
		{"processing_fallback", platformconst.StatusProcessing, RuntimeJobEventFallbackScheduled, "fallback_scheduled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := &models.RuntimeJob{ID: tc.name, Status: tc.from, Stage: tc.from}
			result, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{
				Event:        tc.event,
				Now:          now,
				Stage:        tc.stage,
				StageMessage: "scheduled",
				NextRetryAt:  &retryAt,
				ErrorClass:   "retryable_provider",
				ErrorCode:    "PROVIDER_SUBMIT_FAILED",
				ErrorMessage: "temporary outage",
			})
			if err != nil {
				t.Fatalf("ApplyRuntimeJobTransition: %v", err)
			}
			if result.Noop || job.Status != platformconst.StatusQueued || job.Stage != tc.stage || job.ErrorMessage != "temporary outage" {
				t.Fatalf("unexpected scheduled transition: result=%+v job=%+v", result, job)
			}
		})
	}
}

func TestApplyRuntimeJobTransitionFailedRetryFallbackErrors(t *testing.T) {
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	retryAt := now.Add(time.Minute)
	completedAt := now.Add(-time.Minute)
	for _, event := range []RuntimeJobTransitionEvent{RuntimeJobEventRetryScheduled, RuntimeJobEventFallbackScheduled} {
		t.Run(string(event), func(t *testing.T) {
			job := &models.RuntimeJob{ID: string(event), Status: platformconst.StatusFailed, Stage: platformconst.StatusFailed, StageMessage: "failed", CompletedAt: &completedAt, ErrorMessage: "terminal failure"}
			_, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{
				Event:        event,
				Now:          now,
				Stage:        "retry_scheduled",
				StageMessage: "scheduled",
				NextRetryAt:  &retryAt,
				ErrorMessage: "temporary outage",
			})
			if err == nil {
				t.Fatalf("expected failed -> queued via %s to error", event)
			}
			if job.Status != platformconst.StatusFailed || job.Stage != platformconst.StatusFailed || job.StageMessage != "failed" || job.ErrorMessage != "terminal failure" || job.NextRetryAt != nil || job.CompletedAt == nil || !job.CompletedAt.Equal(completedAt) {
				t.Fatalf("failed retry/fallback mutated terminal job: %+v", job)
			}
		})
	}
}

func TestApplyRuntimeJobTransitionFailedAdminQueuedErrors(t *testing.T) {
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	completedAt := now.Add(-time.Minute)
	job := &models.RuntimeJob{
		ID:             "job-failed-admin-queued",
		Status:         platformconst.StatusFailed,
		Stage:          platformconst.StatusFailed,
		StageMessage:   "failed",
		CompletedAt:    &completedAt,
		ErrorMessage:   "terminal failure",
		OutputManifest: `{"failed":true}`,
	}
	result, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{
		Event:        RuntimeJobEventAdminPatch,
		Now:          now,
		Status:       platformconst.StatusQueued,
		Stage:        platformconst.StatusQueued,
		StageMessage: "retry queued",
	})
	if err == nil {
		t.Fatalf("expected admin failed -> queued to error, got result=%+v", result)
	}
	if job.Status != platformconst.StatusFailed || job.Stage != platformconst.StatusFailed || job.StageMessage != "failed" || job.ErrorMessage != "terminal failure" || job.OutputManifest != `{"failed":true}` || job.CompletedAt == nil || !job.CompletedAt.Equal(completedAt) {
		t.Fatalf("admin failed -> queued mutated terminal job: %+v", job)
	}
}

func TestApplyRuntimeJobTransitionTerminalProviderAcceptedNoops(t *testing.T) {
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	completedAt := now.Add(-time.Hour)
	canceledAt := now.Add(-30 * time.Minute)
	for _, tc := range []struct {
		name       string
		status     string
		completed  *time.Time
		canceled   *time.Time
		output     string
		errorMsg   string
		providerID string
	}{
		{name: "completed", status: platformconst.StatusCompleted, completed: &completedAt, output: `{"ok":true}`, providerID: "provider-original"},
		{name: "failed", status: platformconst.StatusFailed, completed: &completedAt, errorMsg: "terminal failure", providerID: "provider-original"},
		{name: "canceled", status: platformconst.StatusCanceled, canceled: &canceledAt, providerID: "provider-original"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := &models.RuntimeJob{ID: "job-" + tc.name, Status: tc.status, Stage: tc.status, StageMessage: "terminal", ProviderJobID: tc.providerID, OutputManifest: tc.output, ErrorMessage: tc.errorMsg, CompletedAt: tc.completed, CanceledAt: tc.canceled}
			result, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{
				Event:          RuntimeJobEventProviderAccepted,
				Now:            now,
				Stage:          "provider_accepted",
				StageMessage:   "accepted late",
				ProviderJobID:  "provider-late",
				OutputManifest: `{"late":true}`,
			})
			if err != nil {
				t.Fatalf("terminal provider accepted should no-op without error: %v", err)
			}
			if !result.Noop || result.NoopReason == "" || result.ToStatus != tc.status {
				t.Fatalf("expected terminal accepted no-op, got %+v", result)
			}
			if job.Status != tc.status || job.Stage != tc.status || job.StageMessage != "terminal" || job.ProviderJobID != tc.providerID || job.OutputManifest != tc.output || job.ErrorMessage != tc.errorMsg || job.CompletedAt != tc.completed || job.CanceledAt != tc.canceled {
				t.Fatalf("terminal accepted mutated job: %+v", job)
			}
		})
	}
}

func TestApplyRuntimeJobTransitionCompletedProviderProgressNoops(t *testing.T) {
	completedAt := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	job := &models.RuntimeJob{ID: "job-terminal-progress", Status: platformconst.StatusCompleted, Stage: platformconst.StatusCompleted, StageMessage: "done", OutputManifest: `{"ok":true}`, CompletedAt: &completedAt}

	result, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{
		Event:        RuntimeJobEventProviderProgress,
		Now:          completedAt.Add(time.Minute),
		Stage:        "provider_running",
		StageMessage: "still running",
	})
	if err != nil {
		t.Fatalf("terminal provider progress should no-op without error: %v", err)
	}
	if !result.Noop || result.NoopReason == "" {
		t.Fatalf("expected terminal progress no-op, got %+v", result)
	}
	if job.Status != platformconst.StatusCompleted || job.Stage != platformconst.StatusCompleted || job.StageMessage != "done" || job.OutputManifest != `{"ok":true}` || job.CompletedAt == nil || !job.CompletedAt.Equal(completedAt) {
		t.Fatalf("terminal progress downgraded/mutated job: %+v", job)
	}
}

func TestApplyRuntimeJobTransitionCompletedRetryFallbackErrors(t *testing.T) {
	for _, event := range []RuntimeJobTransitionEvent{RuntimeJobEventRetryScheduled, RuntimeJobEventFallbackScheduled} {
		job := &models.RuntimeJob{ID: string(event), Status: platformconst.StatusCompleted, Stage: platformconst.StatusCompleted}
		if _, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{Event: event, Now: time.Now()}); err == nil {
			t.Fatalf("expected completed -> %s to error", event)
		}
		if job.Status != platformconst.StatusCompleted {
			t.Fatalf("terminal status changed after failed event %s: %+v", event, job)
		}
	}
}

func TestApplyRuntimeJobTransitionFailedToCompletedErrors(t *testing.T) {
	job := &models.RuntimeJob{ID: "job-failed", Status: platformconst.StatusFailed, Stage: platformconst.StatusFailed}
	if _, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{Event: RuntimeJobEventCompleted, Now: time.Now(), Stage: platformconst.StatusCompleted}); err == nil {
		t.Fatalf("expected failed -> completed without retry to error")
	}
	if job.Status != platformconst.StatusFailed {
		t.Fatalf("failed job status changed: %+v", job)
	}
}

func TestApplyRuntimeJobTransitionTerminalMetadataPatchKeepsCompletedStatus(t *testing.T) {
	completedAt := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	job := &models.RuntimeJob{ID: "job-terminal-patch", Status: platformconst.StatusCompleted, Stage: platformconst.StatusCompleted, CompletedAt: &completedAt}

	result, err := ApplyRuntimeJobTransition(job, RuntimeJobTransitionInput{
		Event:        RuntimeJobEventTerminalMetadataPatch,
		Now:          completedAt.Add(time.Minute),
		Stage:        "callback_results_failed",
		StageMessage: "Result callback failed; runtime output remains available",
		ErrorClass:   "callback_failed",
		ErrorCode:    "PRODUCT_RESULT_CALLBACK_FAILED",
		ErrorMessage: "boom",
	})
	if err != nil {
		t.Fatalf("terminal metadata patch: %v", err)
	}
	if result.Noop || job.Status != platformconst.StatusCompleted || job.Stage != "callback_results_failed" || job.CompletedAt == nil || !job.CompletedAt.Equal(completedAt) {
		t.Fatalf("unexpected terminal patch result=%+v job=%+v", result, job)
	}
}
