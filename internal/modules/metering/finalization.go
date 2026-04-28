package metering

import (
	"errors"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrReservationNotFinalizable = errors.New("reservation not finalizable")

type FinalizeInput struct {
	FinalizationID string `json:"finalization_id" binding:"required"`
	ReservationID  string `json:"reservation_id" binding:"required"`
	IngestEventInput
}

type FinalizeResult struct {
	Reservation *models.ResourceReservation `json:"reservation"`
	Event       *models.MeterEvent          `json:"event"`
	Settlement  *models.SettlementRecord    `json:"settlement,omitempty"`
}

func (s *Service) Finalize(input FinalizeInput) (*FinalizeResult, error) {
	log := logger.With(
		"finalization_id", input.FinalizationID,
		"reservation_id", input.ReservationID,
		"event_id", input.EventID,
		"product_code", input.ProductCode,
		"billable_item_code", input.BillableItemCode,
	)
	log.Info("metering.finalize.begin")
	if input.EventID == "" {
		log.Warn("metering.finalize.invalid_input", "reason", "missing_event_id")
		return nil, errors.New("event_id is required")
	}
	var (
		reservation *models.ResourceReservation
		event       *models.MeterEvent
	)
	err := s.commercialRepo.DB().Transaction(func(tx *gorm.DB) error {
		var locked models.ResourceReservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", input.ReservationID).
			First(&locked).Error; err != nil {
			return err
		}
		if locked.FinalizationID != nil && *locked.FinalizationID == input.FinalizationID {
			reservation = &locked
			if existing, err := s.commercialRepo.FindMeterEventByEventID(input.EventID); err == nil {
				event = existing
				log.Info("metering.finalize.duplicate", "status", locked.Status, "existing_event_id", existing.EventID)
				return nil
			}
			log.Info("metering.finalize.duplicate", "status", locked.Status, "existing_event_id", input.EventID)
			return nil
		}
		if locked.Status != platformconst.ReservationStatusReserved {
			return controlReservationInvalid()
		}
		ingestInput := input.IngestEventInput
		if ingestInput.BillingSubjectType == "" {
			ingestInput.BillingSubjectType = locked.BillingSubjectType
		}
		if ingestInput.BillingSubjectID == "" {
			ingestInput.BillingSubjectID = locked.BillingSubjectID
		}
		if ingestInput.BillableItemCode == "" {
			ingestInput.BillableItemCode = locked.BillableItemCode
		}
		if ingestInput.UsageUnits <= 0 {
			ingestInput.UsageUnits = locked.Units
		}
		persistedEvent, err := s.ingestEventTx(tx, ingestInput)
		if err != nil {
			return err
		}
		now := time.Now()
		locked.Status = platformconst.ReservationStatusFinalized
		locked.FinalizationID = &input.FinalizationID
		locked.CommittedAt = &now
		if err := tx.Save(&locked).Error; err != nil {
			return err
		}
		reservation = &locked
		event = persistedEvent
		return nil
	})
	if err != nil {
		log.Error("metering.finalize.failed", "error", err)
		return nil, err
	}
	var settlement *models.SettlementRecord
	if event != nil {
		if item, err := s.financeRepo.FindSettlementRecordByEventID(event.EventID); err == nil {
			settlement = item
		}
	}
	log.Info(
		"metering.finalize.success",
		"reservation_status", valueOrEmpty(reservation, func(item *models.ResourceReservation) string { return item.Status }),
		"event_id", valueOrEmpty(event, func(item *models.MeterEvent) string { return item.EventID }),
		"settlement_id", valueOrEmpty(settlement, func(item *models.SettlementRecord) string { return item.ID }),
		"settlement_status", valueOrEmpty(settlement, func(item *models.SettlementRecord) string { return item.Status }),
	)
	return &FinalizeResult{
		Reservation: reservation,
		Event:       event,
		Settlement:  settlement,
	}, nil
}

func (s *Service) ingestEventTx(tx *gorm.DB, input IngestEventInput) (*models.MeterEvent, error) {
	if existing, err := s.commercialRepo.FindMeterEventByEventID(input.EventID); err == nil {
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	occurredAt := time.Now()
	if input.OccurredAt != "" {
		if parsed, err := time.Parse(time.RFC3339, input.OccurredAt); err == nil {
			occurredAt = parsed
		}
	}
	billable := true
	if input.Billable != nil {
		billable = *input.Billable
	}
	if input.UsageUnits <= 0 {
		input.UsageUnits = 1
	}
	if input.Unit == "" {
		input.Unit = platformconst.MeterUnitRequest
	}
	if input.EventRole == "" {
		input.EventRole = platformconst.MeterEventRoleEntry
	}
	if input.BillingSubjectType == "" {
		input.BillingSubjectType = platformconst.SubjectTypeOrganization
	}
	if input.BillingSubjectID == "" {
		input.BillingSubjectID = input.OrgID
	}
	event := &models.MeterEvent{
		ID:                 utils.GenerateID(),
		EventID:            input.EventID,
		RequestID:          input.RequestID,
		TraceID:            input.TraceID,
		SourceType:         input.SourceType,
		SourceID:           input.SourceID,
		SourceAction:       input.SourceAction,
		ProductCode:        input.ProductCode,
		OrgID:              input.OrgID,
		UserID:             input.UserID,
		BillableItemCode:   input.BillableItemCode,
		ChargeGroupID:      input.ChargeGroupID,
		ParentEventID:      input.ParentEventID,
		EventRole:          input.EventRole,
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		UsageUnits:         input.UsageUnits,
		Unit:               input.Unit,
		Billable:           billable,
		BillingProfileKey:  input.BillingProfileKey,
		CurrencyContext:    input.CurrencyContext,
		Dimensions:         input.Dimensions,
		OccurredAt:         occurredAt,
		ReceivedAt:         time.Now(),
		Status:             platformconst.StatusAccepted,
	}
	var billingProfileID, commercialEntityID, merchantAccountID string
	if input.BillingProfileKey != "" {
		profile, err := s.commercialRepo.FindBillingProfileByCode(input.BillingProfileKey)
		if err == nil {
			billingProfileID = profile.ID
			commercialEntityID = profile.CommercialEntityID
			merchantAccountID = profile.DefaultMerchantAccountID
		}
	}
	record := &models.UsageRecord{
		ID:                 utils.GenerateID(),
		EventID:            input.EventID,
		RequestID:          input.RequestID,
		TraceID:            input.TraceID,
		ProductCode:        input.ProductCode,
		OrgID:              input.OrgID,
		UserID:             input.UserID,
		BillableItemCode:   input.BillableItemCode,
		ChargeGroupID:      input.ChargeGroupID,
		EventRole:          input.EventRole,
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		UsageUnits:         input.UsageUnits,
		Billable:           billable,
		BillingProfileID:   billingProfileID,
		CommercialEntityID: commercialEntityID,
		MerchantAccountID:  merchantAccountID,
		Dimensions:         input.Dimensions,
		OccurredAt:         occurredAt,
		RecordedAt:         time.Now(),
	}
	agg := &models.UsageAgg{
		ID:               utils.GenerateID(),
		ProductCode:      input.ProductCode,
		OrgID:            input.OrgID,
		BillableItemCode: input.BillableItemCode,
		TimeGranularity:  "day",
		StatTime:         dayStart(occurredAt),
		Dimensions:       input.Dimensions,
		UsageUnits:       input.UsageUnits,
		EventCount:       1,
		BillableUnits:    ternaryInt64(billable, input.UsageUnits, 0),
	}
	if err := tx.Create(event).Error; err != nil {
		return nil, err
	}
	if err := tx.Create(record).Error; err != nil {
		return nil, err
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "product_code"},
			{Name: "org_id"},
			{Name: "billable_item_code"},
			{Name: "time_granularity"},
			{Name: "stat_time"},
			{Name: "dimensions"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"usage_units":    gorm.Expr("usage_aggs.usage_units + EXCLUDED.usage_units"),
			"event_count":    gorm.Expr("usage_aggs.event_count + EXCLUDED.event_count"),
			"billable_units": gorm.Expr("usage_aggs.billable_units + EXCLUDED.billable_units"),
			"updated_at":     time.Now(),
		}),
	}).Create(agg).Error; err != nil {
		return nil, err
	}
	if _, err := s.applySettlement(tx, event, record, input); err != nil {
		return nil, err
	}
	return event, nil
}

func controlReservationInvalid() error {
	return ErrReservationNotFinalizable
}

func valueOrEmpty[T any](item *T, getter func(*T) string) string {
	if item == nil {
		return ""
	}
	return getter(item)
}
