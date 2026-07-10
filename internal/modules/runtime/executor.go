package runtime

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
)

type JobQueue interface {
	EnqueueDispatch(runtimeJobID string, delay time.Duration) error
	EnqueuePoll(runtimeJobID string, delay time.Duration) error
	EnqueueCallback(deliveryID string, delay time.Duration) error
}

func defaultRuntimeConfig(cfg config.RuntimeConfig) config.RuntimeConfig {
	if cfg.WorkerConcurrency <= 0 {
		cfg.WorkerConcurrency = 8
	}
	if cfg.QueueName == "" {
		cfg.QueueName = "runtime:default"
	}
	if cfg.ExecutionTimeout <= 0 {
		cfg.ExecutionTimeout = 5 * time.Minute
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 15 * time.Second
	}
	if cfg.PollInitialBackoff <= 0 {
		cfg.PollInitialBackoff = 2 * time.Second
	}
	if cfg.PollBackoff <= 0 {
		cfg.PollBackoff = 5 * time.Second
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = 5 * time.Minute
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	return cfg
}

func (s *Service) HandleDispatchTask(ctx context.Context, runtimeJobID string) error {
	job, err := s.repo.FindRuntimeJobByID(runtimeJobID)
	if err != nil {
		return err
	}
	return s.dispatchRuntimeJobWithContext(ctx, job, time.Now())
}

func (s *Service) HandlePollTask(ctx context.Context, runtimeJobID string) error {
	job, err := s.repo.FindRuntimeJobByID(runtimeJobID)
	if err != nil {
		return err
	}
	return s.pollRuntimeJobWithContext(ctx, job, time.Now())
}

func (s *Service) HandleCallbackTask(ctx context.Context, deliveryID string) error {
	delivery, err := s.repo.FindCallbackDeliveryByID(deliveryID)
	if err != nil {
		return err
	}
	if delivery.Status == "delivered" || delivery.Status == "dead_letter" {
		return nil
	}
	job, err := s.repo.FindRuntimeJobByID(delivery.RuntimeJobID)
	if err != nil {
		return err
	}
	callbackClient := s.callbackClientForJob(job)
	now := time.Now()
	delivery.AttemptCount++
	delivery.LastAttemptAt = &now
	if callbackClient == nil {
		delivery.Status = "dead_letter"
		delivery.LastError = "product callback client is not configured"
		delivery.NextAttemptAt = nil
		return s.repo.SaveCallbackDelivery(delivery)
	}
	var deliverErr error
	switch delivery.CallbackType {
	case "runtime_update":
		var input ProductUpdateRuntimeInput
		if err := json.Unmarshal([]byte(delivery.PayloadJSON), &input); err != nil {
			deliverErr = err
		} else {
			deliverErr = callbackClient.UpdateJobRuntime(ctx, delivery.SourceID, input)
		}
	case "result":
		var input ProductRecordResultsInput
		if err := json.Unmarshal([]byte(delivery.PayloadJSON), &input); err != nil {
			deliverErr = err
		} else {
			deliverErr = callbackClient.RecordJobResults(ctx, delivery.SourceID, input)
		}
	default:
		deliverErr = fmt.Errorf("unsupported callback type: %s", delivery.CallbackType)
	}
	if deliverErr == nil {
		delivery.Status = "delivered"
		delivery.LastError = ""
		delivery.DeliveredAt = &now
		delivery.NextAttemptAt = nil
		return s.repo.SaveCallbackDelivery(delivery)
	}
	delivery.LastError = deliverErr.Error()
	if delivery.AttemptCount >= max(delivery.MaxAttempts, 1) {
		delivery.Status = "dead_letter"
		delivery.NextAttemptAt = nil
		return s.repo.SaveCallbackDelivery(delivery)
	}
	nextAttempt := now.Add(s.cfg.RetryBackoff)
	delivery.Status = "retrying"
	delivery.NextAttemptAt = &nextAttempt
	if err := s.repo.SaveCallbackDelivery(delivery); err != nil {
		return err
	}
	if s.queue != nil {
		return s.queue.EnqueueCallback(delivery.ID, s.cfg.RetryBackoff)
	}
	return nil
}

func (s *Service) dispatchRuntimeJob(job *models.RuntimeJob, now time.Time) error {
	return s.dispatchRuntimeJobWithContext(context.Background(), job, now)
}

func (s *Service) dispatchRuntimeJobWithContext(ctx context.Context, job *models.RuntimeJob, now time.Time) error {
	if job.Status != "queued" {
		return nil
	}
	if job.TimeoutAt != nil && !now.Before(*job.TimeoutAt) {
		return s.failRuntimeJob(job, "provider_timeout", "PROVIDER_TIMEOUT", "runtime job timed out before provider dispatch", now)
	}
	if s.registry == nil {
		return nil
	}
	providerCode, err := s.resolveProviderCode(job)
	if err != nil {
		return s.failRuntimeJob(job, "provider_binding_not_found", "PROVIDER_BINDING_NOT_FOUND", err.Error(), now)
	}
	provider, err := s.registry.Get(providerCode)
	if err != nil {
		return s.failRuntimeJob(job, "provider_not_found", "PROVIDER_NOT_FOUND", err.Error(), now)
	}
	input, err := decodeRuntimeInputManifest(job.InputManifest)
	if err != nil {
		return s.failRuntimeJob(job, "input_manifest_invalid", "INPUT_MANIFEST_INVALID", err.Error(), now)
	}
	hydrateErr := s.hydrateRuntimeSourceAssets(&input)
	if hydrateErr != nil {
		return s.failRuntimeJob(job, "source_asset_invalid", "SOURCE_ASSET_INVALID", hydrateErr.Error(), now)
	}
	timeoutAt := job.TimeoutAt
	if timeoutAt == nil {
		defaultTimeoutAt := now.Add(s.cfg.ExecutionTimeout)
		timeoutAt = &defaultTimeoutAt
	}
	if _, _, saveErr := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
		Event:            RuntimeJobEventDispatchStarted,
		Now:              now,
		ProviderCode:     providerCode,
		Stage:            "dispatching",
		StageMessage:     "Dispatching to provider",
		IncrementAttempt: true,
		TimeoutAt:        timeoutAt,
	}); saveErr != nil {
		return saveErr
	}
	s.runtimeJobLogger(job).Info("runtime.dispatch.started")
	s.notifyProductRuntimeUpdate(job, ProductUpdateRuntimeInput{
		Status:        platformconst.StatusProcessing,
		Stage:         "dispatching",
		StageMessage:  "Dispatching to provider",
		ProviderJobID: job.ProviderJobID,
	})
	submission, err := provider.Submit(ctx, ProviderJobRequest{
		RuntimeJobID:   job.ID,
		TaskType:       job.TaskType,
		ProductCode:    job.ProductCode,
		OrganizationID: job.OrganizationID,
		UserID:         job.UserID,
		Provider:       providerCode,
		CallbackURL:    s.providerCallbackURL(job.ID),
		Input:          input,
		Metadata:       decodeJSONMap(job.Metadata),
	})
	attemptStatus := "succeeded"
	providerResponse := ""
	if submission != nil {
		providerResponse = mustMarshal(submission)
	}
	if err != nil {
		attemptStatus = platformconst.StatusFailed
		_, _ = s.RecordRuntimeAttempt(job.ID, RecordRuntimeAttemptInput{
			Status:       attemptStatus,
			ErrorClass:   classifyProviderErrorClass(err),
			ErrorCode:    "PROVIDER_SUBMIT_FAILED",
			ErrorMessage: err.Error(),
			ProviderCode: providerCode,
			ProviderMode: job.ProviderMode,
		})
		return s.handleDispatchError(job, err, now)
	}
	_, _ = s.RecordRuntimeAttempt(job.ID, RecordRuntimeAttemptInput{
		Status:           attemptStatus,
		ProviderCode:     providerCode,
		ProviderMode:     job.ProviderMode,
		ProviderResponse: providerResponse,
	})
	if _, _, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
		Event:         RuntimeJobEventProviderAccepted,
		Now:           now,
		ProviderJobID: submission.ProviderJobID,
		Stage:         defaultString(submission.Stage, "provider_accepted"),
		StageMessage:  defaultString(submission.StageMessage, "Accepted by provider"),
	}); err != nil {
		return err
	}
	s.runtimeJobLogger(job).
		With("eta_seconds", submission.EtaSeconds).
		Info("runtime.dispatch.accepted")
	eta := submission.EtaSeconds
	s.notifyProductRuntimeUpdate(job, ProductUpdateRuntimeInput{
		Status:        platformconst.StatusProcessing,
		Stage:         job.Stage,
		StageMessage:  job.StageMessage,
		EtaSeconds:    &eta,
		ProviderJobID: job.ProviderJobID,
	})
	if submission.Completion != nil {
		if err := s.completeRuntimeJobWithContext(ctx, job, input, submission.Completion, now); err != nil {
			return s.failRuntimeJob(job, "result_persist_failed", "RESULT_PERSIST_FAILED", err.Error(), now)
		}
		return nil
	}
	if s.queue != nil {
		return s.queue.EnqueuePoll(job.ID, s.cfg.PollInitialBackoff)
	}
	return nil
}

func (s *Service) handleDispatchError(job *models.RuntimeJob, err error, now time.Time) error {
	errorClass := classifyProviderErrorClass(err)
	fromProvider := job.ProviderCode
	if s.tryFallbackProvider(job, errorClass) {
		if _, _, saveErr := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
			Event:         RuntimeJobEventFallbackScheduled,
			Now:           now,
			Stage:         "fallback_scheduled",
			StageMessage:  "Fallback provider scheduled",
			ProviderCode:  job.ProviderCode,
			RouteSnapshot: job.RouteSnapshot,
			ErrorClass:    errorClass,
			ErrorCode:     "PROVIDER_FALLBACK_SCHEDULED",
			ErrorMessage:  err.Error(),
		}); saveErr != nil {
			return saveErr
		}
		s.runtimeJobLogger(job).
			With("from_provider", fromProvider, "to_provider", job.ProviderCode, "error_class", errorClass, "error_code", job.ErrorCode).
			Warn("runtime.fallback.scheduled")
		if s.queue != nil {
			return s.queue.EnqueueDispatch(job.ID, 0)
		}
		return nil
	}
	if !isRetryableProviderError(err) || job.AttemptCount >= job.MaxAttempts {
		return s.failRuntimeJob(job, classifyProviderErrorClass(err), "PROVIDER_SUBMIT_FAILED", err.Error(), now)
	}
	retryAt := now.Add(s.cfg.RetryBackoff)
	if _, _, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
		Event:        RuntimeJobEventRetryScheduled,
		Now:          now,
		Stage:        "retry_scheduled",
		StageMessage: "Retry scheduled after provider failure",
		NextRetryAt:  &retryAt,
		ErrorClass:   classifyProviderErrorClass(err),
		ErrorCode:    "PROVIDER_SUBMIT_FAILED",
		ErrorMessage: err.Error(),
	}); err != nil {
		return err
	}
	s.runtimeJobLogger(job).
		With("retry_at", retryAt.Format(time.RFC3339), "error_class", job.ErrorClass, "error_code", job.ErrorCode).
		Warn("runtime.retry.scheduled")
	s.notifyProductRuntimeUpdate(job, ProductUpdateRuntimeInput{
		Status:       "queued",
		Stage:        job.Stage,
		StageMessage: job.StageMessage,
	})
	if s.queue != nil {
		return s.queue.EnqueueDispatch(job.ID, s.cfg.RetryBackoff)
	}
	return nil
}

func (s *Service) pollRuntimeJob(job *models.RuntimeJob, now time.Time) error {
	return s.pollRuntimeJobWithContext(context.Background(), job, now)
}

func (s *Service) pollRuntimeJobWithContext(ctx context.Context, job *models.RuntimeJob, now time.Time) error {
	if job.Status == platformconst.StatusCompleted || job.Status == platformconst.StatusFailed || job.Status == platformconst.StatusCanceled {
		return nil
	}
	if job.TimeoutAt != nil && !now.Before(*job.TimeoutAt) {
		return s.failRuntimeJob(job, "provider_timeout", "PROVIDER_TIMEOUT", "runtime job timed out while waiting for provider", now)
	}
	if job.ProviderCode == "" || job.ProviderJobID == "" || s.registry == nil {
		return nil
	}
	provider, err := s.registry.Get(job.ProviderCode)
	if err != nil {
		return err
	}
	result, err := provider.Poll(ctx, job.ProviderJobID)
	if err != nil {
		return s.handlePollError(job, err, now)
	}
	if result == nil {
		return nil
	}
	progress := result.Progress
	eta := result.EtaSeconds
	s.runtimeJobLogger(job).
		With("progress", progress, "eta_seconds", eta, "provider_stage", result.Stage, "provider_status", result.Status).
		Info("runtime.poll.progress")
	switch result.Status {
	case platformconst.StatusCompleted:
		if result.Completion == nil {
			return s.failRuntimeJob(job, "result_invalid", "PROVIDER_RESULT_INVALID", "provider completed without completion payload", now)
		}
		input, decodeErr := decodeRuntimeInputManifest(job.InputManifest)
		if decodeErr != nil {
			return s.failRuntimeJob(job, "input_manifest_invalid", "INPUT_MANIFEST_INVALID", decodeErr.Error(), now)
		}
		if err := s.completeRuntimeJobWithContext(ctx, job, input, result.Completion, now); err != nil {
			return s.failRuntimeJob(job, "result_persist_failed", "RESULT_PERSIST_FAILED", err.Error(), now)
		}
		return nil
	case platformconst.StatusFailed:
		failErr := newNonRetryableProviderError(defaultString(result.ErrorMessage, "provider task failed"))
		return s.handlePollError(job, failErr, now)
	default:
		_, transitionResult, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
			Event:        RuntimeJobEventProviderProgress,
			Now:          now,
			Stage:        defaultString(result.Stage, "provider_running"),
			StageMessage: defaultString(result.StageMessage, "Provider is still processing"),
		})
		if err != nil {
			return err
		}
		if transitionResult.Noop {
			return nil
		}
		s.notifyProductRuntimeUpdate(job, ProductUpdateRuntimeInput{
			Status:        platformconst.StatusProcessing,
			Stage:         job.Stage,
			StageMessage:  job.StageMessage,
			Progress:      &progress,
			EtaSeconds:    &eta,
			ProviderJobID: job.ProviderJobID,
		})
		if s.queue != nil {
			return s.queue.EnqueuePoll(job.ID, s.cfg.PollBackoff)
		}
	}
	return nil
}

func (s *Service) handlePollError(job *models.RuntimeJob, err error, now time.Time) error {
	if job.TimeoutAt != nil && !now.Before(*job.TimeoutAt) {
		return s.failRuntimeJob(job, "provider_timeout", "PROVIDER_TIMEOUT", "runtime job timed out while waiting for provider", now)
	}
	if !isRetryableProviderError(err) {
		return s.failRuntimeJob(job, classifyProviderErrorClass(err), "PROVIDER_POLL_FAILED", err.Error(), now)
	}
	retryAt := now.Add(s.cfg.PollBackoff)
	if _, _, transitionErr := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
		Event:        RuntimeJobEventProviderProgress,
		Now:          now,
		Stage:        "poll_retry_scheduled",
		StageMessage: "Retry scheduled after provider poll failure",
		NextRetryAt:  &retryAt,
		ErrorClass:   classifyProviderErrorClass(err),
		ErrorCode:    "PROVIDER_POLL_FAILED",
		ErrorMessage: err.Error(),
	}); transitionErr != nil {
		return transitionErr
	}
	s.runtimeJobLogger(job).
		With("retry_at", retryAt.Format(time.RFC3339), "error_class", job.ErrorClass, "error_code", job.ErrorCode).
		Warn("runtime.poll.retry_scheduled")
	if s.queue != nil {
		return s.queue.EnqueuePoll(job.ID, s.cfg.PollBackoff)
	}
	return nil
}

func (s *Service) failRuntimeJob(job *models.RuntimeJob, errorClass, errorCode, message string, now time.Time) error {
	if _, _, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
		Event:        RuntimeJobEventFailed,
		Now:          now,
		Stage:        platformconst.StatusFailed,
		StageMessage: message,
		ErrorClass:   errorClass,
		ErrorCode:    errorCode,
		ErrorMessage: message,
	}); err != nil {
		return err
	}
	s.runtimeJobLogger(job).
		With("error_class", errorClass, "error_code", errorCode, "error_message", message).
		Warn("runtime.failed")
	return s.notifyProductRuntimeUpdate(job, ProductUpdateRuntimeInput{
		Status:       platformconst.StatusFailed,
		Stage:        platformconst.StatusFailed,
		StageMessage: message,
	})
}

func decodeRuntimeInputManifest(raw string) (RuntimeInputManifest, error) {
	var manifest RuntimeInputManifest
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
			return RuntimeInputManifest{}, err
		}
	}
	if manifest.ParamsSnapshot == nil {
		manifest.ParamsSnapshot = map[string]any{}
	}
	return manifest, nil
}

func (s *Service) hydrateRuntimeSourceAssets(input *RuntimeInputManifest) error {
	if input == nil || s.storage == nil {
		return nil
	}
	for i := range input.SourceAssets {
		asset := &input.SourceAssets[i]
		if strings.TrimSpace(asset.StorageKey) == "" {
			continue
		}
		dataURL, err := s.storage.DataURLFromStorageKey(asset.StorageKey, asset.MimeType)
		if err != nil {
			return err
		}
		asset.SourceURL = dataURL
		asset.PreviewURL = dataURL
	}
	return nil
}

func runtimeInlineStoragePayload(inlineData, mimeType string) string {
	trimmed := strings.TrimSpace(inlineData)
	if trimmed == "" || strings.HasPrefix(trimmed, "data:") {
		return trimmed
	}
	lowerMime := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(lowerMime, "text/") || lowerMime == "application/json" || strings.HasSuffix(lowerMime, "+json") {
		return base64.StdEncoding.EncodeToString([]byte(trimmed))
	}
	return trimmed
}

func decodeJSONMap(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var out map[string]any
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func sanitizeProviderCallbackMetadata(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	out, _ := sanitizeProviderCallbackMetadataValue(raw).(map[string]any)
	if len(out) == 0 {
		return nil
	}
	return out
}

func sanitizeProviderCallbackMetadataValue(raw any) any {
	switch value := raw.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			if isRuntimeSensitiveMetadataKey(key) {
				continue
			}
			if sanitized := sanitizeProviderCallbackMetadataValue(child); sanitized != nil {
				out[key] = sanitized
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		out := make([]any, 0, len(value))
		for _, child := range value {
			if sanitized := sanitizeProviderCallbackMetadataValue(child); sanitized != nil {
				out = append(out, sanitized)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		rv := reflect.ValueOf(raw)
		if !rv.IsValid() {
			return nil
		}
		switch rv.Kind() {
		case reflect.Map:
			out := map[string]any{}
			for _, key := range rv.MapKeys() {
				if key.Kind() != reflect.String || isRuntimeSensitiveMetadataKey(key.String()) {
					continue
				}
				child := rv.MapIndex(key)
				if child.IsValid() && child.CanInterface() {
					if sanitized := sanitizeProviderCallbackMetadataValue(child.Interface()); sanitized != nil {
						out[key.String()] = sanitized
					}
				}
			}
			if len(out) == 0 {
				return nil
			}
			return out
		case reflect.Slice, reflect.Array:
			out := make([]any, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				child := rv.Index(i)
				if child.IsValid() && child.CanInterface() {
					if sanitized := sanitizeProviderCallbackMetadataValue(child.Interface()); sanitized != nil {
						out = append(out, sanitized)
					}
				}
			}
			if len(out) == 0 {
				return nil
			}
			return out
		}
		return raw
	}
}

func isRuntimeSensitiveMetadataKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "" {
		return true
	}
	blocked := []string{"credential", "secret", "api_key", "apikey", "token", "password", "billing", "charge", "wallet", "internal", "provider_response", "raw_payload", "storage_key", "connection", "dsn"}
	for _, word := range blocked {
		if lower == word || strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

func mustMarshal(value any) string {
	body, _ := json.Marshal(value)
	return string(body)
}

func classifyProviderErrorClass(err error) string {
	if err == nil {
		return ""
	}
	if isRetryableProviderError(err) {
		return "retryable_provider"
	}
	return "non_retryable_provider"
}

func (s *Service) resolveProviderCode(job *models.RuntimeJob) (string, error) {
	if job.ProviderCode != "" {
		return job.ProviderCode, nil
	}
	snapshot := decodeRouteSnapshot(job.RouteSnapshot)
	bindings, err := s.repo.ListProviderBindings(job.ProductCode, job.TaskType)
	if err != nil {
		return "", err
	}
	ranked := rankProviderBindings(bindings, snapshot)
	if len(ranked) == 0 {
		return "", fmt.Errorf("no enabled provider binding found for %s/%s", job.ProductCode, job.TaskType)
	}
	snapshot.CandidateProviders = candidateProviderCodes(ranked)
	if snapshot.CurrentProviderIdx < 0 || snapshot.CurrentProviderIdx >= len(snapshot.CandidateProviders) {
		snapshot.CurrentProviderIdx = 0
	}
	job.RouteSnapshot = encodeRouteSnapshot(snapshot)
	return snapshot.CandidateProviders[snapshot.CurrentProviderIdx], nil
}

func (s *Service) callbackClientForJob(job *models.RuntimeJob) ProductRuntimeCallbackClient {
	endpoint := s.productEndpoint(job.ProductCode)
	if endpoint == nil {
		return nil
	}
	return buildProductCallbackClient(endpoint)
}

func (s *Service) runtimeJobLogger(job *models.RuntimeJob) *slog.Logger {
	if job == nil {
		return logger.With("module", "runtime")
	}
	return logger.With(
		"module", "runtime",
		"runtime_job_id", job.ID,
		"product_code", job.ProductCode,
		"task_type", job.TaskType,
		"provider_code", job.ProviderCode,
		"provider_job_id", job.ProviderJobID,
		"source_type", job.SourceType,
		"source_id", job.SourceID,
		"attempt_count", job.AttemptCount,
	)
}

func (s *Service) productEndpoint(productCode string) *models.RuntimeProductEndpoint {
	endpoint, err := s.repo.FindActiveProductEndpoint(productCode)
	if err != nil {
		return nil
	}
	return endpoint
}

func (s *Service) notifyProductRuntimeUpdate(job *models.RuntimeJob, input ProductUpdateRuntimeInput) error {
	return s.enqueueCallbackDelivery(job, "runtime_update", input)
}

func (s *Service) notifyProductResults(job *models.RuntimeJob, input ProductRecordResultsInput) error {
	return s.enqueueCallbackDelivery(job, "result", input)
}

func (s *Service) enqueueCallbackDelivery(job *models.RuntimeJob, callbackType string, payload any) error {
	if s.callbackClientForJob(job) == nil {
		return fmt.Errorf("product callback endpoint missing for product_code=%s", job.ProductCode)
	}
	body, _ := json.Marshal(payload)
	delivery := &models.RuntimeCallbackDelivery{
		ID:           fmt.Sprintf("cb_%d", time.Now().UnixNano()),
		RuntimeJobID: job.ID,
		ProductCode:  job.ProductCode,
		SourceID:     job.SourceID,
		CallbackType: callbackType,
		Status:       "pending",
		PayloadJSON:  string(body),
		MaxAttempts:  8,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.repo.CreateCallbackDelivery(delivery); err != nil {
		return err
	}
	if s.queue != nil {
		return s.queue.EnqueueCallback(delivery.ID, 0)
	}
	return s.HandleCallbackTask(context.Background(), delivery.ID)
}

func (s *Service) tryFallbackProvider(job *models.RuntimeJob, errorClass string) bool {
	snapshot := decodeRouteSnapshot(job.RouteSnapshot)
	if len(snapshot.CandidateProviders) == 0 {
		return false
	}
	var currentBinding *models.RuntimeProviderBinding
	bindings, err := s.repo.ListProviderBindings(job.ProductCode, job.TaskType)
	if err != nil {
		return false
	}
	for i := range bindings {
		if bindings[i].ProviderCode == job.ProviderCode {
			currentBinding = &bindings[i]
			break
		}
	}
	if !fallbackAllowed(currentBinding, errorClass) {
		return false
	}
	if snapshot.CurrentProviderIdx+1 >= len(snapshot.CandidateProviders) {
		return false
	}
	snapshot.CurrentProviderIdx++
	job.ProviderCode = snapshot.CandidateProviders[snapshot.CurrentProviderIdx]
	job.RouteSnapshot = encodeRouteSnapshot(snapshot)
	return true
}

func (s *Service) providerCallbackURL(runtimeJobID string) string {
	if strings.TrimSpace(s.security.EncryptionKey) == "" || strings.TrimSpace(s.comfy.CallbackBaseURL) == "" {
		return ""
	}
	expiresAt := time.Now().Add(s.cfg.PollTimeout).Unix()
	payload := runtimeJobID + ":" + strconv.FormatInt(expiresAt, 10)
	mac := hmac.New(sha256.New, []byte(s.security.EncryptionKey))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	values := url.Values{}
	values.Set("runtime_job_id", runtimeJobID)
	values.Set("expires", strconv.FormatInt(expiresAt, 10))
	values.Set("sig", sig)
	return strings.TrimRight(s.comfy.CallbackBaseURL, "/") + "/api/v1/runtime/providers/comfyui/callback?" + values.Encode()
}
