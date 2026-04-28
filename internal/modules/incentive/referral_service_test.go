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

func TestResolveReferralCode_IncludesPolicyDescription(t *testing.T) {
	service := newReferralTestService(t)
	program := &models.ReferralProgram{
		ID:                    "program-1",
		ProductCode:           "menu",
		ProgramCode:           "menu-first-sub",
		Name:                  "Menu First Subscription",
		Status:                "active",
		TriggerType:           "first_subscription",
		CommissionPolicy:      "fixed_amount",
		CommissionCurrency:    "USD",
		CommissionFixedAmount: 50,
		SettlementDelayDays:   7,
	}
	code := &models.ReferralCode{
		ID:                  "code-1",
		ProgramID:           program.ID,
		ProductCode:         "menu",
		Code:                "HELLO50",
		PromoterSubjectType: "organization",
		PromoterSubjectID:   "org-1",
		Status:              "active",
	}
	mustCreateReferralFixture(t, service.repo.DB(), program, code)

	resolved, err := service.ResolveReferralCode("HELLO50", "menu")
	if err != nil {
		t.Fatalf("ResolveReferralCode() error = %v", err)
	}
	if resolved.RewardPolicyDesc == "" {
		t.Fatalf("expected reward policy description")
	}
	if resolved.TriggerType != "first_subscription" || resolved.SettlementDelayDays != 7 {
		t.Fatalf("unexpected resolved response: %+v", resolved)
	}
}

func TestCreateReferralConversion_BlocksDuplicateAndSelfInvite(t *testing.T) {
	service := newReferralTestService(t)
	program := &models.ReferralProgram{
		ID:                    "program-2",
		ProductCode:           "menu",
		ProgramCode:           "signup-program",
		Name:                  "Signup Program",
		Status:                "active",
		TriggerType:           "signup",
		CommissionPolicy:      "fixed_amount",
		CommissionCurrency:    "USD",
		CommissionFixedAmount: 20,
		AllowRepeat:           false,
	}
	code := &models.ReferralCode{
		ID:                  "code-2",
		ProgramID:           program.ID,
		ProductCode:         "menu",
		Code:                "SIGNUP20",
		PromoterSubjectType: "organization",
		PromoterSubjectID:   "org-promoter",
		Status:              "active",
	}
	mustCreateReferralFixture(t, service.repo.DB(), program, code)

	_, err := service.CreateReferralConversion(CreateReferralConversionInput{
		ReferralCode:        "SIGNUP20",
		ProductCode:         "menu",
		TriggerType:         "signup",
		ReferredSubjectType: "organization",
		ReferredSubjectID:   "org-new",
		ReferenceType:       "signup",
		ReferenceID:         "ref-1",
	})
	if err != nil {
		t.Fatalf("first CreateReferralConversion() error = %v", err)
	}

	_, err = service.CreateReferralConversion(CreateReferralConversionInput{
		ReferralCode:        "SIGNUP20",
		ProductCode:         "menu",
		TriggerType:         "signup",
		ReferredSubjectType: "organization",
		ReferredSubjectID:   "org-new",
		ReferenceType:       "signup",
		ReferenceID:         "ref-2",
	})
	if err != ErrReferralAlreadyClaimed {
		t.Fatalf("expected ErrReferralAlreadyClaimed, got %v", err)
	}

	_, err = service.CreateReferralConversion(CreateReferralConversionInput{
		ReferralCode:        "SIGNUP20",
		ProductCode:         "menu",
		TriggerType:         "signup",
		ReferredSubjectType: "organization",
		ReferredSubjectID:   "org-promoter",
		ReferenceType:       "signup",
		ReferenceID:         "self-ref",
	})
	if err != ErrReferralSelfInviteBlocked {
		t.Fatalf("expected ErrReferralSelfInviteBlocked, got %v", err)
	}
}

func TestCreateReferralConversion_UsesPendingStatusWhenDelayed(t *testing.T) {
	service := newReferralTestService(t)
	program := &models.ReferralProgram{
		ID:                  "program-3",
		ProductCode:         "menu",
		ProgramCode:         "menu-first-paid",
		Name:                "Menu First Paid",
		Status:              "active",
		TriggerType:         "first_paid_order",
		CommissionPolicy:    "percentage",
		CommissionCurrency:  "USD",
		CommissionRateBps:   1000,
		SettlementDelayDays: 14,
	}
	code := &models.ReferralCode{
		ID:                  "code-3",
		ProgramID:           program.ID,
		ProductCode:         "menu",
		Code:                "PAID10",
		PromoterSubjectType: "organization",
		PromoterSubjectID:   "org-promoter-2",
		Status:              "active",
	}
	mustCreateReferralFixture(t, service.repo.DB(), program, code)

	item, err := service.CreateReferralConversion(CreateReferralConversionInput{
		ReferralCode:          "PAID10",
		ProductCode:           "menu",
		TriggerType:           "first_paid_order",
		ReferredSubjectType:   "organization",
		ReferredSubjectID:     "org-paid",
		SettlementSubjectType: "organization",
		SettlementSubjectID:   "org-paid",
		ReferenceType:         "order",
		ReferenceID:           "order-1",
		CommissionBaseAmount:  1000,
	})
	if err != nil {
		t.Fatalf("CreateReferralConversion() error = %v", err)
	}
	if item.Status != "pending_reward" || item.CommissionAmount != 100 {
		t.Fatalf("unexpected conversion: %+v", item)
	}

	var ledger models.CommissionLedger
	if err := service.repo.DB().Where("id = ?", item.CommissionLedgerID).First(&ledger).Error; err != nil {
		t.Fatalf("load commission ledger: %v", err)
	}
	if ledger.Status != "pending" {
		t.Fatalf("commission status = %s, want pending", ledger.Status)
	}
}

func TestRedeemCommissions_IssuesRewardAndMarksCommissionRedeemed(t *testing.T) {
	service := newReferralTestService(t)
	now := time.Now()
	commission := &models.CommissionLedger{
		ID:                     "commission-1",
		ProductCode:            "menu",
		CommissionType:         "first_subscription",
		BeneficiarySubjectType: "organization",
		BeneficiarySubjectID:   "org-credits",
		Currency:               "MENU_CREDIT",
		Amount:                 80,
		Status:                 "earned",
		ReferenceType:          "subscription",
		ReferenceID:            "sub-1",
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	conversion := &models.ReferralConversion{
		ID:                  "conversion-1",
		ProgramID:           "program-redeem",
		ReferralCodeID:      "code-redeem",
		ProductCode:         "menu",
		TriggerType:         "first_subscription",
		PromoterSubjectType: "organization",
		PromoterSubjectID:   "org-promoter",
		ReferredSubjectType: "organization",
		ReferredSubjectID:   "org-credits",
		ReferenceType:       "subscription",
		ReferenceID:         "sub-1",
		CommissionCurrency:  "MENU_CREDIT",
		CommissionAmount:    80,
		CommissionLedgerID:  "commission-1",
		Status:              "commission_earned",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := service.repo.DB().Create(commission).Error; err != nil {
		t.Fatalf("create commission: %v", err)
	}
	if err := service.repo.DB().Create(conversion).Error; err != nil {
		t.Fatalf("create conversion: %v", err)
	}

	result, err := service.RedeemCommissions(RedeemCommissionsInput{
		ProductCode:            "menu",
		BeneficiarySubjectType: "organization",
		BeneficiarySubjectID:   "org-credits",
		AssetCode:              "MENU_CREDIT",
	})
	if err != nil {
		t.Fatalf("RedeemCommissions() error = %v", err)
	}
	if result.TotalAmount != 80 || result.RewardLedgerID == "" {
		t.Fatalf("unexpected redeem result: %+v", result)
	}

	var updated models.CommissionLedger
	if err := service.repo.DB().Where("id = ?", "commission-1").First(&updated).Error; err != nil {
		t.Fatalf("load updated commission: %v", err)
	}
	if updated.Status != "redeemed" || updated.RedeemedRewardID == "" {
		t.Fatalf("unexpected updated commission: %+v", updated)
	}

	var reward models.RewardLedger
	if err := service.repo.DB().Where("id = ?", result.RewardLedgerID).First(&reward).Error; err != nil {
		t.Fatalf("load reward ledger: %v", err)
	}
	if reward.Amount != 80 || reward.AssetCode != "MENU_CREDIT" {
		t.Fatalf("unexpected reward ledger: %+v", reward)
	}

	var wallet models.WalletAccount
	if err := service.repo.DB().Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-credits", "MENU_CREDIT").First(&wallet).Error; err != nil {
		t.Fatalf("load wallet account: %v", err)
	}
	if wallet.Balance != 80 {
		t.Fatalf("wallet balance = %d, want 80", wallet.Balance)
	}

	var updatedConversion models.ReferralConversion
	if err := service.repo.DB().Where("id = ?", "conversion-1").First(&updatedConversion).Error; err != nil {
		t.Fatalf("load updated conversion: %v", err)
	}
	if updatedConversion.Status != "reward_issued" {
		t.Fatalf("conversion status = %s, want reward_issued", updatedConversion.Status)
	}
}

func newReferralTestService(t *testing.T) *Service {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(fmt.Sprintf("%s_%d", t.Name(), time.Now().UnixNano()))
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AssetDefinition{}, &models.ReferralProgram{}, &models.ReferralCode{}, &models.ReferralConversion{}, &models.CommissionLedger{}, &models.RewardLedger{}, &models.WalletAccount{}, &models.WalletBucket{}, &models.WalletLedger{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewService(repository.NewFinanceRepository(db))
}

func mustCreateReferralFixture(t *testing.T, db *gorm.DB, program *models.ReferralProgram, code *models.ReferralCode) {
	t.Helper()
	now := time.Now()
	if program.CreatedAt.IsZero() {
		program.CreatedAt = now
	}
	if program.UpdatedAt.IsZero() {
		program.UpdatedAt = now
	}
	if code.CreatedAt.IsZero() {
		code.CreatedAt = now
	}
	if code.UpdatedAt.IsZero() {
		code.UpdatedAt = now
	}
	if err := db.Create(program).Error; err != nil {
		t.Fatalf("create program: %v", err)
	}
	if err := db.Create(code).Error; err != nil {
		t.Fatalf("create code: %v", err)
	}
}
