package repository

import (
	"time"

	"platform-service/internal/models"
)

func (r *FinanceRepository) FindChannelSettlementBatchByID(id string) (*models.ChannelSettlementBatch, error) {
	var item models.ChannelSettlementBatch
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindChannelSettlementBatchByPeriod(productCode, channelProgramID string, periodStart, periodEnd time.Time) (*models.ChannelSettlementBatch, error) {
	var item models.ChannelSettlementBatch
	if err := r.db.
		Where("product_code = ? AND channel_program_id = ? AND period_start = ? AND period_end = ?", productCode, channelProgramID, periodStart, periodEnd).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListChannelSettlementBatches(productCode, channelProgramID, status string) ([]models.ChannelSettlementBatch, error) {
	var items []models.ChannelSettlementBatch
	q := r.db.Order("period_end desc, created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if channelProgramID != "" {
		q = q.Where("channel_program_id = ?", channelProgramID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListChannelSettlementItems(batchID, channelPartnerID, status string) ([]models.ChannelSettlementItem, error) {
	var items []models.ChannelSettlementItem
	q := r.db.Order("created_at asc")
	if batchID != "" {
		q = q.Where("settlement_batch_id = ?", batchID)
	}
	if channelPartnerID != "" {
		q = q.Where("channel_partner_id = ?", channelPartnerID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListChannelClawbackLedgers(productCode, channelPartnerID, status string) ([]models.ChannelClawbackLedger, error) {
	var items []models.ChannelClawbackLedger
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if channelPartnerID != "" {
		q = q.Where("channel_partner_id = ?", channelPartnerID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListMatureableChannelCommissionLedgers(productCode, channelProgramID string, asOf time.Time) ([]models.ChannelCommissionLedger, error) {
	var items []models.ChannelCommissionLedger
	q := r.db.
		Where("status = ? AND available_at IS NOT NULL AND available_at <= ?", "pending", asOf).
		Order("available_at asc, created_at asc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if channelProgramID != "" {
		q = q.Where("channel_program_id = ?", channelProgramID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListEligibleChannelCommissionLedgers(productCode, channelProgramID, currency string, periodEnd time.Time) ([]models.ChannelCommissionLedger, error) {
	var items []models.ChannelCommissionLedger
	q := r.db.
		Table("channel_commission_ledgers").
		Select("channel_commission_ledgers.*").
		Joins("LEFT JOIN channel_settlement_item_ledgers ON channel_settlement_item_ledgers.commission_ledger_id = channel_commission_ledgers.id").
		Joins("LEFT JOIN channel_settlement_batches ON channel_settlement_batches.id = channel_settlement_item_ledgers.settlement_batch_id AND channel_settlement_batches.status <> ?", "canceled").
		Where("channel_settlement_batches.id IS NULL").
		Where("channel_commission_ledgers.status = ? AND channel_commission_ledgers.earned_at IS NOT NULL AND channel_commission_ledgers.earned_at <= ?", "earned", periodEnd).
		Order("earned_at asc, created_at asc")
	if productCode != "" {
		q = q.Where("channel_commission_ledgers.product_code = ?", productCode)
	}
	if channelProgramID != "" {
		q = q.Where("channel_commission_ledgers.channel_program_id = ?", channelProgramID)
	}
	if currency != "" {
		q = q.Where("channel_commission_ledgers.currency = ?", currency)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListEligibleChannelClawbackLedgers(productCode, channelProgramID, currency string, periodEnd time.Time) ([]models.ChannelClawbackLedger, error) {
	var items []models.ChannelClawbackLedger
	q := r.db.
		Table("channel_clawback_ledgers").
		Select("channel_clawback_ledgers.*").
		Joins("LEFT JOIN channel_settlement_item_clawbacks ON channel_settlement_item_clawbacks.clawback_ledger_id = channel_clawback_ledgers.id").
		Joins("LEFT JOIN channel_settlement_batches AS settlement_batches ON settlement_batches.id = channel_settlement_item_clawbacks.settlement_batch_id AND settlement_batches.status <> ?", "canceled").
		Joins("JOIN channel_commission_ledgers ON channel_commission_ledgers.id = channel_clawback_ledgers.source_commission_ledger_id").
		Where("settlement_batches.id IS NULL").
		Where("channel_clawback_ledgers.status = ? AND channel_clawback_ledgers.created_at <= ?", "pending", periodEnd).
		Order("channel_clawback_ledgers.created_at asc")
	if productCode != "" {
		q = q.Where("channel_clawback_ledgers.product_code = ?", productCode)
	}
	if channelProgramID != "" {
		q = q.Where("channel_commission_ledgers.channel_program_id = ?", channelProgramID)
	}
	if currency != "" {
		q = q.Where("channel_clawback_ledgers.currency = ?", currency)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListEligibleChannelCommissionAdjustmentLedgers(productCode, channelProgramID, currency string, periodEnd time.Time) ([]models.ChannelCommissionAdjustmentLedger, error) {
	var items []models.ChannelCommissionAdjustmentLedger
	q := r.db.
		Table("channel_commission_adjustment_ledgers").
		Select("channel_commission_adjustment_ledgers.*").
		Joins("LEFT JOIN channel_settlement_item_adjustments ON channel_settlement_item_adjustments.adjustment_ledger_id = channel_commission_adjustment_ledgers.id").
		Joins("LEFT JOIN channel_settlement_batches AS settlement_batches ON settlement_batches.id = channel_settlement_item_adjustments.settlement_batch_id AND settlement_batches.status <> ?", "canceled").
		Where("settlement_batches.id IS NULL").
		Where("channel_commission_adjustment_ledgers.status = ? AND (channel_commission_adjustment_ledgers.effective_at IS NULL OR channel_commission_adjustment_ledgers.effective_at <= ?)", "pending", periodEnd).
		Order("channel_commission_adjustment_ledgers.created_at asc")
	if productCode != "" {
		q = q.Where("channel_commission_adjustment_ledgers.product_code = ?", productCode)
	}
	if channelProgramID != "" {
		q = q.Where("channel_commission_adjustment_ledgers.channel_program_id = ?", channelProgramID)
	}
	if currency != "" {
		q = q.Where("channel_commission_adjustment_ledgers.currency = ?", currency)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListChannelSettlementItemLedgers(batchID, settlementItemID string) ([]models.ChannelSettlementItemLedger, error) {
	var items []models.ChannelSettlementItemLedger
	q := r.db.Order("created_at asc")
	if batchID != "" {
		q = q.Where("settlement_batch_id = ?", batchID)
	}
	if settlementItemID != "" {
		q = q.Where("settlement_item_id = ?", settlementItemID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListChannelSettlementItemClawbacks(batchID, settlementItemID string) ([]models.ChannelSettlementItemClawback, error) {
	var items []models.ChannelSettlementItemClawback
	q := r.db.Order("created_at asc")
	if batchID != "" {
		q = q.Where("settlement_batch_id = ?", batchID)
	}
	if settlementItemID != "" {
		q = q.Where("settlement_item_id = ?", settlementItemID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListChannelSettlementItemAdjustments(batchID, settlementItemID string) ([]models.ChannelSettlementItemAdjustment, error) {
	var items []models.ChannelSettlementItemAdjustment
	q := r.db.Order("created_at asc")
	if batchID != "" {
		q = q.Where("settlement_batch_id = ?", batchID)
	}
	if settlementItemID != "" {
		q = q.Where("settlement_item_id = ?", settlementItemID)
	}
	err := q.Find(&items).Error
	return items, err
}
