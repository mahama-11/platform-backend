package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	attemptCount := 2
	updated, err := service.UpdateRuntimeJob(job.ID, UpdateRuntimeJobInput{
		Status:        "completed",
		Stage:         "completed",
		StageMessage:  "done",
		ProviderJobID: "provider-job-2",
		OutputManifest:"{}",
		RouteSnapshot: `{"objective":"quality"}`,
		Metadata:      `{"k":"v"}`,
		AttemptCount:  &attemptCount,
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
	finalUnits := int64(3)
	session, err = service.UpdateChargeSession(session.ID, UpdateChargeSessionInput{
		Status:      "settled",
		FinalUnits:  &finalUnits,
		Metadata:    `{"provider":"comfyui_bridge"}`,
		EventID:     "event-1",
		SettlementID:"settlement-1",
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

func buildProviderCallbackSignature(secret, runtimeJobID string, expiresAt int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(runtimeJobID))
	mac.Write([]byte(":"))
	mac.Write([]byte(strconv.FormatInt(expiresAt, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
