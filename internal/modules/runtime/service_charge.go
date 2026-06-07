package runtime

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
