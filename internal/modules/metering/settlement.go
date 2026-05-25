package metering

import (
	"encoding/json"
	"errors"
	"time"

	"platform-service/internal/models"
	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

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
			if consumeErr := s.consumeQuota(tx, record.BillingSubjectType, record.BillingSubjectID, record.BillableItemCode, quotaUnits, event.EventID, "metering_settlement"); consumeErr != nil {
				return result, consumeErr
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
