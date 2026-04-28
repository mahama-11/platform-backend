package incentive

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateChannelBinding_BlocksSecondActiveBinding(t *testing.T) {
	service := newChannelTestService(t)
	partnerA := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-a", "PARTNER_A")
	partnerB := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-b", "PARTNER_B")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-a", "PROGRAM_A")

	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-1",
		ChannelPartnerID: partnerA.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("first CreateChannelBinding() error = %v", err)
	}

	_, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-1",
		ChannelPartnerID: partnerB.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "manual_assign",
	})
	if err != ErrChannelBindingExists {
		t.Fatalf("expected ErrChannelBindingExists, got %v", err)
	}
}

func TestRecordChannelCharge_AndRefundReverse(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-charge", "PARTNER_CHARGE")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-charge", "PROGRAM_CHARGE")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-charge",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_CHARGE",
		Status:           "active",
		AppliesTo:        "wallet_recharge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     3333,
		CooldownDays:     0,
		SettlementCycle:  "monthly",
		HoldbackRateBps:  0,
		Priority:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-charge",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	recorded, err := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:            "evt_charge_1",
		ProductCode:        "menu_ai",
		OrgID:              "org-charge",
		AppliesTo:          "wallet_recharge",
		SourceChargeID:     "charge-1",
		SourceOrderID:      "order-1",
		Currency:           "CNY",
		PaidAmount:         30000,
		NetCollectedAmount: 30000,
	})
	if err != nil {
		t.Fatalf("RecordChannelCharge() error = %v", err)
	}
	if !recorded.Matched || recorded.Ledger == nil {
		t.Fatalf("expected matched ledger, got %+v", recorded)
	}
	if recorded.Ledger.CommissionAmount != 9999 || recorded.Ledger.Status != "earned" {
		t.Fatalf("unexpected ledger after charge: %+v", recorded.Ledger)
	}

	refunded, err := service.RecordChannelRefund(RecordChannelRefundInput{
		EventID:        "evt_refund_1",
		ProductCode:    "menu_ai",
		SourceChargeID: "charge-1",
		SourceRefundID: "refund-1",
		RefundType:     "full_refund",
	})
	if err != nil {
		t.Fatalf("RecordChannelRefund() error = %v", err)
	}
	if !refunded.Matched || refunded.Action != "reversed" || refunded.Ledger == nil {
		t.Fatalf("unexpected refund result: %+v", refunded)
	}
	if refunded.Ledger.Status != "reversed" || refunded.Ledger.ReversalEventID == nil || *refunded.Ledger.ReversalEventID != "evt_refund_1" {
		t.Fatalf("unexpected reversed ledger: %+v", refunded.Ledger)
	}
}

func TestRecordChannelCharge_IsIdempotentByEventAndCharge(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-idem", "PARTNER_IDEM")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-idem", "PROGRAM_IDEM")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-idem",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_IDEM",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1000,
		CooldownDays:     7,
		SettlementCycle:  "monthly",
		HoldbackRateBps:  1000,
		Priority:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-idem",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	first, err := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:            "evt_idem_1",
		ProductCode:        "menu_ai",
		OrgID:              "org-idem",
		AppliesTo:          "usage_charge",
		SourceChargeID:     "charge-idem-1",
		Currency:           "CNY",
		PaidAmount:         10000,
		NetCollectedAmount: 10000,
	})
	if err != nil {
		t.Fatalf("first RecordChannelCharge() error = %v", err)
	}
	second, err := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:            "evt_idem_1",
		ProductCode:        "menu_ai",
		OrgID:              "org-idem",
		AppliesTo:          "usage_charge",
		SourceChargeID:     "charge-idem-1",
		Currency:           "CNY",
		PaidAmount:         10000,
		NetCollectedAmount: 10000,
	})
	if err != nil {
		t.Fatalf("second RecordChannelCharge() error = %v", err)
	}
	if second.Ledger == nil || first.Ledger == nil || second.Ledger.ID != first.Ledger.ID || !second.Idempotent {
		t.Fatalf("expected idempotent replay, first=%+v second=%+v", first, second)
	}
}

func TestChannelSettlementBatch_GenerateConfirmProcessClose(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-settle", "PARTNER_SETTLE")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-settle", "PROGRAM_SETTLE")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-settle",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_SETTLE",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1000,
		CooldownDays:     5,
		SettlementCycle:  "monthly",
		HoldbackRateBps:  0,
		Priority:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-settle",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	if _, err := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:            "evt_settle_1",
		ProductCode:        "menu_ai",
		OrgID:              "org-settle",
		AppliesTo:          "usage_charge",
		SourceChargeID:     "chg_settle_1",
		Currency:           "CNY",
		PaidAmount:         10000,
		NetCollectedAmount: 10000,
		OccurredAt:         "2026-04-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("record charge: %v", err)
	}

	generated, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		SettlementCycle:  "monthly",
		PeriodStart:      "2026-04-01T00:00:00Z",
		PeriodEnd:        "2026-04-30T23:59:59Z",
		Currency:         "CNY",
	})
	if err != nil {
		t.Fatalf("generate batch: %v", err)
	}
	if generated.Batch.Status != "generated" || len(generated.Items) != 1 {
		t.Fatalf("unexpected generated batch: %+v", generated)
	}

	confirmed, err := service.ConfirmChannelSettlementBatch(generated.Batch.ID, UpdateChannelSettlementBatchInput{})
	if err != nil {
		t.Fatalf("confirm batch: %v", err)
	}
	if confirmed.Batch.Status != "confirmed" {
		t.Fatalf("confirm status=%s", confirmed.Batch.Status)
	}

	processing, err := service.ProcessChannelSettlementBatch(generated.Batch.ID, UpdateChannelSettlementBatchInput{})
	if err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if processing.Batch.Status != "processing" {
		t.Fatalf("process status=%s", processing.Batch.Status)
	}

	closed, err := service.CloseChannelSettlementBatch(generated.Batch.ID, UpdateChannelSettlementBatchInput{})
	if err != nil {
		t.Fatalf("close batch: %v", err)
	}
	if closed.Batch.Status != "closed" {
		t.Fatalf("close status=%s", closed.Batch.Status)
	}
	if len(closed.Items) != 1 || closed.Items[0].Item.Status != "completed" {
		t.Fatalf("unexpected closed items: %+v", closed.Items)
	}

	var ledger models.ChannelCommissionLedger
	if err := service.repo.DB().Where("source_charge_id = ?", "chg_settle_1").First(&ledger).Error; err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	if ledger.Status != "settled" || ledger.EarnedAt == nil {
		t.Fatalf("ledger after close=%+v, want settled with earned_at", ledger)
	}
}

func TestChannelSettlementBatch_RefundAfterSettledCreatesClawbackAndAppliedOnClose(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-claw", "PARTNER_CLAW")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-claw", "PROGRAM_CLAW")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-claw",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_CLAW",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1000,
		CooldownDays:     0,
		SettlementCycle:  "monthly",
		HoldbackRateBps:  0,
		Priority:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-claw",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	if _, err := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:            "evt_claw_charge",
		ProductCode:        "menu_ai",
		OrgID:              "org-claw",
		AppliesTo:          "usage_charge",
		SourceChargeID:     "chg_claw_1",
		Currency:           "CNY",
		PaidAmount:         10000,
		NetCollectedAmount: 10000,
		OccurredAt:         "2026-04-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("record charge: %v", err)
	}
	batch, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		PeriodStart:      "2026-04-01T00:00:00Z",
		PeriodEnd:        "2026-04-30T23:59:59Z",
		Currency:         "CNY",
	})
	if err != nil {
		t.Fatalf("generate batch: %v", err)
	}
	if _, confirmErr := service.ConfirmChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{}); confirmErr != nil {
		t.Fatalf("confirm batch: %v", confirmErr)
	}
	if _, processErr := service.ProcessChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{}); processErr != nil {
		t.Fatalf("process batch: %v", processErr)
	}
	if _, closeErr := service.CloseChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{}); closeErr != nil {
		t.Fatalf("close batch: %v", closeErr)
	}

	refund, err := service.RecordChannelRefund(RecordChannelRefundInput{
		EventID:        "evt_claw_refund",
		ProductCode:    "menu_ai",
		SourceChargeID: "chg_claw_1",
		SourceRefundID: "refund_claw_1",
		RefundType:     "full_refund",
		RefundAmount:   10000,
		OccurredAt:     "2026-05-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("record refund: %v", err)
	}
	if refund.Action != "clawback_created" || refund.Clawback == nil {
		t.Fatalf("unexpected refund result: %+v", refund)
	}
	if refund.Clawback.Status != "pending" {
		t.Fatalf("clawback status=%s, want pending", refund.Clawback.Status)
	}

	nextBatch, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		PeriodStart:      "2026-05-01T00:00:00Z",
		PeriodEnd:        "2026-05-31T23:59:59Z",
		Currency:         "CNY",
	})
	if err != nil {
		t.Fatalf("generate next batch: %v", err)
	}
	if _, confirmErr := service.ConfirmChannelSettlementBatch(nextBatch.Batch.ID, UpdateChannelSettlementBatchInput{}); confirmErr != nil {
		t.Fatalf("confirm next batch: %v", confirmErr)
	}
	if _, processErr := service.ProcessChannelSettlementBatch(nextBatch.Batch.ID, UpdateChannelSettlementBatchInput{}); processErr != nil {
		t.Fatalf("process next batch: %v", processErr)
	}
	closed, err := service.CloseChannelSettlementBatch(nextBatch.Batch.ID, UpdateChannelSettlementBatchInput{})
	if err != nil {
		t.Fatalf("close next batch: %v", err)
	}
	if closed.Batch.GrossClawbackAmount <= 0 {
		t.Fatalf("expected clawback applied in batch: %+v", closed.Batch)
	}

	var updated models.ChannelClawbackLedger
	if err := service.repo.DB().Where("id = ?", refund.Clawback.ID).First(&updated).Error; err != nil {
		t.Fatalf("load clawback: %v", err)
	}
	if updated.Status != "applied" || updated.AppliedSettlementBatchID != nextBatch.Batch.ID {
		t.Fatalf("unexpected applied clawback: %+v", updated)
	}
}

func newChannelTestService(t *testing.T) *Service {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.ChannelPartner{},
		&models.ChannelProgram{},
		&models.ChannelPartnerBinding{},
		&models.ChannelCommissionPolicy{},
		&models.ChannelCommissionPolicyVersion{},
		&models.ChannelCommissionPolicyAssignment{},
		&models.ChannelProfitSnapshot{},
		&models.ChannelPolicyResolutionAudit{},
		&models.ChannelCommissionLedger{},
		&models.ChannelClawbackLedger{},
		&models.ChannelCommissionAdjustmentLedger{},
		&models.ChannelSettlementBatch{},
		&models.ChannelSettlementItem{},
		&models.ChannelSettlementItemLedger{},
		&models.ChannelSettlementItemClawback{},
		&models.ChannelSettlementItemAdjustment{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewService(repository.NewFinanceRepository(db))
}

func mustCreateChannelPartnerFixture(t *testing.T, db *gorm.DB, id, code string) *models.ChannelPartner {
	t.Helper()
	item := &models.ChannelPartner{
		ID:              id,
		Code:            code,
		Name:            code,
		PartnerType:     "channel",
		Status:          "active",
		RiskLevel:       "low",
		DefaultCurrency: "CNY",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create partner: %v", err)
	}
	return item
}

func mustCreateChannelProgramFixture(t *testing.T, db *gorm.DB, id, code string) *models.ChannelProgram {
	t.Helper()
	item := &models.ChannelProgram{
		ID:                     id,
		ProductCode:            "menu_ai",
		ProgramCode:            code,
		Name:                   code,
		ProgramType:            "channel_revenue_share",
		Status:                 "active",
		DefaultSettlementCycle: "monthly",
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create program: %v", err)
	}
	return item
}
