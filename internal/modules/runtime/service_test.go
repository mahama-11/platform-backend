package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"
	"time"

	"platform-service/internal/models"
)

func TestCreateRuntimeJobEnqueuesAndSupportsIdempotency(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)

	job, err := service.CreateRuntimeJob(CreateRuntimeJobInput{
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderMode:   "async",
		OrganizationID: "org-1",
		SourceType:     "ecommerce_job",
		SourceID:       "job-1",
		IdempotencyKey: "idem-1",
		InputManifest:  `{"input_mode":"text_to_image"}`,
	})
	if err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	if len(queue.dispatches) != 1 || queue.dispatches[0].RuntimeJobID != job.ID {
		t.Fatalf("expected dispatch enqueue, got %+v", queue.dispatches)
	}

	again, err := service.CreateRuntimeJob(CreateRuntimeJobInput{
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderMode:   "async",
		OrganizationID: "org-1",
		SourceType:     "ecommerce_job",
		SourceID:       "job-1",
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("CreateRuntimeJob second call: %v", err)
	}
	if again.ID != job.ID {
		t.Fatalf("expected idempotent job reuse, got %s want %s", again.ID, job.ID)
	}
	if _, err := repo.FindRuntimeJobByID(job.ID); err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
}

func TestHandleProviderCallbackValidatesSignatureAndReturnsTerminalJob(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{
		ID:          "runtime-1",
		ProductCode: "ecommerce",
		TaskType:    "image_generation",
		Status:      "completed",
		Stage:       "completed",
		SourceType:  "ecommerce_job",
		SourceID:    "job-1",
	}
	if createErr := repo.CreateRuntimeJob(job); createErr != nil {
		t.Fatalf("CreateRuntimeJob: %v", createErr)
	}
	expiresAt := time.Now().Add(time.Minute).Unix()
	validSig := buildProviderCallbackSignature(runtimeSecurityForTest().EncryptionKey, job.ID, expiresAt)
	if err := service.HandleProviderCallback("ecommerce_internal", job.ID, expiresAt, validSig); err != nil {
		t.Fatalf("HandleProviderCallback valid: %v", err)
	}
	if err := service.HandleProviderCallback("ecommerce_internal", job.ID, expiresAt, "bad-signature"); err == nil {
		t.Fatalf("expected invalid signature failure")
	}
}

func TestUpdateRuntimeJobRecordAttemptAndChargeSession(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{
		ID:          "runtime-2",
		ProductCode: "ecommerce",
		TaskType:    "image_generation",
		Status:      "queued",
		SourceType:  "ecommerce_job",
		SourceID:    "job-2",
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	// 先转移到 processing
	_, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{
		Status:       "processing",
		Stage:        "processing",
		StageMessage: "running",
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeJob to processing: %v", err)
	}
	attemptCount := 2
	updated, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{
		Status:        "completed",
		Stage:         "completed",
		StageMessage:  "done",
		ProviderJobID: "provider-job-2",
		OutputManifest: "{}",
		RouteSnapshot:  `{"objective":"quality"}`,
		Metadata:       `{"k":"v"}`,
		AttemptCount:   &attemptCount,
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeJob: %v", err)
	}
	if updated.CompletedAt == nil || updated.ProviderJobID != "provider-job-2" || updated.AttemptCount != 2 {
		t.Fatalf("unexpected updated runtime job: %+v", updated)
	}
	attempt, err := service.RecordRuntimeAttempt(job.ID, RecordRuntimeAttemptInput{
		Status:       "completed",
		ProviderCode: "comfyui_bridge",
		ProviderMode: "async",
		StartedAt:    time.Now().Add(-time.Minute).Format(time.RFC3339),
		EndedAt:      time.Now().Format(time.RFC3339),
	})
	if err != nil || attempt.AttemptNo != 1 {
		t.Fatalf("RecordRuntimeAttempt: %+v err=%v", attempt, err)
	}
	session, err := service.CreateChargeSession(CreateChargeSessionInput{
		SourceType:         "runtime_job",
		SourceID:           job.ID,
		ProductCode:        "ecommerce",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		ResourceType:       "image_generation",
	})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	// 先转移到 reserved
	session, err = service.UpdateChargeSession(session.ID, UpdateChargeSessionInput{
		Status:       "reserved",
		ReservationID: "reservation-1",
	})
	if err != nil {
		t.Fatalf("UpdateChargeSession to reserved: %v", err)
	}
	finalUnits := int64(3)
	session, err = service.UpdateChargeSession(session.ID, UpdateChargeSessionInput{
		Status:       "settled",
		FinalUnits:   &finalUnits,
		Metadata:     `{"provider":"comfyui_bridge"}`,
		EventID:      "event-1",
		SettlementID: "settlement-1",
	})
	if err != nil {
		t.Fatalf("UpdateChargeSession: %v", err)
	}
	if session.FinalizedAt == nil || session.FinalUnits != 3 {
		t.Fatalf("unexpected charge session: %+v", session)
	}
	if _, err := service.GetChargeSession(session.ID); err != nil {
		t.Fatalf("GetChargeSession: %v", err)
	}
}

func TestProviderDefinitionGetRuntimeJobAndCancel(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	definition, err := service.CreateProviderDefinition(CreateProviderDefinitionInput{
		Code:         "comfyui_bridge",
		Name:         "ComfyUI Bridge",
		ProviderType: "image_generation",
	})
	if err != nil {
		t.Fatalf("CreateProviderDefinition: %v", err)
	}
	definitions, err := service.ListProviderDefinitions()
	if err != nil || len(definitions) != 1 || definitions[0].ID != definition.ID {
		t.Fatalf("ListProviderDefinitions: %+v err=%v", definitions, err)
	}
	job := &models.RuntimeJob{
		ID:          "runtime-3",
		ProductCode: "ecommerce",
		TaskType:    "image_generation",
		Status:      "queued",
		SourceType:  "ecommerce_job",
		SourceID:    "job-3",
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	got, getErr := service.GetRuntimeJob(job.ID)
	if getErr != nil || got.Job == nil || got.Job.ID != job.ID {
		t.Fatalf("GetRuntimeJob: %+v err=%v", got, getErr)
	}
	canceled, err := service.CancelRuntimeJob(job.ID)
	if err != nil || canceled.Status != "canceled" || canceled.CanceledAt == nil {
		t.Fatalf("CancelRuntimeJob: %+v err=%v", canceled, err)
	}
}

// ---------------------------------------------------------------------------
// RuntimeJob State Machine Tests
// ---------------------------------------------------------------------------

func TestRuntimeJobStateMachine_AllValidTransitions(t *testing.T) {
	cases := []struct {
		from string
		to   string
	}{
		{"queued", "processing"},
		{"queued", "canceled"},
		{"queued", "failed"},
		{"processing", "completed"},
		{"processing", "failed"},
		{"processing", "canceled"},
		{"failed", "queued"}, // retry
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%s->%s", tc.from, tc.to), func(t *testing.T) {
			service, repo, _ := newRuntimeServiceForTest(t)
			jobID := fmt.Sprintf("job-valid-%d", i)
			job := &models.RuntimeJob{
				ID:          jobID,
				ProductCode: "ecommerce",
				TaskType:    "image_generation",
				Status:      tc.from,
				SourceType:  "ecommerce_job",
				SourceID:    fmt.Sprintf("src-valid-%d", i),
			}
			if err := repo.CreateRuntimeJob(job); err != nil {
				t.Fatalf("CreateRuntimeJob: %v", err)
			}
			updated, err := service.UpdateRuntimeJob(jobID, UpdateRuntimeJobInput{
				Status: tc.to,
				Stage:  tc.to,
			})
			if err != nil {
				t.Fatalf("expected valid transition %s->%s but got error: %v", tc.from, tc.to, err)
			}
			if updated.Status != tc.to {
				t.Fatalf("expected status %s, got %s", tc.to, updated.Status)
			}
		})
	}
}

func TestRuntimeJobStateMachine_InvalidTransitions(t *testing.T) {
	cases := []struct {
		from string
		to   string
	}{
		{"queued", "completed"},    // skip processing
		{"completed", "queued"},    // terminal
		{"completed", "processing"},// terminal
		{"canceled", "queued"},     // terminal
		{"canceled", "processing"}, // terminal
		{"failed", "completed"},    // must retry via queued first
		{"failed", "processing"},   // must retry via queued first
		{"processing", "queued"},   // backwards
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%s->%s", tc.from, tc.to), func(t *testing.T) {
			service, repo, _ := newRuntimeServiceForTest(t)
			jobID := fmt.Sprintf("job-invalid-%d", i)
			job := &models.RuntimeJob{
				ID:          jobID,
				ProductCode: "ecommerce",
				TaskType:    "image_generation",
				Status:      tc.from,
				SourceType:  "ecommerce_job",
				SourceID:    fmt.Sprintf("src-invalid-%d", i),
			}
			if err := repo.CreateRuntimeJob(job); err != nil {
				t.Fatalf("CreateRuntimeJob: %v", err)
			}
			_, err := service.UpdateRuntimeJob(jobID, UpdateRuntimeJobInput{
				Status: tc.to,
				Stage:  tc.to,
			})
			if err == nil {
				t.Fatalf("expected error for invalid transition %s->%s, got nil", tc.from, tc.to)
			}
		})
	}
}

func TestRuntimeJobStateMachine_SameStatusNoop(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{
		ID:          "job-noop-1",
		ProductCode: "ecommerce",
		TaskType:    "image_generation",
		Status:      "queued",
		SourceType:  "ecommerce_job",
		SourceID:    "src-noop-1",
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	updated, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{
		Status: "queued",
		Stage:  "queued",
	})
	if err != nil {
		t.Fatalf("same-status update should be no-op, got error: %v", err)
	}
	if updated.Status != "queued" {
		t.Fatalf("expected status queued, got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// ChargeSession State Machine Tests
// ---------------------------------------------------------------------------

func createTestChargeSession(t *testing.T, service *Service, suffix string) *models.ChargeSession {
	t.Helper()
	session, err := service.CreateChargeSession(CreateChargeSessionInput{
		SourceType:         "runtime_job",
		SourceID:           fmt.Sprintf("job-%s", suffix),
		ProductCode:        "ecommerce",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		ResourceType:       "image_generation",
	})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	return session
}

func transitionChargeSessionTo(t *testing.T, service *Service, sessionID, target string) {
	t.Helper()
	switch target {
	case "created":
		// already created
	case "reserved":
		if _, err := service.UpdateChargeSession(sessionID, UpdateChargeSessionInput{
			Status:        "reserved",
			ReservationID: "res-" + sessionID,
		}); err != nil {
			t.Fatalf("transition to reserved: %v", err)
		}
	case "settled":
		transitionChargeSessionTo(t, service, sessionID, "reserved")
		units := int64(1)
		if _, err := service.UpdateChargeSession(sessionID, UpdateChargeSessionInput{
			Status:       "settled",
			FinalUnits:   &units,
			SettlementID: "settle-" + sessionID,
		}); err != nil {
			t.Fatalf("transition to settled: %v", err)
		}
	case "released":
		transitionChargeSessionTo(t, service, sessionID, "reserved")
		if _, err := service.UpdateChargeSession(sessionID, UpdateChargeSessionInput{
			Status: "released",
		}); err != nil {
			t.Fatalf("transition to released: %v", err)
		}
	case "canceled":
		if _, err := service.UpdateChargeSession(sessionID, UpdateChargeSessionInput{
			Status: "canceled",
		}); err != nil {
			t.Fatalf("transition to canceled: %v", err)
		}
	case "failed":
		if _, err := service.UpdateChargeSession(sessionID, UpdateChargeSessionInput{
			Status: "failed",
		}); err != nil {
			t.Fatalf("transition to failed (from created): %v", err)
		}
	default:
		t.Fatalf("unsupported target status: %s", target)
	}
}

func TestChargeSessionStateMachine_AllValidTransitions(t *testing.T) {
	cases := []struct {
		fromSetup string // status to set up before testing the transition
		to        string
	}{
		{"created", "reserved"},
		{"created", "canceled"},
		{"created", "failed"},
		{"reserved", "settled"},
		{"reserved", "released"},
		{"reserved", "failed"},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%s->%s", tc.fromSetup, tc.to), func(t *testing.T) {
			service, _, _ := newRuntimeServiceForTest(t)
			session := createTestChargeSession(t, service, fmt.Sprintf("cs-valid-%d", i))
			// Transition to the "from" state via valid path
			transitionChargeSessionTo(t, service, session.ID, tc.fromSetup)

			// Now attempt the transition under test
			input := UpdateChargeSessionInput{Status: tc.to}
			if tc.to == "reserved" {
				input.ReservationID = fmt.Sprintf("res-test-%d", i)
			}
			if tc.to == "settled" {
				units := int64(1)
				input.FinalUnits = &units
				input.SettlementID = fmt.Sprintf("settle-test-%d", i)
			}
			updated, err := service.UpdateChargeSession(session.ID, input)
			if err != nil {
				t.Fatalf("expected valid transition %s->%s but got error: %v", tc.fromSetup, tc.to, err)
			}
			if updated.Status != tc.to {
				t.Fatalf("expected status %s, got %s", tc.to, updated.Status)
			}
		})
	}
}

func TestChargeSessionStateMachine_InvalidTransitions(t *testing.T) {
	cases := []struct {
		fromSetup string
		to        string
	}{
		{"created", "settled"},   // skip reserved
		{"reserved", "created"},  // backwards
		{"settled", "reserved"},  // terminal
		{"settled", "released"},  // terminal
		{"released", "settled"},  // terminal
		{"released", "reserved"}, // terminal
		{"canceled", "created"},  // terminal
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%s->%s", tc.fromSetup, tc.to), func(t *testing.T) {
			service, _, _ := newRuntimeServiceForTest(t)
			session := createTestChargeSession(t, service, fmt.Sprintf("cs-invalid-%d", i))
			transitionChargeSessionTo(t, service, session.ID, tc.fromSetup)

			input := UpdateChargeSessionInput{Status: tc.to}
			if tc.to == "reserved" {
				input.ReservationID = fmt.Sprintf("res-inv-%d", i)
			}
			if tc.to == "settled" {
				units := int64(1)
				input.FinalUnits = &units
				input.SettlementID = fmt.Sprintf("settle-inv-%d", i)
			}
			_, err := service.UpdateChargeSession(session.ID, input)
			if err == nil {
				t.Fatalf("expected error for invalid transition %s->%s, got nil", tc.fromSetup, tc.to)
			}
		})
	}
}

func TestChargeSessionStateMachine_SameStatusNoop(t *testing.T) {
	service, _, _ := newRuntimeServiceForTest(t)
	session := createTestChargeSession(t, service, "cs-noop-1")

	updated, err := service.UpdateChargeSession(session.ID, UpdateChargeSessionInput{
		Status: "created",
	})
	if err != nil {
		t.Fatalf("same-status update should be no-op, got error: %v", err)
	}
	if updated.Status != "created" {
		t.Fatalf("expected status created, got %s", updated.Status)
	}
}

func buildProviderCallbackSignature(secret, runtimeJobID string, expiresAt int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(runtimeJobID))
	mac.Write([]byte(":"))
	mac.Write([]byte(strconv.FormatInt(expiresAt, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
