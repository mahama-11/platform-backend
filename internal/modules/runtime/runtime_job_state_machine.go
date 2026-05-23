package runtime

import (
	"fmt"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RuntimeJobTransitionEvent string

const (
	RuntimeJobEventAdminPatch            RuntimeJobTransitionEvent = "admin_patch"
	RuntimeJobEventDispatchStarted       RuntimeJobTransitionEvent = "dispatch_started"
	RuntimeJobEventProviderAccepted      RuntimeJobTransitionEvent = "provider_accepted"
	RuntimeJobEventProviderProgress      RuntimeJobTransitionEvent = "provider_progress"
	RuntimeJobEventRetryScheduled        RuntimeJobTransitionEvent = "retry_scheduled"
	RuntimeJobEventFallbackScheduled     RuntimeJobTransitionEvent = "fallback_scheduled"
	RuntimeJobEventCompleted             RuntimeJobTransitionEvent = "completed"
	RuntimeJobEventFailed                RuntimeJobTransitionEvent = "failed"
	RuntimeJobEventCanceled              RuntimeJobTransitionEvent = "canceled"
	RuntimeJobEventTerminalMetadataPatch RuntimeJobTransitionEvent = "terminal_metadata_patch"
)

type RuntimeJobTransitionInput struct {
	Event RuntimeJobTransitionEvent
	Now   time.Time

	Status       string
	Stage        string
	StageMessage string

	ProviderCode   string
	ProviderJobID  string
	RouteSnapshot  string
	Metadata       string
	OutputManifest string

	ErrorClass   string
	ErrorCode    string
	ErrorMessage string

	IncrementAttempt bool
	AttemptCount     *int
	TimeoutAt        *time.Time
	NextRetryAt      *time.Time
}

type RuntimeJobTransitionResult struct {
	FromStatus         string
	ToStatus           string
	Event              RuntimeJobTransitionEvent
	Noop               bool
	NoopReason         string
	Terminal           bool
	BindTerminalCharge bool
}

func ApplyRuntimeJobTransition(job *models.RuntimeJob, input RuntimeJobTransitionInput) (RuntimeJobTransitionResult, error) {
	if job == nil {
		return RuntimeJobTransitionResult{}, fmt.Errorf("runtime job transition requires job")
	}
	if input.Event == "" {
		input.Event = RuntimeJobEventAdminPatch
	}
	if input.Now.IsZero() {
		input.Now = time.Now()
	}
	from := job.Status
	result := RuntimeJobTransitionResult{FromStatus: from, Event: input.Event}

	if isTerminalRuntimeJobStatus(from) && (input.Event == RuntimeJobEventProviderProgress || input.Event == RuntimeJobEventProviderAccepted) {
		result.ToStatus = from
		result.Noop = true
		result.NoopReason = "stale provider event ignored for terminal runtime job"
		result.Terminal = true
		return result, nil
	}

	target, err := runtimeJobTransitionTargetStatus(job, input)
	if err != nil {
		return result, err
	}
	result.ToStatus = target
	if err := validateRuntimeJobEventTransition(input.Event, from, target); err != nil {
		return result, err
	}

	job.Status = target
	applyRuntimeJobTransitionFields(job, input)
	applyRuntimeJobTransitionTimestamps(job, input.Now, target, input.Event)

	result.Terminal = isTerminalRuntimeJobStatus(target)
	result.BindTerminalCharge = result.Terminal && (input.Event == RuntimeJobEventCompleted || input.Event == RuntimeJobEventFailed || input.Event == RuntimeJobEventCanceled || (input.Event == RuntimeJobEventAdminPatch && target != from))
	return result, nil
}

func runtimeJobTransitionTargetStatus(job *models.RuntimeJob, input RuntimeJobTransitionInput) (string, error) {
	switch input.Event {
	case RuntimeJobEventAdminPatch:
		if input.Status != "" {
			return input.Status, nil
		}
		return job.Status, nil
	case RuntimeJobEventDispatchStarted, RuntimeJobEventProviderAccepted, RuntimeJobEventProviderProgress:
		return platformconst.StatusProcessing, nil
	case RuntimeJobEventRetryScheduled, RuntimeJobEventFallbackScheduled:
		return platformconst.StatusQueued, nil
	case RuntimeJobEventCompleted:
		return platformconst.StatusCompleted, nil
	case RuntimeJobEventFailed:
		return platformconst.StatusFailed, nil
	case RuntimeJobEventCanceled:
		return platformconst.StatusCanceled, nil
	case RuntimeJobEventTerminalMetadataPatch:
		return job.Status, nil
	default:
		return "", fmt.Errorf("unsupported runtime job transition event: %s", input.Event)
	}
}

func validateRuntimeJobEventTransition(event RuntimeJobTransitionEvent, from, to string) error {
	if from == to {
		if event == RuntimeJobEventTerminalMetadataPatch && !isTerminalRuntimeJobStatus(from) {
			return fmt.Errorf("runtime job terminal metadata patch requires terminal status, got %q", from)
		}
		return nil
	}
	switch event {
	case RuntimeJobEventAdminPatch:
		return validateRuntimeJobStatusTransition(from, to)
	case RuntimeJobEventDispatchStarted:
		if from == platformconst.StatusQueued && to == platformconst.StatusProcessing {
			return nil
		}
	case RuntimeJobEventProviderAccepted, RuntimeJobEventProviderProgress:
		if (from == platformconst.StatusQueued || from == platformconst.StatusProcessing) && to == platformconst.StatusProcessing {
			return nil
		}
	case RuntimeJobEventRetryScheduled, RuntimeJobEventFallbackScheduled:
		if from == platformconst.StatusProcessing && to == platformconst.StatusQueued {
			return nil
		}
	case RuntimeJobEventCompleted:
		if from == platformconst.StatusProcessing && to == platformconst.StatusCompleted {
			return nil
		}
	case RuntimeJobEventFailed:
		if (from == platformconst.StatusQueued || from == platformconst.StatusProcessing) && to == platformconst.StatusFailed {
			return nil
		}
	case RuntimeJobEventCanceled:
		if (from == platformconst.StatusQueued || from == platformconst.StatusProcessing) && to == platformconst.StatusCanceled {
			return nil
		}
	case RuntimeJobEventTerminalMetadataPatch:
		return fmt.Errorf("runtime job terminal metadata patch cannot change status: %q -> %q", from, to)
	}
	if isTerminalRuntimeJobStatus(from) {
		return fmt.Errorf("runtime job status %q is terminal, cannot transition to %q via %s", from, to, event)
	}
	return fmt.Errorf("invalid runtime job status transition via %s: %q -> %q", event, from, to)
}

func applyRuntimeJobTransitionFields(job *models.RuntimeJob, input RuntimeJobTransitionInput) {
	if input.Stage != "" {
		job.Stage = input.Stage
	}
	if input.StageMessage != "" {
		job.StageMessage = input.StageMessage
	}
	if input.ProviderCode != "" {
		job.ProviderCode = input.ProviderCode
	}
	if input.ProviderJobID != "" {
		job.ProviderJobID = input.ProviderJobID
	}
	if input.RouteSnapshot != "" {
		job.RouteSnapshot = input.RouteSnapshot
	}
	if input.Metadata != "" {
		job.Metadata = input.Metadata
	}
	if input.OutputManifest != "" {
		job.OutputManifest = input.OutputManifest
	}
	if input.ErrorClass != "" {
		job.ErrorClass = input.ErrorClass
	}
	if input.ErrorCode != "" {
		job.ErrorCode = input.ErrorCode
	}
	if input.ErrorMessage != "" {
		job.ErrorMessage = input.ErrorMessage
	}
	if input.IncrementAttempt {
		job.AttemptCount++
	}
	if input.AttemptCount != nil {
		job.AttemptCount = *input.AttemptCount
	}
	if input.TimeoutAt != nil {
		job.TimeoutAt = input.TimeoutAt
	}
	if input.NextRetryAt != nil {
		job.NextRetryAt = input.NextRetryAt
	}
	if input.Event == RuntimeJobEventProviderAccepted || input.Event == RuntimeJobEventFallbackScheduled {
		job.NextRetryAt = nil
	}
}

func applyRuntimeJobTransitionTimestamps(job *models.RuntimeJob, now time.Time, target string, event RuntimeJobTransitionEvent) {
	switch target {
	case platformconst.StatusCompleted:
		if job.CompletedAt == nil && event != RuntimeJobEventTerminalMetadataPatch {
			job.CompletedAt = &now
		}
		if event == RuntimeJobEventCompleted {
			job.CanceledAt = nil
			job.NextRetryAt = nil
		}
	case platformconst.StatusCanceled:
		if job.CanceledAt == nil {
			job.CanceledAt = &now
		}
		job.NextRetryAt = nil
	case platformconst.StatusFailed:
		if job.CompletedAt == nil {
			job.CompletedAt = &now
		}
		job.NextRetryAt = nil
	}
}

func isTerminalRuntimeJobStatus(status string) bool {
	switch status {
	case platformconst.StatusCompleted, platformconst.StatusFailed, platformconst.StatusCanceled:
		return true
	default:
		return false
	}
}

func (s *Service) transitionRuntimeJob(job *models.RuntimeJob, input RuntimeJobTransitionInput) (*models.RuntimeJob, RuntimeJobTransitionResult, error) {
	if job != nil && job.ID != "" {
		var updated *models.RuntimeJob
		var result RuntimeJobTransitionResult
		err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
			var locked models.RuntimeJob
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", job.ID).First(&locked).Error; err != nil {
				result = RuntimeJobTransitionResult{Event: input.Event}
				return err
			}
			if isTerminalRuntimeJobStatus(locked.Status) {
				switch {
				case input.Event == RuntimeJobEventTerminalMetadataPatch:
					// Intentionally patch metadata/stage fields on the locked terminal row.
				case input.Event == RuntimeJobEventProviderProgress || input.Event == RuntimeJobEventProviderAccepted || (input.Event == RuntimeJobEventAdminPatch && (input.Status == "" || input.Status == locked.Status)):
					result = RuntimeJobTransitionResult{
						FromStatus: locked.Status,
						ToStatus:   locked.Status,
						Event:      input.Event,
						Noop:       true,
						NoopReason: "stale or idempotent transition ignored for terminal runtime job in database",
						Terminal:   true,
					}
					updated = &locked
					return nil
				default:
					result = RuntimeJobTransitionResult{
						FromStatus: locked.Status,
						ToStatus:   locked.Status,
						Event:      input.Event,
						Terminal:   true,
					}
					updated = &locked
					return fmt.Errorf("runtime job status %q is terminal, cannot apply %s", locked.Status, input.Event)
				}
			}

			applied, applyErr := ApplyRuntimeJobTransition(&locked, input)
			result = applied
			updated = &locked
			if applyErr != nil {
				return applyErr
			}
			if applied.Noop {
				return nil
			}
			if err := tx.Save(&locked).Error; err != nil {
				return err
			}
			if applied.BindTerminalCharge {
				return s.bindRuntimeTerminalChargeSessionTx(tx, &locked, locked.Status)
			}
			return nil
		})
		if updated == nil {
			updated = job
		}
		if updated != nil && job != nil && updated != job {
			*job = *updated
			updated = job
		}
		return updated, result, err
	}
	result, err := ApplyRuntimeJobTransition(job, input)
	if err != nil {
		return job, result, err
	}
	if result.Noop {
		return job, result, nil
	}
	if result.BindTerminalCharge {
		if err := s.saveRuntimeJobWithTerminalChargeBinding(job, job.Status); err != nil {
			return job, result, err
		}
		return job, result, nil
	}
	if err := s.repo.SaveRuntimeJob(job); err != nil {
		return job, result, err
	}
	return job, result, nil
}
