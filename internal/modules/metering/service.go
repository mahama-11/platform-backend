package metering

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"platform-service/internal/models"
	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/internal/repository"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	commercialRepo *repository.CommercialRepository
	financeRepo    *repository.FinanceRepository
	walletService  *walletmodule.Service
}

type IngestEventInput struct {
	EventID               string `json:"event_id" binding:"required"`
	RequestID             string `json:"request_id"`
	TraceID               string `json:"trace_id"`
	SourceType            string `json:"source_type"`
	SourceID              string `json:"source_id"`
	SourceAction          string `json:"source_action"`
	ProductCode           string `json:"product_code" binding:"required"`
	OrgID                 string `json:"org_id"`
	UserID                string `json:"user_id"`
	BillableItemCode      string `json:"billable_item_code" binding:"required"`
	ChargeGroupID         string `json:"charge_group_id"`
	ParentEventID         string `json:"parent_event_id"`
	EventRole             string `json:"event_role"`
	BillingSubjectType    string `json:"billing_subject_type"`
	BillingSubjectID      string `json:"billing_subject_id"`
	UsageUnits            int64  `json:"usage_units"`
	Unit                  string `json:"unit"`
	Billable              *bool  `json:"billable"`
	BillingProfileKey     string `json:"billing_profile_key"`
	CurrencyContext       string `json:"currency_context"`
	Dimensions            string `json:"dimensions"`
	OccurredAt            string `json:"occurred_at"`
	DiscountType          string `json:"discount_type"`
	DiscountAmount        int64  `json:"discount_amount"`
	CampaignCode          string `json:"campaign_code"`
	RewardAmount          int64  `json:"reward_amount"`
	RewardAssetCode       string `json:"reward_asset_code"`
	RewardSubjectType     string `json:"reward_subject_type"`
	RewardSubjectID       string `json:"reward_subject_id"`
	ReferralCode          string `json:"referral_code"`
	CommissionAmount      int64  `json:"commission_amount"`
	CommissionType        string `json:"commission_type"`
	CommissionSubjectType string `json:"commission_subject_type"`
	CommissionSubjectID   string `json:"commission_subject_id"`
}

type UsageSummary struct {
	ProductCode      string `json:"product_code"`
	OrgID            string `json:"org_id"`
	BillableItemCode string `json:"billable_item_code"`
	UsageUnits       int64  `json:"usage_units"`
	EventCount       int64  `json:"event_count"`
	BillableUnits    int64  `json:"billable_units"`
}

type ReverseSettlementInput struct {
	Reason   string `json:"reason"`
	Metadata string `json:"metadata"`
}

type priceConfig struct {
	UnitAmount int64 `json:"unit_amount"`
}

type settlementResult struct {
	Mode             string
	Currency         string
	GrossAmount      int64
	DiscountAmount   int64
	NetAmount        int64
	QuotaConsumed    int64
	CreditsConsumed  int64
	WalletDebited    int64
	BilledAmount     int64
	WalletAssetCode  string
	WalletDebits     []walletmodule.DebitBreakdown
	RewardAmount     int64
	CommissionAmount int64
}

var (
	ErrInsufficientQuotaBalance   = errors.New("insufficient quota balance")
	ErrInsufficientCreditsBalance = errors.New("insufficient credits balance")
	ErrInsufficientWalletBalance  = errors.New("insufficient wallet balance")
	ErrSettlementAlreadyReversed  = errors.New("settlement already reversed")
)

func NewService(
	commercialRepo *repository.CommercialRepository,
	financeRepo *repository.FinanceRepository,
	walletService *walletmodule.Service,
) *Service {
	return &Service{
		commercialRepo: commercialRepo,
		financeRepo:    financeRepo,
		walletService:  walletService,
	}
}

func (s *Service) IngestEvent(input IngestEventInput) (*models.MeterEvent, error) {
	log := logger.With(
		"request_id", input.RequestID,
		"trace_id", input.TraceID,
		"event_id", input.EventID,
		"product_code", input.ProductCode,
		"org_id", input.OrgID,
		"billable_item_code", input.BillableItemCode,
	)
	log.Info("metering.ingest.begin")
	if existing, err := s.commercialRepo.FindMeterEventByEventID(input.EventID); err == nil {
		log.Info("metering.ingest.duplicate")
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("metering.ingest.lookup_failed", "error", err)
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

	var billingProfileID string
	var commercialEntityID string
	var merchantAccountID string
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

	var settlement settlementResult
	err := s.commercialRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		if err := tx.Create(record).Error; err != nil {
			return err
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
			return err
		}
		result, err := s.applySettlement(tx, event, record, input)
		if err != nil {
			return err
		}
		settlement = result
		return nil
	})
	if err != nil {
		log.Error("metering.ingest.persist_failed", "error", err)
		return nil, err
	}
	log.Info(
		"metering.ingest.persisted",
		"billable", billable,
		"usage_units", input.UsageUnits,
		"billing_profile_key", input.BillingProfileKey,
		"settlement_mode", settlement.Mode,
		"gross_amount", settlement.GrossAmount,
		"discount_amount", settlement.DiscountAmount,
		"net_amount", settlement.NetAmount,
		"quota_consumed", settlement.QuotaConsumed,
		"credits_consumed", settlement.CreditsConsumed,
		"wallet_debited", settlement.WalletDebited,
		"wallet_asset_code", settlement.WalletAssetCode,
		"billed_amount", settlement.BilledAmount,
		"reward_amount", settlement.RewardAmount,
		"commission_amount", settlement.CommissionAmount,
	)
	return event, nil
}

func (s *Service) UsageSummary(orgID, productCode string) ([]UsageSummary, error) {
	var rows []UsageSummary
	q := s.commercialRepo.DB().Model(&models.UsageAgg{}).
		Select("product_code, org_id, billable_item_code, SUM(usage_units) as usage_units, SUM(event_count) as event_count, SUM(billable_units) as billable_units").
		Group("product_code, org_id, billable_item_code")
	if orgID != "" {
		q = q.Where("org_id = ?", orgID)
	}
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	err := q.Scan(&rows).Error
	return rows, err
}

func (s *Service) GetSettlementRecord(eventID string) (*models.SettlementRecord, error) {
	return s.financeRepo.FindSettlementRecordByEventID(eventID)
}

func (s *Service) ListSettlementRecords(subjectType, subjectID, productCode string) ([]models.SettlementRecord, error) {
	return s.financeRepo.ListSettlementRecords(subjectType, subjectID, productCode)
}

func (s *Service) ListDiscountLedgers(productCode, subjectType, subjectID string) ([]models.DiscountLedger, error) {
	return s.financeRepo.ListDiscountLedgers(productCode, subjectType, subjectID)
}

func (s *Service) ReverseSettlement(eventID string, input ReverseSettlementInput) (*models.SettlementRecord, error) {
	log := logger.With("event_id", eventID, "reason", input.Reason)
	log.Info("metering.reverse.begin")
	err := s.commercialRepo.DB().Transaction(func(tx *gorm.DB) error {
		var settlement models.SettlementRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("event_id = ?", eventID).
			First(&settlement).Error; err != nil {
			return err
		}
		if settlement.Status == platformconst.SettlementStatusReversed {
			return ErrSettlementAlreadyReversed
		}

		reversalRef := "reverse:" + eventID
		now := time.Now()

		if settlement.QuotaConsumed > 0 {
			if err := tx.Create(&models.QuotaLedger{
				ID:                 utils.GenerateID(),
				BillingSubjectType: settlement.BillingSubjectType,
				BillingSubjectID:   settlement.BillingSubjectID,
				BillableItemCode:   settlement.BillableItemCode,
				Direction:          platformconst.LedgerDirectionRefund,
				Units:              settlement.QuotaConsumed,
				Reason:             firstNonEmpty(input.Reason, "settlement_reverse"),
				ReferenceID:        reversalRef,
				CreatedAt:          now,
			}).Error; err != nil {
				return err
			}
		}

		walletDebits, parseErr := parseWalletDebitsFromSnapshot(settlement.Snapshot)
		if parseErr != nil {
			return parseErr
		}
		if len(walletDebits) > 0 {
			for _, debit := range walletDebits {
				if debit.Amount <= 0 || debit.AssetCode == "" {
					continue
				}
				if _, err := s.postWalletChange(
					tx,
					settlement.BillingSubjectType,
					settlement.BillingSubjectID,
					debit.AssetCode,
					"settlement_reverse",
					platformconst.LedgerDirectionCredit,
					debit.Amount,
					"settlement_record",
					settlement.ID,
					input.Metadata,
				); err != nil {
					return err
				}
			}
		} else if settlement.WalletDebited > 0 && settlement.WalletAssetCode != "" {
			if _, err := s.postWalletChange(
				tx,
				settlement.BillingSubjectType,
				settlement.BillingSubjectID,
				settlement.WalletAssetCode,
				"settlement_reverse",
				platformconst.LedgerDirectionCredit,
				settlement.WalletDebited,
				"settlement_record",
				settlement.ID,
				input.Metadata,
			); err != nil {
				return err
			}
		} else if settlement.CreditsConsumed > 0 {
			if err := tx.Create(&models.CreditsLedger{
				ID:                 utils.GenerateID(),
				BillingSubjectType: settlement.BillingSubjectType,
				BillingSubjectID:   settlement.BillingSubjectID,
				Direction:          platformconst.LedgerDirectionRefund,
				Amount:             settlement.CreditsConsumed,
				Reason:             firstNonEmpty(input.Reason, "settlement_reverse"),
				ReferenceID:        reversalRef,
				CreatedAt:          now,
			}).Error; err != nil {
				return err
			}
		}

		if settlement.BillingAmount > 0 {
			if err := tx.Create(&models.BillingLedger{
				ID:                 utils.GenerateID(),
				BillingSubjectType: settlement.BillingSubjectType,
				BillingSubjectID:   settlement.BillingSubjectID,
				ProductCode:        settlement.ProductCode,
				BillableItemCode:   settlement.BillableItemCode,
				Currency:           settlement.Currency,
				Amount:             settlement.BillingAmount,
				Direction:          platformconst.LedgerDirectionCredit,
				Status:             platformconst.BillingLedgerStatusBooked,
				ReferenceID:        reversalRef,
				OccurredAt:         now,
				CreatedAt:          now,
				UpdatedAt:          now,
			}).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&models.DiscountLedger{}).
			Where("reference_type = ? AND reference_id = ? AND status <> ?", "meter_event", eventID, platformconst.SettlementStatusReversed).
			Updates(map[string]any{
				"status":     platformconst.SettlementStatusReversed,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		if err := s.reverseRewards(tx, eventID, input, now); err != nil {
			return err
		}
		if err := s.reverseCommissions(tx, eventID, now); err != nil {
			return err
		}

		snapshot, err := s.buildReversalSnapshot(settlement.Snapshot, input, reversalRef, now)
		if err != nil {
			return err
		}
		settlement.Status = platformconst.SettlementStatusReversed
		settlement.Snapshot = snapshot
		settlement.UpdatedAt = now
		return tx.Save(&settlement).Error
	})
	if err != nil {
		log.Error("metering.reverse.failed", "error", err)
		return nil, err
	}
	item, err := s.financeRepo.FindSettlementRecordByEventID(eventID)
	if err != nil {
		log.Error("metering.reverse.lookup_failed", "error", err)
		return nil, err
	}
	log.Info("metering.reverse.success", "settlement_id", item.ID, "status", item.Status)
	return item, nil
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func ternaryInt64(cond bool, t, f int64) int64 {
	if cond {
		return t
	}
	return f
}

func (s *Service) applySettlement(tx *gorm.DB, event *models.MeterEvent, record *models.UsageRecord, input IngestEventInput) (settlementResult, error) {
	result := settlementResult{Mode: "none"}
	if !event.Billable {
		return s.finalizeSettlement(tx, event, record, input, result)
	}
	billableItem, err := s.findBillableItemByCode(tx, event.BillableItemCode)
	if err != nil {
		result.Mode = "skipped_missing_billable_item"
		return s.finalizeSettlement(tx, event, record, input, result)
	}
	if billableItem.PricingBehavior == "child_non_billable" || event.EventRole == "child" {
		result.Mode = "skipped_child_non_billable"
		return s.finalizeSettlement(tx, event, record, input, result)
	}
	switch billableItem.SettlementMode {
	case platformconst.SettlementModeQuota:
		if err := s.consumeQuota(tx, record.BillingSubjectType, record.BillingSubjectID, record.BillableItemCode, record.UsageUnits, event.EventID, "metering_settlement"); err != nil {
			return result, err
		}
		result.Mode = platformconst.SettlementModeQuota
		result.QuotaConsumed = record.UsageUnits
		return s.finalizeSettlement(tx, event, record, input, result)
	case platformconst.SettlementModeCredits:
		debited, debitedAssetCode, breakdown, err := s.walletService.DebitByPriorityTx(
			tx,
			record.BillingSubjectType,
			record.BillingSubjectID,
			record.ProductCode,
			event.CurrencyContext,
			record.UsageUnits,
			"metering_settlement",
			"meter_event",
			event.EventID,
			record.Dimensions,
		)
		if err != nil {
			if errors.Is(err, walletmodule.ErrInsufficientWalletBalance) {
				return result, ErrInsufficientCreditsBalance
			}
			return result, err
		}
		if debited != record.UsageUnits {
			return result, ErrInsufficientCreditsBalance
		}
		result.Mode = platformconst.SettlementModeCredits
		result.CreditsConsumed = debited
		result.WalletAssetCode = debitedAssetCode
		result.WalletDebits = breakdown
		return s.finalizeSettlement(tx, event, record, input, result)
	case platformconst.SettlementModeIncludedThenOverage:
		result.Mode = platformconst.SettlementModeIncludedThenOverage
		availableQuota, err := s.quotaAvailable(tx, record.BillingSubjectType, record.BillingSubjectID, record.BillableItemCode)
		if err != nil {
			return result, err
		}
		quotaUnits := minInt64(record.UsageUnits, maxInt64(availableQuota, 0))
		if quotaUnits > 0 {
			if createErr := s.createQuotaConsumeLedger(tx, record.BillingSubjectType, record.BillingSubjectID, record.BillableItemCode, quotaUnits, event.EventID, "metering_settlement"); createErr != nil {
				return result, createErr
			}
			result.QuotaConsumed = quotaUnits
		}
		remainingUnits := record.UsageUnits - quotaUnits
		if remainingUnits <= 0 {
			return s.finalizeSettlement(tx, event, record, input, result)
		}
		billingResult, err := s.settleUsageBilling(tx, event, record, billableItem, input, remainingUnits)
		if err != nil {
			return result, err
		}
		result.Currency = billingResult.Currency
		result.GrossAmount = billingResult.GrossAmount
		result.DiscountAmount = billingResult.DiscountAmount
		result.NetAmount = billingResult.NetAmount
		result.WalletDebited = billingResult.WalletDebited
		result.BilledAmount = billingResult.BilledAmount
		result.WalletAssetCode = billingResult.WalletAssetCode
		result.WalletDebits = billingResult.WalletDebits
		return s.finalizeSettlement(tx, event, record, input, result)
	case platformconst.SettlementModeUsageBilling, "":
		billingResult, err := s.settleUsageBilling(tx, event, record, billableItem, input, record.UsageUnits)
		if err != nil {
			return result, err
		}
		billingResult.Mode = platformconst.SettlementModeUsageBilling
		return s.finalizeSettlement(tx, event, record, input, billingResult)
	default:
		result.Mode = "skipped_unknown_settlement_mode"
		return s.finalizeSettlement(tx, event, record, input, result)
	}
}

func (s *Service) settleUsageBilling(
	tx *gorm.DB,
	event *models.MeterEvent,
	record *models.UsageRecord,
	billableItem *models.BillableItem,
	input IngestEventInput,
	usageUnits int64,
) (settlementResult, error) {
	result := settlementResult{Mode: platformconst.SettlementModeUsageBilling}
	rateCard, err := s.findActiveRateCard(tx, "billable_item", billableItem.ID, event.OccurredAt)
	if err != nil {
		result.Mode = "skipped_missing_rate_card"
		return result, nil
	}
	if rateCard.PriceModel != "flat" {
		result.Mode = "skipped_unsupported_price_model"
		return result, nil
	}
	var cfg priceConfig
	if err := json.Unmarshal([]byte(rateCard.PriceConfig), &cfg); err != nil {
		result.Mode = "skipped_invalid_price_config"
		return result, nil
	}
	if cfg.UnitAmount <= 0 {
		result.Mode = "skipped_zero_priced"
		return result, nil
	}
	totalAmount := cfg.UnitAmount * usageUnits
	discountAmount := minInt64(maxInt64(input.DiscountAmount, 0), totalAmount)
	netAmount := totalAmount - discountAmount
	remainingAmount := netAmount
	result.Currency = rateCard.Currency
	result.GrossAmount = totalAmount
	result.DiscountAmount = discountAmount
	result.NetAmount = netAmount
	assetCode := firstNonEmpty(event.CurrencyContext, rateCard.Currency)
	if assetCode != "" {
		debited, debitedAssetCode, breakdown, err := s.walletService.DebitByPriorityTx(
			tx,
			record.BillingSubjectType,
			record.BillingSubjectID,
			record.ProductCode,
			assetCode,
			netAmount,
			"metering_settlement",
			"meter_event",
			event.EventID,
			record.Dimensions,
		)
		if err != nil {
			if errors.Is(err, walletmodule.ErrInsufficientWalletBalance) {
				err = nil
			} else {
				return result, err
			}
		}
		result.WalletDebited = debited
		result.WalletAssetCode = debitedAssetCode
		result.WalletDebits = breakdown
		remainingAmount -= debited
	}
	if remainingAmount <= 0 {
		return result, nil
	}
	ledger := &models.BillingLedger{
		ID:                 utils.GenerateID(),
		BillingSubjectType: record.BillingSubjectType,
		BillingSubjectID:   record.BillingSubjectID,
		ProductCode:        record.ProductCode,
		BillableItemCode:   record.BillableItemCode,
		Currency:           rateCard.Currency,
		Amount:             remainingAmount,
		Direction:          platformconst.LedgerDirectionDebit,
		Status:             platformconst.BillingLedgerStatusBooked,
		ReferenceID:        event.EventID,
		OccurredAt:         event.OccurredAt,
	}
	if err := tx.Create(ledger).Error; err != nil {
		return result, err
	}
	result.BilledAmount = remainingAmount
	return result, nil
}

func (s *Service) finalizeSettlement(
	tx *gorm.DB,
	event *models.MeterEvent,
	record *models.UsageRecord,
	input IngestEventInput,
	result settlementResult,
) (settlementResult, error) {
	result.RewardAmount = maxInt64(input.RewardAmount, 0)
	result.CommissionAmount = maxInt64(input.CommissionAmount, 0)

	if result.GrossAmount == 0 && result.DiscountAmount > 0 {
		result.NetAmount = maxInt64(0, result.GrossAmount-result.DiscountAmount)
	}

	if result.DiscountAmount > 0 {
		if err := tx.Create(&models.DiscountLedger{
			ID:                 utils.GenerateID(),
			ProductCode:        record.ProductCode,
			CampaignCode:       input.CampaignCode,
			DiscountType:       firstNonEmpty(input.DiscountType, "promotion"),
			BillingSubjectType: record.BillingSubjectType,
			BillingSubjectID:   record.BillingSubjectID,
			Currency:           result.Currency,
			Amount:             result.DiscountAmount,
			Status:             platformconst.DiscountLedgerStatusApplied,
			ReferenceType:      "meter_event",
			ReferenceID:        event.EventID,
			Metadata:           record.Dimensions,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}).Error; err != nil {
			return result, err
		}
	}

	if result.RewardAmount > 0 {
		if err := s.issueReward(tx, event, record, input, result); err != nil {
			return result, err
		}
	}

	if result.CommissionAmount > 0 {
		if err := tx.Create(&models.CommissionLedger{
			ID:                     utils.GenerateID(),
			ProductCode:            record.ProductCode,
			CommissionType:         firstNonEmpty(input.CommissionType, "referral"),
			BeneficiarySubjectType: firstNonEmpty(input.CommissionSubjectType, record.BillingSubjectType),
			BeneficiarySubjectID:   firstNonEmpty(input.CommissionSubjectID, record.BillingSubjectID),
			SettlementSubjectType:  record.BillingSubjectType,
			SettlementSubjectID:    record.BillingSubjectID,
			Currency:               result.Currency,
			Amount:                 result.CommissionAmount,
			Status:                 platformconst.CommissionStatusEarned,
			ReferenceType:          "meter_event",
			ReferenceID:            event.EventID,
			Metadata:               record.Dimensions,
			CreatedAt:              time.Now(),
			UpdatedAt:              time.Now(),
		}).Error; err != nil {
			return result, err
		}
	}
	if result.CommissionAmount == 0 && input.ReferralCode != "" {
		commissionAmount, err := s.issueReferralCommission(tx, event, record, input, result)
		if err != nil {
			return result, err
		}
		result.CommissionAmount = commissionAmount
	}

	snapshot, err := json.Marshal(map[string]any{
		"mode":              result.Mode,
		"currency":          result.Currency,
		"gross_amount":      result.GrossAmount,
		"discount_amount":   result.DiscountAmount,
		"net_amount":        result.NetAmount,
		"quota_consumed":    result.QuotaConsumed,
		"credits_consumed":  result.CreditsConsumed,
		"wallet_asset_code": result.WalletAssetCode,
		"wallet_debited":    result.WalletDebited,
		"wallet_debits":     result.WalletDebits,
		"billing_amount":    result.BilledAmount,
		"reward_amount":     result.RewardAmount,
		"commission_amount": result.CommissionAmount,
		"campaign_code":     input.CampaignCode,
		"discount_type":     input.DiscountType,
		"commission_type":   input.CommissionType,
	})
	if err != nil {
		return result, err
	}

	if err := tx.Create(&models.SettlementRecord{
		ID:                 utils.GenerateID(),
		EventID:            event.EventID,
		RequestID:          event.RequestID,
		TraceID:            event.TraceID,
		BillingSubjectType: record.BillingSubjectType,
		BillingSubjectID:   record.BillingSubjectID,
		ProductCode:        record.ProductCode,
		BillableItemCode:   record.BillableItemCode,
		BillingProfileID:   record.BillingProfileID,
		CommercialEntityID: record.CommercialEntityID,
		MerchantAccountID:  record.MerchantAccountID,
		SettlementMode:     result.Mode,
		Currency:           result.Currency,
		GrossAmount:        result.GrossAmount,
		DiscountAmount:     result.DiscountAmount,
		NetAmount:          result.NetAmount,
		QuotaConsumed:      result.QuotaConsumed,
		CreditsConsumed:    result.CreditsConsumed,
		WalletAssetCode:    result.WalletAssetCode,
		WalletDebited:      result.WalletDebited,
		BillingAmount:      result.BilledAmount,
		RewardAmount:       result.RewardAmount,
		CommissionAmount:   result.CommissionAmount,
		Status:             platformconst.SettlementStatusSettled,
		Snapshot:           string(snapshot),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}).Error; err != nil {
		return result, err
	}

	return result, nil
}

func (s *Service) issueReward(
	tx *gorm.DB,
	event *models.MeterEvent,
	record *models.UsageRecord,
	input IngestEventInput,
	result settlementResult,
) error {
	beneficiaryType := firstNonEmpty(input.RewardSubjectType, record.BillingSubjectType)
	beneficiaryID := firstNonEmpty(input.RewardSubjectID, record.BillingSubjectID)
	assetCode := firstNonEmpty(input.RewardAssetCode, result.WalletAssetCode, result.Currency)

	item := &models.RewardLedger{
		ID:                     utils.GenerateID(),
		ProductCode:            record.ProductCode,
		CampaignCode:           input.CampaignCode,
		RewardType:             "usage_reward",
		BeneficiarySubjectType: beneficiaryType,
		BeneficiarySubjectID:   beneficiaryID,
		AssetCode:              assetCode,
		Amount:                 result.RewardAmount,
		Status:                 platformconst.RewardStatusIssued,
		ReferenceType:          "meter_event",
		ReferenceID:            event.EventID,
		Metadata:               record.Dimensions,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}
	if err := tx.Create(item).Error; err != nil {
		return err
	}
	if assetCode == "" {
		return nil
	}

	var account models.WalletAccount
	if err := tx.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", beneficiaryType, beneficiaryID, assetCode).First(&account).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		account = models.WalletAccount{
			ID:                 utils.GenerateID(),
			BillingSubjectType: beneficiaryType,
			BillingSubjectID:   beneficiaryID,
			AssetCode:          assetCode,
			AssetType:          platformconst.WalletAssetTypeRewardCredit,
			Status:             platformconst.StatusActive,
		}
		if err := tx.Create(&account).Error; err != nil {
			return err
		}
	}

	if err := tx.Create(&models.WalletLedger{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		Direction:          platformconst.LedgerDirectionCredit,
		Amount:             result.RewardAmount,
		Reason:             "reward_issue",
		ReferenceType:      "reward_ledger",
		ReferenceID:        item.ID,
		Status:             platformconst.WalletLedgerStatusPosted,
		CreatedAt:          time.Now(),
	}).Error; err != nil {
		return err
	}
	account.Balance += result.RewardAmount
	return tx.Save(&account).Error
}

func (s *Service) findBillableItemByCode(tx *gorm.DB, code string) (*models.BillableItem, error) {
	var item models.BillableItem
	if err := tx.Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) findActiveRateCard(tx *gorm.DB, targetType, targetID string, at any) (*models.RateCard, error) {
	var item models.RateCard
	q := tx.Where("target_type = ? AND target_id = ? AND status = ?", targetType, targetID, platformconst.StatusActive).
		Order("version desc, created_at desc")
	if at != nil {
		q = q.Where("(effective_from IS NULL OR effective_from <= ?) AND (effective_to IS NULL OR effective_to >= ?)", at, at)
	}
	if err := q.First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) quotaAvailable(tx *gorm.DB, subjectType, subjectID, billableItemCode string) (int64, error) {
	granted, err := s.sumQuotaDirection(tx, subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionGrant)
	if err != nil {
		return 0, err
	}
	consumed, err := s.sumQuotaDirection(tx, subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionConsume)
	if err != nil {
		return 0, err
	}
	refunded, err := s.sumQuotaDirection(tx, subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionRefund)
	if err != nil {
		return 0, err
	}
	reserved, err := s.sumReserved(tx, platformconst.ResourceTypeQuota, subjectType, subjectID, billableItemCode)
	if err != nil {
		return 0, err
	}
	return granted + refunded - consumed - reserved, nil
}

func (s *Service) consumeQuota(tx *gorm.DB, subjectType, subjectID, billableItemCode string, units int64, referenceID, reason string) error {
	available, err := s.quotaAvailable(tx, subjectType, subjectID, billableItemCode)
	if err != nil {
		return err
	}
	if available < units {
		return ErrInsufficientQuotaBalance
	}
	return s.createQuotaConsumeLedger(tx, subjectType, subjectID, billableItemCode, units, referenceID, reason)
}

func (s *Service) createQuotaConsumeLedger(tx *gorm.DB, subjectType, subjectID, billableItemCode string, units int64, referenceID, reason string) error {
	return tx.Create(&models.QuotaLedger{
		ID:                 utils.GenerateID(),
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		BillableItemCode:   billableItemCode,
		Direction:          platformconst.LedgerDirectionConsume,
		Units:              units,
		Reason:             reason,
		ReferenceID:        referenceID,
		CreatedAt:          time.Now(),
	}).Error
}

func parseWalletDebitsFromSnapshot(snapshot string) ([]walletmodule.DebitBreakdown, error) {
	if strings.TrimSpace(snapshot) == "" {
		return nil, nil
	}
	var payload struct {
		WalletDebits []walletmodule.DebitBreakdown `json:"wallet_debits"`
	}
	if err := json.Unmarshal([]byte(snapshot), &payload); err != nil {
		return nil, err
	}
	return payload.WalletDebits, nil
}

func (s *Service) postWalletChange(
	tx *gorm.DB,
	subjectType, subjectID, assetCode, assetType, direction string,
	amount int64,
	referenceType, referenceID, metadata string,
) (*models.WalletAccount, error) {
	if amount <= 0 || assetCode == "" {
		return nil, nil
	}

	account, err := s.findOrCreateWalletAccount(tx, subjectType, subjectID, assetCode, assetType)
	if err != nil {
		return nil, err
	}
	if direction == "debit" && account.Balance < amount {
		return nil, ErrInsufficientWalletBalance
	}

	if err := tx.Create(&models.WalletLedger{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		Direction:          direction,
		Amount:             amount,
		Reason:             assetType,
		ReferenceType:      referenceType,
		ReferenceID:        referenceID,
		Status:             "posted",
		Metadata:           metadata,
		CreatedAt:          time.Now(),
	}).Error; err != nil {
		return nil, err
	}

	if direction == "debit" {
		account.Balance -= amount
	} else {
		account.Balance += amount
	}
	if err := tx.Save(account).Error; err != nil {
		return nil, err
	}
	return account, nil
}

func (s *Service) findOrCreateWalletAccount(tx *gorm.DB, subjectType, subjectID, assetCode, assetType string) (*models.WalletAccount, error) {
	var account models.WalletAccount
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", subjectType, subjectID, assetCode).
		First(&account).Error
	if err == nil {
		return &account, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	account = models.WalletAccount{
		ID:                 utils.GenerateID(),
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		AssetCode:          assetCode,
		AssetType:          firstNonEmpty(assetType, "wallet_credit"),
		Status:             "active",
	}
	if err := tx.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *Service) reverseRewards(tx *gorm.DB, eventID string, input ReverseSettlementInput, now time.Time) error {
	var rewards []models.RewardLedger
	if err := tx.Where("reference_type = ? AND reference_id = ? AND status <> ?", "meter_event", eventID, "reversed").Find(&rewards).Error; err != nil {
		return err
	}
	for i := range rewards {
		reward := &rewards[i]
		if reward.AssetCode != "" && reward.Amount > 0 {
			if _, err := s.postWalletChange(
				tx,
				reward.BeneficiarySubjectType,
				reward.BeneficiarySubjectID,
				reward.AssetCode,
				"reward_reverse",
				"debit",
				reward.Amount,
				"reward_ledger",
				reward.ID,
				input.Metadata,
			); err != nil {
				return err
			}
		}
		reward.Status = "reversed"
		reward.UpdatedAt = now
		if err := tx.Save(reward).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reverseCommissions(tx *gorm.DB, eventID string, now time.Time) error {
	if err := tx.Model(&models.CommissionLedger{}).
		Where("reference_type = ? AND reference_id = ? AND status <> ?", "meter_event", eventID, "reversed").
		Updates(map[string]any{
			"status":     "reversed",
			"updated_at": now,
		}).Error; err != nil {
		return err
	}
	return tx.Model(&models.ReferralConversion{}).
		Where("reference_type = ? AND reference_id = ? AND status <> ?", "meter_event", eventID, "reversed").
		Updates(map[string]any{
			"status":     "reversed",
			"updated_at": now,
		}).Error
}

func (s *Service) issueReferralCommission(
	tx *gorm.DB,
	event *models.MeterEvent,
	record *models.UsageRecord,
	input IngestEventInput,
	result settlementResult,
) (int64, error) {
	referralCodeValue := strings.ToUpper(strings.TrimSpace(input.ReferralCode))
	if referralCodeValue == "" {
		return 0, nil
	}

	var referralCode models.ReferralCode
	if err := tx.Where("code = ?", referralCodeValue).First(&referralCode).Error; err != nil {
		return 0, err
	}
	if referralCode.Status != "active" {
		return 0, nil
	}

	var program models.ReferralProgram
	if err := tx.Where("id = ?", referralCode.ProgramID).First(&program).Error; err != nil {
		return 0, err
	}
	now := time.Now()
	if program.Status != "active" || !referralProgramActive(program, now) || program.TriggerType != "usage_settlement" {
		return 0, nil
	}
	if program.ProductCode != "" && program.ProductCode != record.ProductCode {
		return 0, nil
	}

	commissionBaseAmount := maxInt64(result.NetAmount, maxInt64(result.BilledAmount, result.GrossAmount))
	commissionAmount := computeReferralCommission(program, commissionBaseAmount)
	currency := firstNonEmpty(program.CommissionCurrency, result.Currency, "CNY")
	conversionStatus := "tracked"
	commissionLedgerID := ""
	if commissionAmount > 0 {
		ledger := &models.CommissionLedger{
			ID:                     utils.GenerateID(),
			ProductCode:            record.ProductCode,
			CommissionType:         firstNonEmpty(program.TriggerType, "referral"),
			BeneficiarySubjectType: referralCode.PromoterSubjectType,
			BeneficiarySubjectID:   referralCode.PromoterSubjectID,
			SettlementSubjectType:  record.BillingSubjectType,
			SettlementSubjectID:    record.BillingSubjectID,
			Currency:               currency,
			Amount:                 commissionAmount,
			Status:                 "earned",
			ReferenceType:          "meter_event",
			ReferenceID:            event.EventID,
			Metadata:               record.Dimensions,
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		if err := tx.Create(ledger).Error; err != nil {
			return 0, err
		}
		commissionLedgerID = ledger.ID
		conversionStatus = "commission_earned"
	}

	conversion := &models.ReferralConversion{
		ID:                    utils.GenerateID(),
		ProgramID:             program.ID,
		ReferralCodeID:        referralCode.ID,
		ProductCode:           record.ProductCode,
		TriggerType:           "usage_settlement",
		PromoterSubjectType:   referralCode.PromoterSubjectType,
		PromoterSubjectID:     referralCode.PromoterSubjectID,
		ReferredSubjectType:   record.BillingSubjectType,
		ReferredSubjectID:     record.BillingSubjectID,
		SettlementSubjectType: record.BillingSubjectType,
		SettlementSubjectID:   record.BillingSubjectID,
		ReferenceType:         "meter_event",
		ReferenceID:           event.EventID,
		CommissionCurrency:    currency,
		CommissionAmount:      commissionAmount,
		CommissionLedgerID:    commissionLedgerID,
		Status:                conversionStatus,
		Metadata:              record.Dimensions,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := tx.Create(conversion).Error; err != nil {
		return 0, err
	}
	return commissionAmount, nil
}

func referralProgramActive(program models.ReferralProgram, now time.Time) bool {
	if program.EffectiveFrom != nil && now.Before(*program.EffectiveFrom) {
		return false
	}
	if program.EffectiveTo != nil && now.After(*program.EffectiveTo) {
		return false
	}
	return true
}

func computeReferralCommission(program models.ReferralProgram, baseAmount int64) int64 {
	switch program.CommissionPolicy {
	case "fixed_amount":
		return maxInt64(program.CommissionFixedAmount, 0)
	case "percentage":
		if baseAmount <= 0 {
			return 0
		}
		return maxInt64(baseAmount*program.CommissionRateBps/10000, 0)
	default:
		return 0
	}
}

func (s *Service) buildReversalSnapshot(original string, input ReverseSettlementInput, reversalRef string, now time.Time) (string, error) {
	snapshot := map[string]any{}
	if original != "" {
		if err := json.Unmarshal([]byte(original), &snapshot); err != nil {
			snapshot["original_snapshot_raw"] = original
		}
	}
	snapshot["reversal"] = map[string]any{
		"reference_id": reversalRef,
		"reason":       input.Reason,
		"metadata":     input.Metadata,
		"reversed_at":  now.Format(time.RFC3339),
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Service) sumQuotaDirection(tx *gorm.DB, subjectType, subjectID, billableItemCode, direction string) (int64, error) {
	var total int64
	err := tx.Model(&models.QuotaLedger{}).
		Select("COALESCE(SUM(units), 0)").
		Where("billing_subject_type = ? AND billing_subject_id = ? AND billable_item_code = ? AND direction = ?", subjectType, subjectID, billableItemCode, direction).
		Scan(&total).Error
	return total, err
}

func (s *Service) sumReserved(tx *gorm.DB, resourceType, subjectType, subjectID, billableItemCode string) (int64, error) {
	q := tx.Model(&models.ResourceReservation{}).
		Select("COALESCE(SUM(units), 0)").
		Where("resource_type = ? AND billing_subject_type = ? AND billing_subject_id = ? AND status = ?", resourceType, subjectType, subjectID, "reserved")
	if billableItemCode != "" {
		q = q.Where("billable_item_code = ?", billableItemCode)
	}
	var total int64
	err := q.Scan(&total).Error
	return total, err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
