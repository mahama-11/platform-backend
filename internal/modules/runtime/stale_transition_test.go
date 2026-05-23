package runtime

import (
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
)

func TestTransitionRuntimeJobStaleProviderEventsDoNotOverwriteTerminalDBRow(t *testing.T) {
	for _, event := range []RuntimeJobTransitionEvent{RuntimeJobEventProviderProgress, RuntimeJobEventProviderAccepted} {
		t.Run(string(event), func(t *testing.T) {
			service, repo, _ := newRuntimeServiceForTest(t)
			completedAt := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
			terminalJob := &models.RuntimeJob{
				ID:             "job-stale-" + string(event),
				ProductCode:    "ecommerce",
				TaskType:       "image_generation",
				ProviderMode:   "async",
				ProviderJobID:  "provider-original",
				OrganizationID: "org-1",
				SourceType:     "product",
				SourceID:       "source-1",
				Status:         platformconst.StatusCompleted,
				Stage:          platformconst.StatusCompleted,
				StageMessage:   "done",
				OutputManifest: `{"ok":true}`,
				CompletedAt:    &completedAt,
			}
			if err := repo.CreateRuntimeJob(terminalJob); err != nil {
				t.Fatalf("CreateRuntimeJob: %v", err)
			}

			stale := *terminalJob
			stale.Status = platformconst.StatusProcessing
			stale.Stage = platformconst.StatusProcessing
			stale.StageMessage = "old processing copy"
			stale.ProviderJobID = "provider-old"
			stale.OutputManifest = ""
			stale.CompletedAt = nil

			updated, result, err := service.transitionRuntimeJob(&stale, RuntimeJobTransitionInput{
				Event:          event,
				Now:            completedAt.Add(time.Minute),
				Stage:          "provider_late",
				StageMessage:   "late provider event",
				ProviderJobID:  "provider-late",
				OutputManifest: `{"late":true}`,
			})
			if err != nil {
				t.Fatalf("transitionRuntimeJob stale %s: %v", event, err)
			}
			if !result.Noop {
				t.Fatalf("expected stale %s to no-op, got %+v", event, result)
			}
			if result.FromStatus != platformconst.StatusCompleted || result.ToStatus != platformconst.StatusCompleted {
				t.Fatalf("expected result statuses to reflect locked DB row, got %+v", result)
			}
			if updated.Status != platformconst.StatusCompleted || updated.Stage != platformconst.StatusCompleted || updated.ProviderJobID != "provider-original" || updated.OutputManifest != `{"ok":true}` || updated.CompletedAt == nil || !updated.CompletedAt.Equal(completedAt) {
				t.Fatalf("expected returned job to reflect terminal DB row, got %+v", updated)
			}

			reloaded, err := repo.FindRuntimeJobByID(terminalJob.ID)
			if err != nil {
				t.Fatalf("FindRuntimeJobByID: %v", err)
			}
			if reloaded.Status != platformconst.StatusCompleted || reloaded.Stage != platformconst.StatusCompleted || reloaded.StageMessage != "done" || reloaded.ProviderJobID != "provider-original" || reloaded.OutputManifest != `{"ok":true}` || reloaded.CompletedAt == nil || !reloaded.CompletedAt.Equal(completedAt) {
				t.Fatalf("stale %s mutated terminal DB row: %+v", event, reloaded)
			}
		})
	}
}

func TestUpdateRuntimeJobRejectsFailedToQueued(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	completedAt := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	job := &models.RuntimeJob{
		ID:             "job-admin-failed-queued",
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderMode:   "async",
		OrganizationID: "org-1",
		SourceType:     "product",
		SourceID:       "source-1",
		Status:         platformconst.StatusFailed,
		Stage:          platformconst.StatusFailed,
		StageMessage:   "failed",
		ErrorMessage:   "terminal failure",
		CompletedAt:    &completedAt,
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}

	updated, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{
		Status:       platformconst.StatusQueued,
		Stage:        platformconst.StatusQueued,
		StageMessage: "retry queued",
	})
	if err == nil {
		t.Fatalf("expected UpdateRuntimeJob failed -> queued to error, got %+v", updated)
	}
	reloaded, err := repo.FindRuntimeJobByID(job.ID)
	if err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
	if reloaded.Status != platformconst.StatusFailed || reloaded.Stage != platformconst.StatusFailed || reloaded.StageMessage != "failed" || reloaded.ErrorMessage != "terminal failure" || reloaded.CompletedAt == nil || !reloaded.CompletedAt.Equal(completedAt) {
		t.Fatalf("failed -> queued admin patch mutated DB row: %+v", reloaded)
	}
}

func TestTransitionRuntimeJobTerminalMetadataPatchUsesLockedLatestRow(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	completedAt := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	terminalJob := &models.RuntimeJob{
		ID:             "job-terminal-metadata-latest",
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderMode:   "async",
		ProviderJobID:  "provider-original",
		OrganizationID: "org-1",
		SourceType:     "product",
		SourceID:       "source-1",
		Status:         platformconst.StatusCompleted,
		Stage:          platformconst.StatusCompleted,
		StageMessage:   "done",
		OutputManifest: `{"ok":true}`,
		CompletedAt:    &completedAt,
	}
	if err := repo.CreateRuntimeJob(terminalJob); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}

	stale := *terminalJob
	stale.Status = platformconst.StatusProcessing
	stale.Stage = platformconst.StatusProcessing
	stale.StageMessage = "old processing copy"
	stale.OutputManifest = ""
	stale.CompletedAt = nil

	updated, result, err := service.transitionRuntimeJob(&stale, RuntimeJobTransitionInput{
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
	if result.Noop || result.FromStatus != platformconst.StatusCompleted || result.ToStatus != platformconst.StatusCompleted {
		t.Fatalf("expected terminal metadata patch to use locked completed row, got %+v", result)
	}
	if updated.Status != platformconst.StatusCompleted || updated.Stage != "callback_results_failed" || updated.OutputManifest != `{"ok":true}` || updated.CompletedAt == nil || !updated.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected updated latest terminal row, got %+v", updated)
	}
	reloaded, err := repo.FindRuntimeJobByID(terminalJob.ID)
	if err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
	if reloaded.Status != platformconst.StatusCompleted || reloaded.Stage != "callback_results_failed" || reloaded.StageMessage == "old processing copy" || reloaded.OutputManifest != `{"ok":true}` || reloaded.CompletedAt == nil || !reloaded.CompletedAt.Equal(completedAt) {
		t.Fatalf("terminal metadata patch did not preserve latest terminal row: %+v", reloaded)
	}
}
