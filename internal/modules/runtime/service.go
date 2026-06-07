package runtime

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	assetstorage "platform-service/internal/modules/assetstorage"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

type Service struct {
	repo     *repository.RuntimeRepository
	cfg      config.RuntimeConfig
	security config.SecurityConfig
	comfy    config.ComfyUIBridgeConfig
	queue    JobQueue
	registry *ProviderRegistry
	storage  *assetstorage.Service
}

type CreateProviderDefinitionInput struct {
	Code          string `json:"code" binding:"required"`
	Name          string `json:"name" binding:"required"`
	ProviderType  string `json:"provider_type" binding:"required"`
	Mode          string `json:"mode" binding:"required"`
	CredentialRef string `json:"credential_ref"`
	Capabilities  string `json:"capabilities"`
	Status        string `json:"status"`
	Metadata      string `json:"metadata"`
}

type CreateRuntimeJobInput struct {
	ProductCode     string `json:"product_code" binding:"required"`
	TaskType        string `json:"task_type" binding:"required"`
	ProviderCode    string `json:"provider_code"`
	ProviderMode    string `json:"provider_mode" binding:"required"`
	OrganizationID  string `json:"organization_id" binding:"required"`
	UserID          string `json:"user_id"`
	SourceType      string `json:"source_type" binding:"required"`
	SourceID        string `json:"source_id" binding:"required"`
	IdempotencyKey  string `json:"idempotency_key"`
	ChargeSessionID string `json:"charge_session_id"`
	InputManifest   string `json:"input_manifest"`
	RouteSnapshot   string `json:"route_snapshot"`
	Metadata        string `json:"metadata"`
	Priority        int    `json:"priority"`
	MaxAttempts     int    `json:"max_attempts"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
}

type UpdateRuntimeJobInput struct {
	Status         string `json:"status"`
	Stage          string `json:"stage"`
	StageMessage   string `json:"stage_message"`
	ProviderJobID  string `json:"provider_job_id"`
	ErrorClass     string `json:"error_class"`
	ErrorCode      string `json:"error_code"`
	ErrorMessage   string `json:"error_message"`
	OutputManifest string `json:"output_manifest"`
	RouteSnapshot  string `json:"route_snapshot"`
	Metadata       string `json:"metadata"`
	AttemptCount   *int   `json:"attempt_count"`
	NextRetryAt    string `json:"next_retry_at"`
}

var ErrRuntimeJobIdempotencyConflict = errors.New("runtime job idempotency key conflicts with a different request boundary")

type RecordRuntimeAttemptInput struct {
	Status           string `json:"status" binding:"required"`
	ErrorClass       string `json:"error_class"`
	ErrorCode        string `json:"error_code"`
	ErrorMessage     string `json:"error_message"`
	ProviderCode     string `json:"provider_code"`
	ProviderMode     string `json:"provider_mode"`
	ProviderRequest  string `json:"provider_request"`
	ProviderResponse string `json:"provider_response"`
	StartedAt        string `json:"started_at"`
	EndedAt          string `json:"ended_at"`
}

type RuntimeJobDetail struct {
	Job      *models.RuntimeJob      `json:"job"`
	Attempts []models.RuntimeAttempt `json:"attempts"`
}

type ListRuntimeJobsInput struct {
	OrganizationID string
	Status         string
	Stage          string
	Query          string
	Limit          int
	Offset         int
}

type RuntimeJobListResult struct {
	Items  []models.RuntimeJob `json:"items"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

func NewService(repo *repository.RuntimeRepository, cfg config.RuntimeConfig, security config.SecurityConfig, comfy config.ComfyUIBridgeConfig) *Service {
	return &Service{repo: repo, cfg: defaultRuntimeConfig(cfg), security: security, comfy: comfy}
}

func (s *Service) UseRuntime(queue JobQueue, registry *ProviderRegistry) {
	if queue != nil {
		s.queue = queue
	}
	if registry != nil {
		s.registry = registry
	}
}

func (s *Service) UseAssetStorage(storage *assetstorage.Service) {
	if storage != nil {
		s.storage = storage
	}
}

func (s *Service) CreateProviderDefinition(input CreateProviderDefinitionInput) (*models.RuntimeProviderDefinition, error) {
	item := &models.RuntimeProviderDefinition{
		ID:            utils.GenerateID(),
		Code:          input.Code,
		Name:          input.Name,
		ProviderType:  input.ProviderType,
		Mode:          defaultString(input.Mode, "sync"),
		CredentialRef: input.CredentialRef,
		Capabilities:  input.Capabilities,
		Status:        defaultString(input.Status, platformconst.StatusActive),
		Metadata:      input.Metadata,
	}
	if err := s.repo.CreateProviderDefinition(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListProviderDefinitions() ([]models.RuntimeProviderDefinition, error) {
	return s.repo.ListProviderDefinitions()
}

func (s *Service) CreateRuntimeJob(input CreateRuntimeJobInput) (*models.RuntimeJob, error) {
	if input.IdempotencyKey != "" {
		if existing, err := s.repo.FindRuntimeJobByIdempotencyKey(input.ProductCode, input.OrganizationID, input.SourceType, input.SourceID, input.TaskType, input.IdempotencyKey); err == nil {
			return existing, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	// NOTE: 先检查 queue 是否配置，避免创建孤儿记录
	if s.queue == nil {
		return nil, errors.New("runtime queue is not configured")
	}
	item := &models.RuntimeJob{
		ID:              utils.GenerateID(),
		ProductCode:     input.ProductCode,
		TaskType:        input.TaskType,
		ProviderCode:    input.ProviderCode,
		ProviderMode:    defaultString(input.ProviderMode, "sync"),
		OrganizationID:  input.OrganizationID,
		UserID:          input.UserID,
		SourceType:      input.SourceType,
		SourceID:        input.SourceID,
		ChargeSessionID: input.ChargeSessionID,
		Status:          platformconst.StatusQueued,
		Stage:           platformconst.StatusQueued,
		StageMessage:    "Runtime job queued",
		InputManifest:   input.InputManifest,
		RouteSnapshot:   input.RouteSnapshot,
		Metadata:        input.Metadata,
		Priority:        input.Priority,
		AttemptCount:    0,
		MaxAttempts:     defaultInt(input.MaxAttempts, 3),
	}
	if input.IdempotencyKey != "" {
		item.IdempotencyKey = &input.IdempotencyKey
	}
	if input.TimeoutSeconds > 0 {
		timeoutAt := time.Now().Add(time.Duration(input.TimeoutSeconds) * time.Second)
		item.TimeoutAt = &timeoutAt
	}
	// NOTE: 创建作业和入队必须在同一事务内，入队失败时回滚 DB 记录
	var created *models.RuntimeJob
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if err := s.queue.EnqueueDispatch(item.ID, 0); err != nil {
			// 入队失败，事务回滚删除孤儿记录
			return err
		}
		created = item
		return nil
	}); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) GetRuntimeJob(id string) (*RuntimeJobDetail, error) {
	job, err := s.repo.FindRuntimeJobByID(id)
	if err != nil {
		return nil, err
	}
	attempts, err := s.repo.ListRuntimeAttempts(id)
	if err != nil {
		return nil, err
	}
	return &RuntimeJobDetail{Job: job, Attempts: attempts}, nil
}

func (s *Service) ListRuntimeJobs(input ListRuntimeJobsInput) (*RuntimeJobListResult, error) {
	limit := defaultInt(input.Limit, 20)
	if limit > 100 {
		limit = 100
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	items, total, err := s.repo.ListRuntimeJobs(repository.RuntimeJobListFilter{
		OrganizationID: input.OrganizationID,
		Status:         input.Status,
		Stage:          input.Stage,
		Query:          input.Query,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	return &RuntimeJobListResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *Service) UpdateRuntimeJob(id string, input UpdateRuntimeJobInput) (*models.RuntimeJob, error) {
	item, err := s.repo.FindRuntimeJobByID(id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var nextRetryAt *time.Time
	if input.NextRetryAt != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, input.NextRetryAt); parseErr == nil {
			nextRetryAt = &parsed
		}
	}
	updated, _, err := s.transitionRuntimeJob(item, RuntimeJobTransitionInput{
		Event:          RuntimeJobEventAdminPatch,
		Now:            now,
		Status:         RuntimeJobStatus(input.Status),
		Stage:          input.Stage,
		StageMessage:   input.StageMessage,
		ProviderJobID:  input.ProviderJobID,
		ErrorClass:     input.ErrorClass,
		ErrorCode:      input.ErrorCode,
		ErrorMessage:   input.ErrorMessage,
		OutputManifest: input.OutputManifest,
		RouteSnapshot:  input.RouteSnapshot,
		Metadata:       input.Metadata,
		AttemptCount:   input.AttemptCount,
		NextRetryAt:    nextRetryAt,
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) CancelRuntimeJob(id string) (*models.RuntimeJob, error) {
	job, err := s.repo.FindRuntimeJobByID(id)
	if err != nil {
		return nil, err
	}
	if job.Status != platformconst.StatusCompleted && job.Status != platformconst.StatusFailed && job.Status != platformconst.StatusCanceled && job.ProviderCode != "" && job.ProviderJobID != "" && s.registry != nil {
		provider, providerErr := s.registry.Get(job.ProviderCode)
		if providerErr != nil {
			return nil, providerErr
		}
		if cancelErr := provider.Cancel(context.Background(), job.ProviderJobID); cancelErr != nil {
			return nil, cancelErr
		}
	}
	return s.UpdateRuntimeJob(id, UpdateRuntimeJobInput{
		Status:       platformconst.StatusCanceled,
		Stage:        platformconst.StatusCanceled,
		StageMessage: "Runtime job canceled",
	})
}

func (s *Service) RecordRuntimeAttempt(runtimeJobID string, input RecordRuntimeAttemptInput) (*models.RuntimeAttempt, error) {
	previous, err := s.repo.ListRuntimeAttempts(runtimeJobID)
	if err != nil {
		return nil, err
	}
	item := &models.RuntimeAttempt{
		ID:               utils.GenerateID(),
		RuntimeJobID:     runtimeJobID,
		AttemptNo:        len(previous) + 1,
		Status:           input.Status,
		ErrorClass:       input.ErrorClass,
		ErrorCode:        input.ErrorCode,
		ErrorMessage:     input.ErrorMessage,
		ProviderCode:     input.ProviderCode,
		ProviderMode:     input.ProviderMode,
		ProviderRequest:  input.ProviderRequest,
		ProviderResponse: input.ProviderResponse,
	}
	if input.StartedAt != "" {
		if startedAt, err := time.Parse(time.RFC3339, input.StartedAt); err == nil {
			item.StartedAt = &startedAt
		}
	}
	if input.EndedAt != "" {
		if endedAt, err := time.Parse(time.RFC3339, input.EndedAt); err == nil {
			item.EndedAt = &endedAt
		}
	}
	if err := s.repo.CreateRuntimeAttempt(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) HandleProviderCallback(providerCode, runtimeJobID string, expiresAt int64, sig string) error {
	return s.HandleProviderCallbackPayload(providerCode, runtimeJobID, expiresAt, sig, nil)
}

func (s *Service) HandleProviderCallbackPayload(providerCode, runtimeJobID string, expiresAt int64, sig string, payload *NormalizedProviderCallbackPayload) error {
	if !s.validateProviderCallbackSignature(runtimeJobID, expiresAt, sig) {
		return fmt.Errorf("invalid provider callback signature")
	}
	job, err := s.repo.FindRuntimeJobByID(runtimeJobID)
	if err != nil {
		return err
	}
	if job.Status == platformconst.StatusCompleted || job.Status == platformconst.StatusFailed || job.Status == platformconst.StatusCanceled {
		if payload != nil {
			switch strings.ToLower(strings.TrimSpace(payload.Status)) {
			case platformconst.StatusQueued, platformconst.StatusProcessing, "running", "in_progress":
				_, _, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
					Event:         RuntimeJobEventProviderProgress,
					Now:           time.Now(),
					Stage:         defaultString(payload.Stage, "provider_running"),
					StageMessage:  defaultString(payload.StageMessage, "Provider is still processing"),
					ProviderJobID: payload.ProviderJobID,
				})
				return err
			}
		}
		return nil
	}
	if providerCode != "" && job.ProviderCode != "" && providerCode != job.ProviderCode && !(providerCode == "comfyui" && job.ProviderCode == "comfyui_bridge") {
		return fmt.Errorf("provider callback mismatch")
	}
	if payload == nil || strings.TrimSpace(payload.Status) == "" {
		return s.pollRuntimeJob(job, time.Now())
	}
	if payload.ProviderCode == "" {
		payload.ProviderCode = providerCode
	}
	if payload.ProviderJobID != "" && job.ProviderJobID == "" {
		if _, _, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
			Event:         RuntimeJobEventProviderAccepted,
			Now:           time.Now(),
			ProviderJobID: payload.ProviderJobID,
			Stage:         defaultString(payload.Stage, "provider_accepted"),
			StageMessage:  defaultString(payload.StageMessage, "Accepted by provider callback"),
		}); err != nil {
			return err
		}
	}
	now := time.Now()
	switch strings.ToLower(strings.TrimSpace(payload.Status)) {
	case platformconst.StatusCompleted, "succeeded", "success":
		completion := payload.Completion
		if completion == nil {
			completion = &ProviderCompletion{Status: platformconst.StatusCompleted, Progress: payload.Progress, StageMessage: payload.StageMessage, Variants: payload.Variants, Metadata: payload.Metadata}
		}
		if completion.Progress <= 0 {
			completion.Progress = 100
		}
		if completion.Status == "" {
			completion.Status = platformconst.StatusCompleted
		}
		if len(completion.Variants) == 0 {
			if job.TaskType == RuntimeTaskImageUnderstanding {
				if resultText, _ := completion.Metadata["result_text"].(string); strings.TrimSpace(resultText) != "" {
					completion.Variants = append(completion.Variants, ProviderResultVariant{Index: 0, AssetType: "json", InlineData: resultText, MimeType: "application/json", Metadata: map[string]any{"provider": job.ProviderCode, "task_id": job.ProviderJobID}})
				} else if payload != nil {
					if resultText, _ := payload.Metadata["result_text"].(string); strings.TrimSpace(resultText) != "" {
						assetType := "text"
						mimeType := "text/plain"
						if json.Valid([]byte(resultText)) {
							assetType = "json"
							mimeType = "application/json"
						}
						completion.Variants = append(completion.Variants, ProviderResultVariant{Index: 0, AssetType: assetType, InlineData: resultText, MimeType: mimeType, Metadata: map[string]any{"provider": job.ProviderCode, "task_id": job.ProviderJobID}})
					}
				}
			}
		}
		if len(completion.Variants) == 0 {
			return s.failRuntimeJob(job, "provider_callback_invalid", "PROVIDER_CALLBACK_INVALID", "provider callback completed without variants", now)
		}
		input, _ := decodeRuntimeInputManifest(job.InputManifest)
		return s.completeRuntimeJob(job, input, completion, now)
	case platformconst.StatusFailed, "error":
		return s.failRuntimeJob(job, defaultString(payload.ErrorClass, "provider_failed"), defaultString(payload.ErrorCode, "PROVIDER_CALLBACK_FAILED"), defaultString(payload.ErrorMessage, payload.StageMessage), now)
	case platformconst.StatusCanceled, "cancelled":
		return s.cancelRuntimeJobFromProviderCallback(job, defaultString(payload.StageMessage, "Provider canceled runtime job"), now)
	case platformconst.StatusQueued, platformconst.StatusProcessing, "running", "in_progress":
		progress := payload.Progress
		_, result, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
			Event:         RuntimeJobEventProviderProgress,
			Now:           now,
			Stage:         defaultString(payload.Stage, "provider_running"),
			StageMessage:  defaultString(payload.StageMessage, "Provider is still processing"),
			ProviderJobID: payload.ProviderJobID,
		})
		if err != nil {
			return err
		}
		if result.Noop {
			return nil
		}
		return s.notifyProductRuntimeUpdate(job, ProductUpdateRuntimeInput{Status: platformconst.StatusProcessing, Stage: job.Stage, StageMessage: job.StageMessage, Progress: &progress, ProviderJobID: job.ProviderJobID})
	default:
		return fmt.Errorf("invalid provider callback status: %s", payload.Status)
	}
}

func (s *Service) cancelRuntimeJobFromProviderCallback(job *models.RuntimeJob, message string, now time.Time) error {
	if _, _, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
		Event:        RuntimeJobEventCanceled,
		Now:          now,
		Stage:        platformconst.StatusCanceled,
		StageMessage: message,
	}); err != nil {
		return err
	}
	return s.notifyProductRuntimeUpdate(job, ProductUpdateRuntimeInput{Status: platformconst.StatusCanceled, Stage: platformconst.StatusCanceled, StageMessage: message, ProviderJobID: job.ProviderJobID})
}

func (s *Service) validateProviderCallbackSignature(runtimeJobID string, expiresAt int64, sig string) bool {
	if runtimeJobID == "" || sig == "" || expiresAt <= 0 || time.Now().Unix() > expiresAt {
		return false
	}
	mac := hmac.New(sha256.New, []byte(s.security.EncryptionKey))
	mac.Write([]byte(runtimeJobID))
	mac.Write([]byte(":"))
	mac.Write([]byte(strconv.FormatInt(expiresAt, 10)))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// --- 状态机转移校验 ---

// RuntimeJob 合法状态转移白名单。终态不可重新打开；失败后的重试应创建新的尝试/作业生命周期，而不是把终态作业改回 queued。
var validRuntimeJobTransitions = map[string][]string{
	platformconst.StatusQueued:     {platformconst.StatusProcessing, platformconst.StatusCanceled, platformconst.StatusFailed},
	platformconst.StatusProcessing: {platformconst.StatusCompleted, platformconst.StatusFailed, platformconst.StatusCanceled},
}

func validateRuntimeJobStatusTransition(from, to string) error {
	if from == to {
		return nil
	}
	allowed, ok := validRuntimeJobTransitions[from]
	if !ok {
		return fmt.Errorf("runtime job status %q is terminal, cannot transition to %q", from, to)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid runtime job status transition: %q -> %q", from, to)
}
