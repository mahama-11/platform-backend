package runtime

import (
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
	"gorm.io/gorm/clause"
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
	if input.Status == platformconst.StatusCompleted || input.Status == platformconst.StatusFailed || input.Status == platformconst.StatusCanceled {
		if err := s.saveRuntimeJobWithTerminalChargeBinding(item, input.Status); err != nil {
			return nil, err
		}
		return item, nil
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
		if existing, findErr := s.repo.FindChargeSessionByReservationKey(item.ReservationKey); findErr == nil && chargeSessionIdempotentBoundaryMatches(existing, item) {
			return existing, nil
		}
		return nil, err
	}
	return item, nil
}

func chargeSessionIdempotentBoundaryMatches(existing, requested *models.ChargeSession) bool {
	if existing == nil || requested == nil {
		return false
	}
	return strings.TrimSpace(existing.ReservationKey) != "" &&
		strings.TrimSpace(existing.ReservationKey) == strings.TrimSpace(requested.ReservationKey) &&
		existing.SourceType == requested.SourceType &&
		existing.SourceID == requested.SourceID &&
		existing.ProductCode == requested.ProductCode &&
		existing.OrganizationID == requested.OrganizationID &&
		existing.UserID == requested.UserID &&
		existing.BillingSubjectType == requested.BillingSubjectType &&
		existing.BillingSubjectID == requested.BillingSubjectID &&
		existing.BillableItemCode == requested.BillableItemCode &&
		existing.ResourceType == requested.ResourceType
}

func (s *Service) GetChargeSession(id string) (*models.ChargeSession, error) {
	return s.repo.FindChargeSessionByID(id)
}

func (s *Service) saveRuntimeJobWithTerminalChargeBinding(job *models.RuntimeJob, terminalStatus string) error {
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(job).Error; err != nil {
			return err
		}
		return s.bindRuntimeTerminalChargeSessionTx(tx, job, terminalStatus)
	})
}

func (s *Service) bindRuntimeTerminalChargeSessionTx(tx *gorm.DB, job *models.RuntimeJob, terminalStatus string) error {
	if job == nil || strings.TrimSpace(job.ChargeSessionID) == "" {
		return nil
	}
	var session models.ChargeSession
	if err := tx.Where("id = ?", job.ChargeSessionID).First(&session).Error; err != nil {
		return err
	}
	if err := validateRuntimeChargeSessionBoundary(job, &session); err != nil {
		return err
	}
	if isTerminalChargeSessionStatus(session.Status) {
		return nil
	}
	switch terminalStatus {
	case platformconst.StatusCompleted:
		if session.Status == platformconst.StatusCreated {
			if err := transitionChargeSessionTx(tx, &session, UpdateChargeSessionInput{
				Status:        platformconst.ReservationStatusReserved,
				ReservationID: defaultString(session.ReservationID, "runtime-reservation-"+job.ID),
			}); err != nil {
				return err
			}
		}
		units := finalUnitsForRuntimeJob(job, &session)
		if err := commitRuntimeChargeReservationTx(tx, &session, units); err != nil {
			return err
		}
		return transitionChargeSessionTx(tx, &session, UpdateChargeSessionInput{
			Status:         platformconst.SettlementStatusSettled,
			FinalUnits:     &units,
			FinalizationID: defaultString(session.FinalizationID, "runtime-finalization-"+job.ID),
			SettlementID:   defaultString(session.SettlementID, "runtime-settlement-"+job.ID),
			Metadata:       mergeRuntimeChargeMetadata(session.Metadata, job, terminalStatus),
		})
	case platformconst.StatusFailed:
		if session.Status == platformconst.ReservationStatusReserved {
			return transitionChargeSessionTx(tx, &session, UpdateChargeSessionInput{
				Status:   platformconst.ReservationStatusReleased,
				Metadata: mergeRuntimeChargeMetadata(session.Metadata, job, terminalStatus),
			})
		}
		return transitionChargeSessionTx(tx, &session, UpdateChargeSessionInput{
			Status:   platformconst.StatusFailed,
			Metadata: mergeRuntimeChargeMetadata(session.Metadata, job, terminalStatus),
		})
	case platformconst.StatusCanceled:
		if session.Status == platformconst.ReservationStatusReserved {
			return transitionChargeSessionTx(tx, &session, UpdateChargeSessionInput{
				Status:   platformconst.ReservationStatusReleased,
				Metadata: mergeRuntimeChargeMetadata(session.Metadata, job, terminalStatus),
			})
		}
		return transitionChargeSessionTx(tx, &session, UpdateChargeSessionInput{
			Status:   platformconst.StatusCanceled,
			Metadata: mergeRuntimeChargeMetadata(session.Metadata, job, terminalStatus),
		})
	default:
		return nil
	}
}

func commitRuntimeChargeReservationTx(tx *gorm.DB, session *models.ChargeSession, units int64) error {
	if session == nil || strings.TrimSpace(session.ReservationID) == "" || !tx.Migrator().HasTable(&models.ResourceReservation{}) {
		return nil
	}
	var reservation models.ResourceReservation
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", session.ReservationID)
	if strings.TrimSpace(session.ID) != "" {
		query = query.Or("reference_id = ?", session.ID)
	}
	if strings.TrimSpace(session.SourceID) != "" {
		query = query.Or("reservation_key = ?", "reserve:"+session.SourceID)
	}
	if err := query.First(&reservation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if reservation.Status != platformconst.ReservationStatusReserved {
		return nil
	}
	if units <= 0 {
		units = reservation.Units
	}
	if units <= 0 {
		units = 1
	}
	now := time.Now()
	if reservation.ResourceType != platformconst.ResourceTypeQuota {
		return fmt.Errorf("runtime charge session reservation resource type %q is not supported by runtime settlement", reservation.ResourceType)
	}
	if !reservationMatchesChargeSession(&reservation, session) {
		return fmt.Errorf("runtime charge session reservation boundary mismatch")
	}
	if tx.Migrator().HasTable(&models.QuotaLedger{}) {
		if err := tx.Create(&models.QuotaLedger{
			ID:                 utils.GenerateID(),
			BillingSubjectType: reservation.BillingSubjectType,
			BillingSubjectID:   reservation.BillingSubjectID,
			BillableItemCode:   reservation.BillableItemCode,
			Direction:          platformconst.LedgerDirectionConsume,
			Units:              units,
			Reason:             "runtime_charge_session_settlement",
			ReferenceID:        session.ID,
			CreatedAt:          now,
		}).Error; err != nil {
			return err
		}
	}
	reservation.Status = platformconst.ReservationStatusCommitted
	reservation.CommittedAt = &now
	finalizationID := defaultString(session.FinalizationID, "runtime-finalization-"+session.SourceID)
	reservation.FinalizationID = &finalizationID
	return tx.Save(&reservation).Error
}

func reservationMatchesChargeSession(reservation *models.ResourceReservation, session *models.ChargeSession) bool {
	if reservation == nil || session == nil {
		return false
	}
	if session.BillingSubjectType != "" && reservation.BillingSubjectType != "" && reservation.BillingSubjectType != session.BillingSubjectType {
		return false
	}
	if session.BillingSubjectID != "" && reservation.BillingSubjectID != "" && reservation.BillingSubjectID != session.BillingSubjectID {
		return false
	}
	if session.BillableItemCode != "" && reservation.BillableItemCode != "" && reservation.BillableItemCode != session.BillableItemCode {
		return false
	}
	if session.SourceID != "" && reservation.ReferenceID != "" && reservation.ReferenceID != session.SourceID && reservation.ReferenceID != session.ID {
		return false
	}
	return true
}

func validateRuntimeChargeSessionBoundary(job *models.RuntimeJob, session *models.ChargeSession) error {
	if job == nil || session == nil {
		return fmt.Errorf("runtime charge session boundary missing job or charge session")
	}
	if session.ProductCode != "" && job.ProductCode != "" && session.ProductCode != job.ProductCode {
		return fmt.Errorf("runtime charge session product mismatch: job=%s charge_session=%s", job.ProductCode, session.ProductCode)
	}
	if session.OrganizationID != "" && job.OrganizationID != "" && session.OrganizationID != job.OrganizationID {
		return fmt.Errorf("runtime charge session organization mismatch: job=%s charge_session=%s", job.OrganizationID, session.OrganizationID)
	}
	if session.UserID != "" && job.UserID != "" && session.UserID != job.UserID {
		return fmt.Errorf("runtime charge session user mismatch: job=%s charge_session=%s", job.UserID, session.UserID)
	}
	if session.SourceType == "runtime_job" {
		if session.SourceID != job.ID {
			return fmt.Errorf("runtime charge session runtime source mismatch: runtime_job_id=%s charge_source=%s", job.ID, session.SourceID)
		}
		return nil
	}
	if session.SourceType != "" && job.SourceType != "" && session.SourceType != job.SourceType {
		return fmt.Errorf("runtime charge session source type mismatch: job_source_type=%s charge_source_type=%s", job.SourceType, session.SourceType)
	}
	if session.SourceID != "" && job.SourceID != "" && session.SourceID != job.SourceID {
		return fmt.Errorf("runtime charge session source mismatch: job_source=%s charge_source=%s", job.SourceID, session.SourceID)
	}
	return nil
}

func transitionChargeSessionTx(tx *gorm.DB, item *models.ChargeSession, input UpdateChargeSessionInput) error {
	now := time.Now()
	if input.Status != "" {
		if err := validateChargeSessionStatusTransition(item.Status, input.Status); err != nil {
			return err
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
	return tx.Save(item).Error
}

func isTerminalChargeSessionStatus(status string) bool {
	switch status {
	case platformconst.SettlementStatusSettled, platformconst.ReservationStatusReleased, platformconst.StatusCanceled, platformconst.StatusFailed:
		return true
	default:
		return false
	}
}

func finalUnitsForRuntimeJob(_ *models.RuntimeJob, session *models.ChargeSession) int64 {
	if session != nil {
		if session.FinalUnits > 0 {
			return session.FinalUnits
		}
		if session.EstimatedUnits > 0 {
			return session.EstimatedUnits
		}
	}
	return 1
}

func mergeRuntimeChargeMetadata(raw string, job *models.RuntimeJob, terminalStatus string) string {
	metadata := decodeJSONMap(raw)
	metadata["runtime_job_id"] = ""
	metadata["runtime_terminal_status"] = terminalStatus
	if job != nil {
		metadata["runtime_job_id"] = job.ID
		metadata["provider_code"] = job.ProviderCode
		metadata["provider_job_id"] = job.ProviderJobID
		metadata["task_type"] = job.TaskType
	}
	return mustMarshal(metadata)
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
		job.ProviderJobID = payload.ProviderJobID
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
		job.Status = platformconst.StatusProcessing
		job.Stage = defaultString(payload.Stage, "provider_running")
		job.StageMessage = defaultString(payload.StageMessage, "Provider is still processing")
		if err := s.repo.SaveRuntimeJob(job); err != nil {
			return err
		}
		return s.notifyProductRuntimeUpdate(job, ProductUpdateRuntimeInput{Status: platformconst.StatusProcessing, Stage: job.Stage, StageMessage: job.StageMessage, Progress: &progress, ProviderJobID: job.ProviderJobID})
	default:
		return fmt.Errorf("invalid provider callback status: %s", payload.Status)
	}
}

func (s *Service) cancelRuntimeJobFromProviderCallback(job *models.RuntimeJob, message string, now time.Time) error {
	job.Status = platformconst.StatusCanceled
	job.Stage = platformconst.StatusCanceled
	job.StageMessage = message
	job.CompletedAt = &now
	if err := s.saveRuntimeJobWithTerminalChargeBinding(job, platformconst.StatusCanceled); err != nil {
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
	platformconst.StatusCreated:             {platformconst.ReservationStatusReserved, platformconst.StatusCanceled, platformconst.StatusFailed},
	platformconst.ReservationStatusReserved: {platformconst.SettlementStatusSettled, platformconst.ReservationStatusReleased, platformconst.StatusFailed},
	platformconst.ReservationStatusReleased: {},
	platformconst.SettlementStatusSettled:   {},
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
