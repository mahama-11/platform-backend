package runtime

import (
	"fmt"
	"strings"
	"testing"

	"platform-service/internal/models"
)

// ---------------------------------------------------------------------------
// ChargeSession State Machine Tests
// ---------------------------------------------------------------------------

func TestRuntimeTerminalChargeBindingCompletedSettlesReservedSessionIdempotently(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{
		ID:             "runtime-charge-complete",
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		Status:         "processing",
		SourceType:     "ecommerce_job",
		SourceID:       "job-charge-complete",
		OrganizationID: "org-1",
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	session, err := service.CreateChargeSession(CreateChargeSessionInput{SourceType: job.SourceType, SourceID: job.SourceID, ProductCode: job.ProductCode, OrganizationID: job.OrganizationID, BillingSubjectType: "organization", BillingSubjectID: job.OrganizationID, BillableItemCode: "IMAGE_GENERATION", ResourceType: "image_generation"})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	if _, err := service.UpdateChargeSession(session.ID, UpdateChargeSessionInput{Status: "reserved", ReservationID: "reservation-complete"}); err != nil {
		t.Fatalf("reserve charge session: %v", err)
	}
	job.ChargeSessionID = session.ID
	if err := repo.SaveRuntimeJob(job); err != nil {
		t.Fatalf("attach charge session: %v", err)
	}

	updated, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{Status: "completed", Stage: "completed", StageMessage: "done"})
	if err != nil {
		t.Fatalf("complete runtime job: %v", err)
	}
	if updated.CompletedAt == nil {
		t.Fatalf("expected completed_at set: %+v", updated)
	}
	charged, err := service.GetChargeSession(session.ID)
	if err != nil {
		t.Fatalf("load charge session: %v", err)
	}
	if charged.Status != "settled" || charged.FinalizedAt == nil || charged.FinalUnits != charged.EstimatedUnits || charged.SettlementID == "" {
		t.Fatalf("expected settled charge session, got %+v", charged)
	}
	firstSettlementID := charged.SettlementID

	if _, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{Status: "completed", Stage: "completed"}); err != nil {
		t.Fatalf("repeat complete runtime job should be idempotent: %v", err)
	}
	chargedAgain, err := service.GetChargeSession(session.ID)
	if err != nil {
		t.Fatalf("reload charge session: %v", err)
	}
	if chargedAgain.Status != "settled" || chargedAgain.SettlementID != firstSettlementID {
		t.Fatalf("expected idempotent settled charge session, got %+v", chargedAgain)
	}
}

func TestRuntimeTerminalChargeBindingFailedReleasesReservedSession(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{ID: "runtime-charge-fail", ProductCode: "ecommerce", TaskType: "image_generation", Status: "processing", SourceType: "ecommerce_job", SourceID: "job-charge-fail", OrganizationID: "org-1"}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	session, err := service.CreateChargeSession(CreateChargeSessionInput{SourceType: job.SourceType, SourceID: job.SourceID, ProductCode: job.ProductCode, OrganizationID: job.OrganizationID, BillingSubjectType: "organization", BillingSubjectID: job.OrganizationID, BillableItemCode: "IMAGE_GENERATION", ResourceType: "image_generation"})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	if _, err := service.UpdateChargeSession(session.ID, UpdateChargeSessionInput{Status: "reserved", ReservationID: "reservation-fail"}); err != nil {
		t.Fatalf("reserve charge session: %v", err)
	}
	job.ChargeSessionID = session.ID
	if err := repo.SaveRuntimeJob(job); err != nil {
		t.Fatalf("attach charge session: %v", err)
	}
	if _, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{Status: "failed", Stage: "failed", StageMessage: "provider failed"}); err != nil {
		t.Fatalf("fail runtime job: %v", err)
	}
	charged, err := service.GetChargeSession(session.ID)
	if err != nil {
		t.Fatalf("load charge session: %v", err)
	}
	if charged.Status != "released" || charged.ReleasedAt == nil {
		t.Fatalf("expected released charge session, got %+v", charged)
	}
}

func TestRuntimeTerminalChargeBindingCanceledCancelsCreatedSession(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{ID: "runtime-charge-cancel", ProductCode: "ecommerce", TaskType: "image_generation", Status: "queued", SourceType: "ecommerce_job", SourceID: "job-charge-cancel", OrganizationID: "org-1"}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	session, err := service.CreateChargeSession(CreateChargeSessionInput{SourceType: job.SourceType, SourceID: job.SourceID, ProductCode: job.ProductCode, OrganizationID: job.OrganizationID, BillingSubjectType: "organization", BillingSubjectID: job.OrganizationID, BillableItemCode: "IMAGE_GENERATION", ResourceType: "image_generation"})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	job.ChargeSessionID = session.ID
	if err := repo.SaveRuntimeJob(job); err != nil {
		t.Fatalf("attach charge session: %v", err)
	}
	if _, err := service.CancelRuntimeJob(job.ID); err != nil {
		t.Fatalf("cancel runtime job: %v", err)
	}
	charged, err := service.GetChargeSession(session.ID)
	if err != nil {
		t.Fatalf("load charge session: %v", err)
	}
	if charged.Status != "canceled" {
		t.Fatalf("expected canceled charge session, got %+v", charged)
	}
}

func TestRuntimeTerminalChargeBindingRejectsBoundaryMismatchAndRollsBackJob(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{ID: "runtime-charge-mismatch", ProductCode: "ecommerce", TaskType: "image_generation", Status: "processing", SourceType: "ecommerce_job", SourceID: "job-charge-mismatch", OrganizationID: "org-1"}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	session, err := service.CreateChargeSession(CreateChargeSessionInput{
		SourceType:         "runtime_job",
		SourceID:           job.ID,
		ProductCode:        "other-product",
		OrganizationID:     "org-2",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-2",
		BillableItemCode:   "IMAGE_GENERATION",
		ResourceType:       "image_generation",
	})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	job.ChargeSessionID = session.ID
	if err := repo.SaveRuntimeJob(job); err != nil {
		t.Fatalf("attach charge session: %v", err)
	}
	if _, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{Status: "completed", Stage: "completed"}); err == nil {
		t.Fatalf("expected boundary mismatch error")
	}
	reloaded, err := repo.FindRuntimeJobByID(job.ID)
	if err != nil {
		t.Fatalf("reload runtime job: %v", err)
	}
	if reloaded.Status != "processing" || reloaded.CompletedAt != nil {
		t.Fatalf("expected terminal job update rollback after charge mismatch, got %+v", reloaded)
	}
	charged, err := service.GetChargeSession(session.ID)
	if err != nil {
		t.Fatalf("reload charge session: %v", err)
	}
	if charged.Status != "created" {
		t.Fatalf("expected charge session unchanged, got %+v", charged)
	}
}

func TestRuntimeTerminalChargeBindingRejectsSourceTypeMismatch(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{ID: "runtime-charge-source-type", ProductCode: "ecommerce", TaskType: "image_generation", Status: "processing", SourceType: "ecommerce_job", SourceID: "shared-source-id", OrganizationID: "org-1"}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	session, err := service.CreateChargeSession(CreateChargeSessionInput{
		SourceType:         "other_source_namespace",
		SourceID:           job.SourceID,
		ProductCode:        job.ProductCode,
		OrganizationID:     job.OrganizationID,
		BillingSubjectType: "organization",
		BillingSubjectID:   job.OrganizationID,
		BillableItemCode:   "IMAGE_GENERATION",
		ResourceType:       "image_generation",
	})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	job.ChargeSessionID = session.ID
	if err := repo.SaveRuntimeJob(job); err != nil {
		t.Fatalf("attach charge session: %v", err)
	}
	if _, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{Status: "completed", Stage: "completed"}); err == nil {
		t.Fatalf("expected source type mismatch error")
	}
	reloaded, err := repo.FindRuntimeJobByID(job.ID)
	if err != nil {
		t.Fatalf("reload runtime job: %v", err)
	}
	if reloaded.Status != "processing" {
		t.Fatalf("expected rollback to processing, got %+v", reloaded)
	}
	charged, err := service.GetChargeSession(session.ID)
	if err != nil {
		t.Fatalf("reload charge session: %v", err)
	}
	if charged.Status != "created" {
		t.Fatalf("expected charge session unchanged, got %+v", charged)
	}
}

func TestRuntimeTerminalChargeBindingNoChargeSessionNoop(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{ID: "runtime-charge-none", ProductCode: "ecommerce", TaskType: "image_generation", Status: "processing", SourceType: "ecommerce_job", SourceID: "job-charge-none", OrganizationID: "org-1"}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	if _, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{Status: "completed", Stage: "completed"}); err != nil {
		t.Fatalf("complete runtime job without charge session should not error: %v", err)
	}
}

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

func TestRuntimeChargeSessionBoundaryAndHelperBranches(t *testing.T) {
	job := &models.RuntimeJob{ID: "rt-charge-helper", ProductCode: "ecommerce", OrganizationID: "org-1", UserID: "user-1", SourceType: "ecommerce_job", SourceID: "source-1", ProviderCode: "provider-a", ProviderJobID: "provider-job-a", TaskType: "image_generation"}
	session := &models.ChargeSession{ID: "charge-helper", ProductCode: "ecommerce", OrganizationID: "org-1", UserID: "user-1", SourceType: "ecommerce_job", SourceID: "source-1", BillingSubjectType: "organization", BillingSubjectID: "org-1", BillableItemCode: "IMAGE_GENERATION", EstimatedUnits: 7}
	if err := validateRuntimeChargeSessionBoundary(job, session); err != nil {
		t.Fatalf("expected matching boundary: %v", err)
	}
	for _, tc := range []struct {
		name string
		mut  func(*models.ChargeSession)
	}{
		{"product", func(s *models.ChargeSession) { s.ProductCode = "menu" }},
		{"organization", func(s *models.ChargeSession) { s.OrganizationID = "org-2" }},
		{"user", func(s *models.ChargeSession) { s.UserID = "user-2" }},
		{"source_type", func(s *models.ChargeSession) { s.SourceType = "other" }},
		{"source_id", func(s *models.ChargeSession) { s.SourceID = "other" }},
		{"runtime_job_source", func(s *models.ChargeSession) { s.SourceType = "runtime_job"; s.SourceID = "other-job" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copySession := *session
			tc.mut(&copySession)
			if err := validateRuntimeChargeSessionBoundary(job, &copySession); err == nil {
				t.Fatalf("expected boundary mismatch for %s", tc.name)
			}
		})
	}
	if err := validateRuntimeChargeSessionBoundary(nil, session); err == nil {
		t.Fatalf("expected nil boundary error")
	}
	reservation := &models.ResourceReservation{BillingSubjectType: "organization", BillingSubjectID: "org-1", BillableItemCode: "IMAGE_GENERATION", ReferenceID: "source-1"}
	if !reservationMatchesChargeSession(reservation, session) || reservationMatchesChargeSession(nil, session) || reservationMatchesChargeSession(reservation, nil) {
		t.Fatalf("unexpected reservation boundary match")
	}
	for _, mutate := range []func(*models.ResourceReservation){
		func(r *models.ResourceReservation) { r.BillingSubjectType = "user" },
		func(r *models.ResourceReservation) { r.BillingSubjectID = "org-2" },
		func(r *models.ResourceReservation) { r.BillableItemCode = "OTHER" },
		func(r *models.ResourceReservation) { r.ReferenceID = "other-source" },
	} {
		copyReservation := *reservation
		mutate(&copyReservation)
		if reservationMatchesChargeSession(&copyReservation, session) {
			t.Fatalf("expected reservation boundary mismatch: %+v", copyReservation)
		}
	}
	if !isTerminalChargeSessionStatus("settled") || !isTerminalChargeSessionStatus("released") || !isTerminalChargeSessionStatus("failed") || isTerminalChargeSessionStatus("reserved") {
		t.Fatalf("unexpected terminal charge session classification")
	}
	if finalUnitsForRuntimeJob(job, session) != 7 || finalUnitsForRuntimeJob(job, &models.ChargeSession{FinalUnits: 3, EstimatedUnits: 7}) != 3 || finalUnitsForRuntimeJob(job, &models.ChargeSession{}) != 1 {
		t.Fatalf("unexpected final units helper")
	}
	metadata := mergeRuntimeChargeMetadata(`{"existing":true}`, job, "completed")
	if !strings.Contains(metadata, "rt-charge-helper") || !strings.Contains(metadata, "provider-job-a") || !strings.Contains(metadata, "completed") {
		t.Fatalf("metadata missing runtime fields: %s", metadata)
	}
	if defaultInt64(0, 5) != 5 || defaultInt64(9, 5) != 9 {
		t.Fatalf("unexpected defaultInt64")
	}
	if err := validateChargeSessionStatusTransition("created", "reserved"); err != nil {
		t.Fatalf("created -> reserved should be allowed: %v", err)
	}
	if err := validateChargeSessionStatusTransition("settled", "reserved"); err == nil {
		t.Fatalf("terminal status should not transition")
	}
	if err := validateChargeSessionStatusTransition("created", "settled"); err == nil {
		t.Fatalf("created -> settled should be rejected")
	}
}

func TestRuntimeBindChargeSessionToTerminalJobSettlesLateBindingWithCallerAnchors(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	if err := repo.DB().AutoMigrate(&models.ResourceReservation{}, &models.QuotaLedger{}); err != nil {
		t.Fatalf("migrate reservations: %v", err)
	}
	job := &models.RuntimeJob{
		ID:             "rt-late-bind-completed",
		ProductCode:    "novel_video",
		TaskType:       "video_text_to_video",
		Status:         "completed",
		Stage:          "completed",
		SourceType:     "novel_video_job",
		SourceID:       "video-late-bind",
		OrganizationID: "org-late-bind",
		UserID:         "user-late-bind",
		ProviderCode:   "pai_video",
		ProviderJobID:  "pai-late-bind",
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	session, err := service.CreateChargeSession(CreateChargeSessionInput{SourceType: job.SourceType, SourceID: job.SourceID, ProductCode: job.ProductCode, OrganizationID: job.OrganizationID, UserID: job.UserID, BillingSubjectType: "organization", BillingSubjectID: job.OrganizationID, BillableItemCode: "novel_video_generation", ResourceType: "quota", EstimatedUnits: 5, ReservationKey: "late-bind-key"})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	if err := repo.DB().Create(&models.ResourceReservation{ID: "reservation-late-bind", BillingSubjectType: "organization", BillingSubjectID: job.OrganizationID, BillableItemCode: "novel_video_generation", ResourceType: "quota", Units: 5, Status: "reserved", ReferenceID: session.ID}).Error; err != nil {
		t.Fatalf("seed reservation: %v", err)
	}
	reserved, err := service.UpdateChargeSession(session.ID, UpdateChargeSessionInput{Status: "reserved", ReservationID: "reservation-late-bind"})
	if err != nil {
		t.Fatalf("reserve charge session: %v", err)
	}
	finalUnits := int64(2)
	boundJob, boundSession, err := service.BindChargeSessionToRuntimeJobWithChargeUpdate(reserved.ID, job.ID, UpdateChargeSessionInput{
		FinalUnits:     &finalUnits,
		FinalizationID: "custom-finalization",
		EventID:        "custom-event",
		SettlementID:   "custom-settlement",
		Metadata:       `{"gateway_version":"v2"}`,
	})
	if err != nil {
		t.Fatalf("BindChargeSessionToRuntimeJobWithChargeUpdate: %v", err)
	}
	if boundJob.ChargeSessionID != reserved.ID {
		t.Fatalf("expected job binding, got %+v", boundJob)
	}
	if boundSession.Status != "settled" || boundSession.FinalUnits != 2 || boundSession.FinalizationID != "custom-finalization" || boundSession.EventID != "custom-event" || boundSession.SettlementID != "custom-settlement" {
		t.Fatalf("expected late bind to settle with caller anchors, got %+v", boundSession)
	}
	var reservation models.ResourceReservation
	if err := repo.DB().First(&reservation, "id = ?", "reservation-late-bind").Error; err != nil {
		t.Fatalf("reload reservation: %v", err)
	}
	if reservation.Status != "committed" || reservation.FinalizationID == nil || *reservation.FinalizationID != "custom-finalization" {
		t.Fatalf("expected reservation committed with custom finalization, got %+v", reservation)
	}
	var consumeCount int64
	if err := repo.DB().Model(&models.QuotaLedger{}).Where("reference_id = ? AND units = ?", session.ID, int64(2)).Count(&consumeCount).Error; err != nil {
		t.Fatalf("count quota consume ledger: %v", err)
	}
	if consumeCount != 1 {
		t.Fatalf("expected one quota consume ledger with final units, got %d", consumeCount)
	}
	if _, err := service.GetChargeSessionByReservationKey("late-bind-key"); err != nil {
		t.Fatalf("GetChargeSessionByReservationKey: %v", err)
	}
	if !hasChargeSessionUpdateFields(UpdateChargeSessionInput{EventID: "event"}) || hasChargeSessionUpdateFields(UpdateChargeSessionInput{}) {
		t.Fatalf("unexpected hasChargeSessionUpdateFields result")
	}
}

func TestRuntimeChargeSessionListClampAndTerminalBindingBranches(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{ID: "rt-terminal-branches", ProductCode: "ecommerce", TaskType: "image_generation", OrganizationID: "org-terminal", UserID: "user-terminal", SourceType: "ecommerce_job", SourceID: "source-terminal", Status: "processing", ProviderCode: "provider-a", ProviderJobID: "provider-job-a"}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	created, err := service.CreateChargeSession(CreateChargeSessionInput{SourceType: job.SourceType, SourceID: job.SourceID, ProductCode: job.ProductCode, OrganizationID: job.OrganizationID, UserID: job.UserID, BillingSubjectType: "organization", BillingSubjectID: job.OrganizationID, BillableItemCode: "IMAGE_GENERATION", ResourceType: "quota", EstimatedUnits: 2, ReservationKey: "terminal-branches"})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	job.ChargeSessionID = created.ID
	job.Status = "completed"
	if err := service.bindRuntimeTerminalChargeSessionTx(repo.DB(), job, "completed"); err != nil {
		t.Fatalf("bind completed charge session: %v", err)
	}
	settled, err := service.GetChargeSession(created.ID)
	if err != nil || settled.Status != "settled" || settled.FinalUnits != 2 || settled.FinalizationID == "" || settled.SettlementID == "" {
		t.Fatalf("completed charge session mismatch: %+v err=%v", settled, err)
	}
	if err := service.bindRuntimeTerminalChargeSessionTx(repo.DB(), job, "completed"); err != nil {
		t.Fatalf("terminal settled binding should noop: %v", err)
	}
	failed, err := service.CreateChargeSession(CreateChargeSessionInput{SourceType: job.SourceType, SourceID: "failed-source", ProductCode: job.ProductCode, OrganizationID: job.OrganizationID, UserID: job.UserID, BillingSubjectType: "organization", BillingSubjectID: job.OrganizationID, BillableItemCode: "IMAGE_GENERATION", ResourceType: "quota", ReservationKey: "failed-branches"})
	if err != nil {
		t.Fatalf("CreateChargeSession failed branch: %v", err)
	}
	jobFailed := *job
	jobFailed.ID = "rt-terminal-failed"
	jobFailed.SourceID = "failed-source"
	jobFailed.ChargeSessionID = failed.ID
	if err := service.bindRuntimeTerminalChargeSessionTx(repo.DB(), &jobFailed, "failed"); err != nil {
		t.Fatalf("bind failed charge session: %v", err)
	}
	failed, _ = service.GetChargeSession(failed.ID)
	if failed.Status != "failed" || !strings.Contains(failed.Metadata, "rt-terminal-failed") {
		t.Fatalf("failed charge session mismatch: %+v", failed)
	}
	reserved, err := service.CreateChargeSession(CreateChargeSessionInput{SourceType: job.SourceType, SourceID: "cancel-source", ProductCode: job.ProductCode, OrganizationID: job.OrganizationID, UserID: job.UserID, BillingSubjectType: "organization", BillingSubjectID: job.OrganizationID, BillableItemCode: "IMAGE_GENERATION", ResourceType: "quota", ReservationKey: "cancel-branches"})
	if err != nil {
		t.Fatalf("CreateChargeSession reserved branch: %v", err)
	}
	reserved, err = service.UpdateChargeSession(reserved.ID, UpdateChargeSessionInput{Status: "reserved", ReservationID: "cancel-reservation"})
	if err != nil {
		t.Fatalf("reserve session: %v", err)
	}
	jobCanceled := *job
	jobCanceled.ID = "rt-terminal-canceled"
	jobCanceled.SourceID = "cancel-source"
	jobCanceled.ChargeSessionID = reserved.ID
	if err := service.bindRuntimeTerminalChargeSessionTx(repo.DB(), &jobCanceled, "canceled"); err != nil {
		t.Fatalf("bind canceled charge session: %v", err)
	}
	reserved, _ = service.GetChargeSession(reserved.ID)
	if reserved.Status != "released" || reserved.ReleasedAt == nil {
		t.Fatalf("reserved cancel should release: %+v", reserved)
	}
	list, err := service.ListChargeSessions(ListChargeSessionsInput{OrganizationID: job.OrganizationID, Limit: 1000, Offset: -20, Query: "terminal"})
	if err != nil || list.Limit != 100 || list.Offset != 0 || list.Total == 0 {
		t.Fatalf("ListChargeSessions clamp mismatch: %+v err=%v", list, err)
	}
	if err := service.bindRuntimeTerminalChargeSessionTx(repo.DB(), nil, "completed"); err != nil {
		t.Fatalf("nil job should noop: %v", err)
	}
	jobNoCharge := *job
	jobNoCharge.ChargeSessionID = ""
	if err := service.bindRuntimeTerminalChargeSessionTx(repo.DB(), &jobNoCharge, "completed"); err != nil {
		t.Fatalf("empty charge session should noop: %v", err)
	}
	jobMismatch := *job
	jobMismatch.ChargeSessionID = created.ID
	jobMismatch.ProductCode = "menu"
	if err := service.bindRuntimeTerminalChargeSessionTx(repo.DB(), &jobMismatch, "failed"); err == nil {
		t.Fatalf("expected product boundary mismatch")
	}
}
