package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"platform-service/internal/models"
	assetstorage "platform-service/internal/modules/assetstorage"
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

func TestCreateRuntimeJobScopesIdempotencyReplayByBoundary(t *testing.T) {
	service, _, queue := newRuntimeServiceForTest(t)

	first, err := service.CreateRuntimeJob(CreateRuntimeJobInput{
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderMode:   "async",
		OrganizationID: "org-1",
		SourceType:     "ecommerce_job",
		SourceID:       "job-1",
		IdempotencyKey: "idem-cross-boundary",
	})
	if err != nil {
		t.Fatalf("CreateRuntimeJob first call: %v", err)
	}

	second, err := service.CreateRuntimeJob(CreateRuntimeJobInput{
		ProductCode:    "menu_ai",
		TaskType:       "image_generation",
		ProviderMode:   "async",
		OrganizationID: "org-2",
		SourceType:     "menu_job",
		SourceID:       "job-2",
		IdempotencyKey: "idem-cross-boundary",
	})
	if err != nil {
		t.Fatalf("same idempotency key in a different boundary must be allowed: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("expected independent job for scoped idempotency, got same id %s", second.ID)
	}
	if len(queue.dispatches) != 2 {
		t.Fatalf("independent scoped idempotency jobs must both enqueue, got %+v", queue.dispatches)
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

func TestHandleProviderCallbackPayloadProgressDoesNotDowngradeCompletedJob(t *testing.T) {
	service, repo, queue := newRuntimeServiceForTest(t)
	completedAt := time.Now().Add(-time.Minute)
	job := &models.RuntimeJob{
		ID:             "runtime-terminal-progress",
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderCode:   "comfyui_bridge",
		ProviderJobID:  "provider-terminal-progress",
		OrganizationID: "org-1",
		Status:         "completed",
		Stage:          "completed",
		StageMessage:   "done",
		OutputManifest: `{"ok":true}`,
		SourceType:     "ecommerce_job",
		SourceID:       "job-terminal-progress",
		CompletedAt:    &completedAt,
	}
	if createErr := repo.CreateRuntimeJob(job); createErr != nil {
		t.Fatalf("CreateRuntimeJob: %v", createErr)
	}
	expiresAt := time.Now().Add(time.Minute).Unix()
	validSig := buildProviderCallbackSignature(runtimeSecurityForTest().EncryptionKey, job.ID, expiresAt)
	if err := service.HandleProviderCallbackPayload("comfyui_bridge", job.ID, expiresAt, validSig, &NormalizedProviderCallbackPayload{
		ProviderCode:  "comfyui_bridge",
		ProviderJobID: "provider-terminal-progress",
		Status:        "running",
		Stage:         "provider_running",
		StageMessage:  "late progress",
		Progress:      50,
	}); err != nil {
		t.Fatalf("HandleProviderCallbackPayload: %v", err)
	}
	updated, err := repo.FindRuntimeJobByID(job.ID)
	if err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
	if updated.Status != "completed" || updated.Stage != "completed" || updated.StageMessage != "done" || updated.OutputManifest != `{"ok":true}` {
		t.Fatalf("late provider progress downgraded/mutated terminal job: %+v", updated)
	}
	if len(queue.callbacks) != 0 {
		t.Fatalf("stale terminal progress must not enqueue product callback, got %+v", queue.callbacks)
	}
}

func TestCreateChargeSessionReusesExistingReservationKeyForSameBoundary(t *testing.T) {
	service, _, _ := newRuntimeServiceForTest(t)

	input := CreateChargeSessionInput{
		SourceType:         "visual_generation",
		SourceID:           "gv-1",
		ProductCode:        "ecommerce",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "ecommerce.image.generate",
		ResourceType:       "quota",
		ReservationKey:     "reservation-key-1",
		EstimatedUnits:     1,
	}
	first, err := service.CreateChargeSession(input)
	if err != nil {
		t.Fatalf("CreateChargeSession first: %v", err)
	}
	again, err := service.CreateChargeSession(input)
	if err != nil {
		t.Fatalf("CreateChargeSession duplicate reservation key should be idempotent: %v", err)
	}
	if again.ID != first.ID || again.ReservationKey != first.ReservationKey {
		t.Fatalf("expected existing charge session reuse, got %+v want %+v", again, first)
	}
}

func TestCreateChargeSessionRejectsDuplicateReservationKeyForDifferentSource(t *testing.T) {
	service, _, _ := newRuntimeServiceForTest(t)

	first, err := service.CreateChargeSession(CreateChargeSessionInput{
		SourceType:         "visual_generation",
		SourceID:           "gv-1",
		ProductCode:        "ecommerce",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "ecommerce.image.generate",
		ResourceType:       "quota",
		ReservationKey:     "reservation-key-1",
		EstimatedUnits:     1,
	})
	if err != nil {
		t.Fatalf("CreateChargeSession first: %v", err)
	}
	again, err := service.CreateChargeSession(CreateChargeSessionInput{
		SourceType:         first.SourceType,
		SourceID:           "gv-2",
		ProductCode:        first.ProductCode,
		OrganizationID:     first.OrganizationID,
		UserID:             first.UserID,
		BillingSubjectType: first.BillingSubjectType,
		BillingSubjectID:   first.BillingSubjectID,
		BillableItemCode:   first.BillableItemCode,
		ResourceType:       first.ResourceType,
		ReservationKey:     first.ReservationKey,
		EstimatedUnits:     1,
	})
	if err == nil || again != nil {
		t.Fatalf("expected duplicate reservation key with different source to fail, got session=%+v err=%v", again, err)
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
		Status:         "completed",
		Stage:          "completed",
		StageMessage:   "done",
		ProviderJobID:  "provider-job-2",
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
		Status:        "reserved",
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
		{"queued", "completed"},     // skip processing
		{"completed", "queued"},     // terminal
		{"completed", "processing"}, // terminal
		{"canceled", "queued"},      // terminal
		{"canceled", "processing"},  // terminal
		{"failed", "completed"},     // must retry via queued first
		{"failed", "processing"},    // must retry via queued first
		{"processing", "queued"},    // backwards
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

func TestSanitizeProviderCallbackMetadataRecursesTypedCollections(t *testing.T) {
	metadata := map[string]any{
		"safe":    "ok",
		"outputs": []map[string]any{{"label": "hero", "api_key": "secret", "nested": map[string]string{"token": "hidden", "caption": "front"}}},
		"billing": map[string]any{"charge_session_id": "cs_1"},
	}
	sanitized := sanitizeProviderCallbackMetadata(metadata)
	body, _ := json.Marshal(sanitized)
	text := string(body)
	if strings.Contains(text, "secret") || strings.Contains(text, "hidden") || strings.Contains(text, "charge_session_id") || strings.Contains(text, "billing") || strings.Contains(text, "api_key") || strings.Contains(text, "token") {
		t.Fatalf("metadata sanitizer leaked sensitive nested fields: %s", text)
	}
	if !strings.Contains(text, "hero") || !strings.Contains(text, "front") || sanitized["safe"] != "ok" {
		t.Fatalf("metadata sanitizer dropped safe nested fields: %s", text)
	}
}

func TestHandleProviderCallbackPayloadNormalizesOutputManifestAndRegistersStorage(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	service.UseAssetStorage(assetstorage.NewService(repo))
	baseDir := t.TempDir()
	if err := repo.CreateStorageBinding(&models.StorageBinding{ID: "storage-runtime-output", ProductCode: "ecommerce", Category: "runtime-assets", ProviderCode: "local", LocalBaseDir: baseDir, Enabled: true, Priority: 1}); err != nil {
		t.Fatalf("CreateStorageBinding: %v", err)
	}
	job := &models.RuntimeJob{ID: "runtime-callback-output", ProductCode: "ecommerce", TaskType: "image_generation", ProviderCode: "comfyui_bridge", ProviderMode: "async", OrganizationID: "org-1", UserID: "user-1", Status: "processing", Stage: "provider_running", SourceType: "visual_generation", SourceID: "version-1", InputManifest: `{"input_mode":"prompt_snapshot"}`}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	expiresAt := time.Now().Add(time.Minute).Unix()
	validSig := buildProviderCallbackSignature(runtimeSecurityForTest().EncryptionKey, job.ID, expiresAt)
	err := service.HandleProviderCallbackPayload("comfyui", job.ID, expiresAt, validSig, &NormalizedProviderCallbackPayload{
		Status:       "completed",
		Progress:     100,
		StageMessage: "done",
		Variants:     []ProviderResultVariant{{Index: 0, InlineData: "iVBORw0KGgo=", MimeType: "image/png", Metadata: map[string]any{"secret": "drop"}}},
		Metadata:     map[string]any{"provider_trace_id": "trace-1", "api_key": "drop"},
	})
	if err != nil {
		t.Fatalf("HandleProviderCallbackPayload: %v", err)
	}
	updated, err := repo.FindRuntimeJobByID(job.ID)
	if err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
	var manifest RuntimeOutputManifest
	if err := json.Unmarshal([]byte(updated.OutputManifest), &manifest); err != nil {
		t.Fatalf("output manifest json: %v raw=%s", err, updated.OutputManifest)
	}
	if manifest.Contract != "platform.runtime.output.v1" || len(manifest.Variants) != 1 || manifest.Variants[0].Asset.StorageKey == "" {
		t.Fatalf("unexpected output manifest: %+v", manifest)
	}
	if _, ok := manifest.ProviderMeta["api_key"]; ok {
		t.Fatalf("provider internals leaked in manifest: %+v", manifest.ProviderMeta)
	}
	if _, err := repo.FindStorageAssetBySource("ecommerce", "runtime-assets", "runtime_output", job.ID+":0"); err != nil {
		t.Fatalf("expected storage registry asset: %v", err)
	}
}

func TestHandleProviderCallbackPayloadNormalizesTextInlineDataForStorage(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	service.UseAssetStorage(assetstorage.NewService(repo))
	baseDir := t.TempDir()
	if err := repo.CreateStorageBinding(&models.StorageBinding{ID: "storage-runtime-text-output", ProductCode: "ecommerce", Category: "runtime-assets", ProviderCode: "local", LocalBaseDir: baseDir, Enabled: true, Priority: 1}); err != nil {
		t.Fatalf("CreateStorageBinding: %v", err)
	}
	job := &models.RuntimeJob{ID: "runtime-callback-text-output", ProductCode: "ecommerce", TaskType: "text_reasoning", ProviderCode: "kimi_coding_text", ProviderMode: "sync", OrganizationID: "org-1", UserID: "user-1", Status: "processing", Stage: "provider_running", SourceType: "regression_smoke", SourceID: "text-1", InputManifest: `{"input_mode":"prompt_snapshot"}`}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	expiresAt := time.Now().Add(time.Minute).Unix()
	validSig := buildProviderCallbackSignature(runtimeSecurityForTest().EncryptionKey, job.ID, expiresAt)
	err := service.HandleProviderCallbackPayload("kimi_coding_text", job.ID, expiresAt, validSig, &NormalizedProviderCallbackPayload{
		Status:       "completed",
		Progress:     100,
		StageMessage: "text done",
		Variants:     []ProviderResultVariant{{Index: 0, InlineData: `{"ok":true}`, MimeType: "application/json", AssetType: "json", Metadata: map[string]any{"provider": "kimi_coding_text"}}},
		Metadata:     map[string]any{"provider": "kimi_coding_text"},
	})
	if err != nil {
		t.Fatalf("HandleProviderCallbackPayload text inline: %v", err)
	}
	updated, err := repo.FindRuntimeJobByID(job.ID)
	if err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
	if updated.Status != "completed" || updated.OutputManifest == "" {
		t.Fatalf("expected completed text runtime with output manifest, got %+v", updated)
	}
	if _, err := repo.FindStorageAssetBySource("ecommerce", "runtime-assets", "runtime_output", job.ID+":0"); err != nil {
		t.Fatalf("expected text runtime storage registry asset: %v", err)
	}
}

func buildProviderCallbackSignature(secret, runtimeJobID string, expiresAt int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(runtimeJobID))
	mac.Write([]byte(":"))
	mac.Write([]byte(strconv.FormatInt(expiresAt, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
