package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
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

type CreateChargeSessionInput struct {
	SourceType         string `json:"source_type" binding:"required"`
	SourceID           string `json:"source_id" binding:"required"`
	ProductCode        string `json:"product_code" binding:"required"`
	OrganizationID     string `json:"organization_id" binding:"required"`
	UserID             string `json:"user_id"`
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	BillableItemCode   string `json:"billable_item_code" binding:"required"`
	ResourceType       string `json:"resource_type" binding:"required"`
	ReservationKey     string `json:"reservation_key"`
	EstimatedUnits     int64  `json:"estimated_units"`
	RouteSnapshot      string `json:"route_snapshot"`
	Metadata           string `json:"metadata"`
}

type UpdateChargeSessionInput struct {
	Status         string `json:"status"`
	ReservationID  string `json:"reservation_id"`
	FinalizationID string `json:"finalization_id"`
	EventID        string `json:"event_id"`
	SettlementID   string `json:"settlement_id"`
	FinalUnits     *int64 `json:"final_units"`
	RouteSnapshot  string `json:"route_snapshot"`
	Metadata       string `json:"metadata"`
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

type ListChargeSessionsInput struct {
	OrganizationID string
	Status         string
	ProductCode    string
	Query          string
	Limit          int
	Offset         int
}

type ChargeSessionListResult struct {
	Items  []models.ChargeSession `json:"items"`
	Total  int64                  `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
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
		if existing, err := s.repo.FindRuntimeJobByIdempotencyKey(input.IdempotencyKey); err == nil {
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
	if input.Status != "" {
		if err := validateRuntimeJobStatusTransition(item.Status, input.Status); err != nil {
			return nil, err
		}
		item.Status = input.Status
	}
	if input.Stage != "" {
		item.Stage = input.Stage
	}
	if input.StageMessage != "" {
		item.StageMessage = input.StageMessage
	}
	if input.ProviderJobID != "" {
		item.ProviderJobID = input.ProviderJobID
	}
	if input.ErrorClass != "" {
		item.ErrorClass = input.ErrorClass
	}
	if input.ErrorCode != "" {
		item.ErrorCode = input.ErrorCode
	}
	if input.ErrorMessage != "" {
		item.ErrorMessage = input.ErrorMessage
	}
	if input.OutputManifest != "" {
		item.OutputManifest = input.OutputManifest
	}
	if input.RouteSnapshot != "" {
		item.RouteSnapshot = input.RouteSnapshot
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if input.AttemptCount != nil {
		item.AttemptCount = *input.AttemptCount
	}
	if input.NextRetryAt != "" {
		if nextRetryAt, err := time.Parse(time.RFC3339, input.NextRetryAt); err == nil {
			item.NextRetryAt = &nextRetryAt
		}
	}
	switch item.Status {
	case platformconst.StatusCompleted:
		item.CompletedAt = &now
		item.CanceledAt = nil
		item.NextRetryAt = nil
	case platformconst.StatusCanceled:
		item.CanceledAt = &now
		item.NextRetryAt = nil
	case platformconst.StatusFailed:
		item.NextRetryAt = nil
	}
	if err := s.repo.SaveRuntimeJob(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CancelRuntimeJob(id string) (*models.RuntimeJob, error) {
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

func (s *Service) CreateChargeSession(input CreateChargeSessionInput) (*models.ChargeSession, error) {
	item := &models.ChargeSession{
		ID:                 utils.GenerateID(),
		SourceType:         input.SourceType,
		SourceID:           input.SourceID,
		ProductCode:        input.ProductCode,
		OrganizationID:     input.OrganizationID,
		UserID:             input.UserID,
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		BillableItemCode:   input.BillableItemCode,
		ResourceType:       input.ResourceType,
		Status:             platformconst.StatusCreated,
		ReservationKey:     defaultString(input.ReservationKey, utils.GenerateID()),
		EstimatedUnits:     defaultInt64(input.EstimatedUnits, 1),
		RouteSnapshot:      input.RouteSnapshot,
		Metadata:           input.Metadata,
	}
	if err := s.repo.CreateChargeSession(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) GetChargeSession(id string) (*models.ChargeSession, error) {
	return s.repo.FindChargeSessionByID(id)
}

func (s *Service) ListChargeSessions(input ListChargeSessionsInput) (*ChargeSessionListResult, error) {
	limit := defaultInt(input.Limit, 20)
	if limit > 100 {
		limit = 100
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	items, total, err := s.repo.ListChargeSessions(repository.ChargeSessionListFilter{
		OrganizationID: input.OrganizationID,
		Status:         input.Status,
		ProductCode:    input.ProductCode,
		Query:          input.Query,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	return &ChargeSessionListResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *Service) UpdateChargeSession(id string, input UpdateChargeSessionInput) (*models.ChargeSession, error) {
	item, err := s.repo.FindChargeSessionByID(id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if input.Status != "" {
		if err := validateChargeSessionStatusTransition(item.Status, input.Status); err != nil {
			return nil, err
		}
		item.Status = input.Status
	}
	if input.ReservationID != "" {
		item.ReservationID = input.ReservationID
	}
	if input.FinalizationID != "" {
		item.FinalizationID = input.FinalizationID
	}
	if input.EventID != "" {
		item.EventID = input.EventID
	}
	if input.SettlementID != "" {
		item.SettlementID = input.SettlementID
	}
	if input.FinalUnits != nil {
		item.FinalUnits = *input.FinalUnits
	}
	if input.RouteSnapshot != "" {
		item.RouteSnapshot = input.RouteSnapshot
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	switch item.Status {
	case platformconst.ReservationStatusReserved:
		item.ReservedAt = &now
	case platformconst.SettlementStatusSettled:
		item.FinalizedAt = &now
	case platformconst.ReservationStatusReleased:
		item.ReleasedAt = &now
	}
	if err := s.repo.SaveChargeSession(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) HandleProviderCallback(providerCode, runtimeJobID string, expiresAt int64, sig string) error {
	if !s.validateProviderCallbackSignature(runtimeJobID, expiresAt, sig) {
		return fmt.Errorf("invalid provider callback signature")
	}
	job, err := s.repo.FindRuntimeJobByID(runtimeJobID)
	if err != nil {
		return err
	}
	if job.Status == platformconst.StatusCompleted || job.Status == platformconst.StatusFailed || job.Status == platformconst.StatusCanceled {
		return nil
	}
	if providerCode != "" && job.ProviderCode != "" && providerCode != job.ProviderCode && !(providerCode == "comfyui" && job.ProviderCode == "comfyui_bridge") {
		return fmt.Errorf("provider callback mismatch")
	}
	return s.pollRuntimeJob(job, time.Now())
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

// RuntimeJob 合法状态转移白名单
var validRuntimeJobTransitions = map[string][]string{
	platformconst.StatusQueued:     {platformconst.StatusProcessing, platformconst.StatusCanceled, platformconst.StatusFailed},
	platformconst.StatusProcessing: {platformconst.StatusCompleted, platformconst.StatusFailed, platformconst.StatusCanceled},
	platformconst.StatusFailed:     {platformconst.StatusQueued}, // 允许重试
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

// ChargeSession 合法状态转移白名单
var validChargeSessionTransitions = map[string][]string{
	platformconst.StatusCreated:              {platformconst.ReservationStatusReserved, platformconst.StatusCanceled, platformconst.StatusFailed},
	platformconst.ReservationStatusReserved:  {platformconst.SettlementStatusSettled, platformconst.ReservationStatusReleased, platformconst.StatusFailed},
	platformconst.ReservationStatusReleased:  {},
	platformconst.SettlementStatusSettled:     {},
}

func validateChargeSessionStatusTransition(from, to string) error {
	if from == to {
		return nil
	}
	allowed, ok := validChargeSessionTransitions[from]
	if !ok {
		return fmt.Errorf("charge session status %q is terminal, cannot transition to %q", from, to)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid charge session status transition: %q -> %q", from, to)
}

func defaultInt64(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}
