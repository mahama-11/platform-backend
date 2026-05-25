package metering

import (
	"errors"
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
