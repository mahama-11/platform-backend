package incentive

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

type GenerateChannelSettlementBatchInput struct {
	ProductCode      string `json:"product_code" binding:"required"`
	ChannelProgramID string `json:"channel_program_id" binding:"required"`
	SettlementCycle  string `json:"settlement_cycle"`
	PeriodStart      string `json:"period_start" binding:"required"`
	PeriodEnd        string `json:"period_end" binding:"required"`
	Currency         string `json:"currency"`
	Metadata         string `json:"metadata"`
}

type UpdateChannelSettlementBatchInput struct {
	Metadata string `json:"metadata"`
	Reason   string `json:"reason"`
}

type ChannelSettlementItemDetail struct {
	Item                models.ChannelSettlementItem `json:"item"`
	CommissionLedgerIDs []string                     `json:"commission_ledger_ids"`
	ClawbackLedgerIDs   []string                     `json:"clawback_ledger_ids"`
	AdjustmentLedgerIDs []string                     `json:"adjustment_ledger_ids"`
}

type ChannelSettlementBatchDetail struct {
	Batch models.ChannelSettlementBatch `json:"batch"`
	Items []ChannelSettlementItemDetail `json:"items"`
}

var (
	ErrChannelSettlementBatchExists         = errors.New("channel settlement batch already exists")
	ErrChannelSettlementBatchInvalidState   = errors.New("channel settlement batch invalid state")
	ErrChannelSettlementBatchEmpty          = errors.New("channel settlement batch has no eligible items")
	ErrChannelSettlementBatchPeriodInvalid  = errors.New("channel settlement batch period invalid")
	ErrChannelSettlementBatchProgramMissing = errors.New("channel settlement batch program missing")
)

func (s *Service) ListChannelSettlementBatches(productCode, channelProgramID, status string) ([]models.ChannelSettlementBatch, error) {
	return s.repo.ListChannelSettlementBatches(productCode, channelProgramID, status)
}

func (s *Service) ListChannelSettlementItems(batchID, channelPartnerID, status string) ([]models.ChannelSettlementItem, error) {
	return s.repo.ListChannelSettlementItems(batchID, channelPartnerID, status)
}

func (s *Service) ListChannelClawbacks(productCode, channelPartnerID, status string) ([]models.ChannelClawbackLedger, error) {
	return s.repo.ListChannelClawbackLedgers(productCode, channelPartnerID, status)
}

func (s *Service) GetChannelSettlementBatchDetail(batchID string) (*ChannelSettlementBatchDetail, error) {
	batch, err := s.repo.FindChannelSettlementBatchByID(batchID)
	if err != nil {
		return nil, err
	}
	return s.buildChannelSettlementBatchDetail(batch)
}

func (s *Service) GenerateChannelSettlementBatch(input GenerateChannelSettlementBatchInput) (*ChannelSettlementBatchDetail, error) {
	periodStart, err := time.Parse(time.RFC3339, input.PeriodStart)
	if err != nil {
		return nil, err
	}
	periodEnd, err := time.Parse(time.RFC3339, input.PeriodEnd)
	if err != nil {
		return nil, err
	}
	if !periodEnd.After(periodStart) {
		return nil, ErrChannelSettlementBatchPeriodInvalid
	}
	program, err := s.repo.FindChannelProgramByID(input.ChannelProgramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelSettlementBatchProgramMissing
		}
		return nil, err
	}
	if existing, lookupErr := s.repo.FindChannelSettlementBatchByPeriod(input.ProductCode, input.ChannelProgramID, periodStart, periodEnd); lookupErr == nil {
		return s.buildChannelSettlementBatchDetail(existing)
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
	}

	now := time.Now()
	if _, matureErr := s.matureChannelCommissionsTx(s.repo.DB(), input.ProductCode, input.ChannelProgramID, periodEnd); matureErr != nil {
		return nil, matureErr
	}

	commissions, err := s.repo.ListEligibleChannelCommissionLedgers(input.ProductCode, input.ChannelProgramID, input.Currency, periodEnd)
	if err != nil {
		return nil, err
	}
	clawbacks, err := s.repo.ListEligibleChannelClawbackLedgers(input.ProductCode, input.ChannelProgramID, input.Currency, periodEnd)
	if err != nil {
		return nil, err
	}
	adjustments, err := s.repo.ListEligibleChannelCommissionAdjustmentLedgers(input.ProductCode, input.ChannelProgramID, input.Currency, periodEnd)
	if err != nil {
		return nil, err
	}
	if len(commissions) == 0 && len(clawbacks) == 0 && len(adjustments) == 0 {
		return nil, ErrChannelSettlementBatchEmpty
	}

	batch := &models.ChannelSettlementBatch{
		ID:               utils.GenerateID(),
		BatchNo:          "csb_" + utils.GenerateID(),
		ProductCode:      input.ProductCode,
		ChannelProgramID: input.ChannelProgramID,
		SettlementCycle:  defaultString(input.SettlementCycle, firstNonEmpty(program.DefaultSettlementCycle, "monthly")),
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		Currency:         defaultString(input.Currency, "CNY"),
		Status:           "generated",
		GeneratedAt:      &now,
		Metadata:         input.Metadata,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	grouped := groupSettlementEntries(commissions, clawbacks, adjustments)
	items := make([]models.ChannelSettlementItem, 0, len(grouped))
	itemLedgers := make([]models.ChannelSettlementItemLedger, 0, len(commissions))
	itemClawbacks := make([]models.ChannelSettlementItemClawback, 0, len(clawbacks))
	itemAdjustments := make([]models.ChannelSettlementItemAdjustment, 0, len(adjustments))

	var totalCommission, totalClawback, totalNet int64
	for _, group := range grouped {
		itemID := utils.GenerateID()
		snapshot, err := buildSettlementItemSnapshot(group.partnerID, group.currency, group.ledgerIDs, group.clawbackIDs, group.adjustmentIDs, periodStart, periodEnd)
		if err != nil {
			return nil, err
		}
		item := models.ChannelSettlementItem{
			ID:                itemID,
			SettlementBatchID: batch.ID,
			ChannelPartnerID:  group.partnerID,
			Currency:          group.currency,
			CommissionAmount:  group.commissionAmount,
			ClawbackAmount:    group.clawbackAmount,
			AdjustmentAmount:  group.adjustmentAmount,
			NetAmount:         group.commissionAmount - group.clawbackAmount + group.adjustmentAmount,
			Status:            platformconst.StatusPending,
			StatementSnapshot: snapshot,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		items = append(items, item)
		totalCommission += item.CommissionAmount
		totalClawback += item.ClawbackAmount
		totalNet += item.NetAmount
		for _, ledgerID := range group.ledgerIDs {
			itemLedgers = append(itemLedgers, models.ChannelSettlementItemLedger{
				ID:                 utils.GenerateID(),
				SettlementBatchID:  batch.ID,
				SettlementItemID:   itemID,
				CommissionLedgerID: ledgerID,
				CreatedAt:          now,
			})
		}
		for _, clawbackID := range group.clawbackIDs {
			itemClawbacks = append(itemClawbacks, models.ChannelSettlementItemClawback{
				ID:                utils.GenerateID(),
				SettlementBatchID: batch.ID,
				SettlementItemID:  itemID,
				ClawbackLedgerID:  clawbackID,
				CreatedAt:         now,
			})
		}
		for _, adjustmentID := range group.adjustmentIDs {
			itemAdjustments = append(itemAdjustments, models.ChannelSettlementItemAdjustment{
				ID:                 utils.GenerateID(),
				SettlementBatchID:  batch.ID,
				SettlementItemID:   itemID,
				AdjustmentLedgerID: adjustmentID,
				CreatedAt:          now,
			})
		}
	}
	batch.TotalPartnerCount = int64(len(grouped))
	batch.TotalItemCount = int64(len(items))
	batch.GrossCommissionAmount = totalCommission
	batch.GrossClawbackAmount = totalClawback
	batch.NetSettleableAmount = totalNet
	if batch.Currency == "" {
		batch.Currency = resolveBatchCurrency(grouped)
	}

	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		if len(itemLedgers) > 0 {
			if err := tx.Create(&itemLedgers).Error; err != nil {
				return err
			}
		}
		if len(itemClawbacks) > 0 {
			if err := tx.Create(&itemClawbacks).Error; err != nil {
				return err
			}
		}
		if len(itemAdjustments) > 0 {
			if err := tx.Create(&itemAdjustments).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return s.buildChannelSettlementBatchDetail(batch)
}

func (s *Service) ConfirmChannelSettlementBatch(batchID string, input UpdateChannelSettlementBatchInput) (*ChannelSettlementBatchDetail, error) {
	now := time.Now()
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var batch models.ChannelSettlementBatch
		if err := tx.Where("id = ?", batchID).First(&batch).Error; err != nil {
			return err
		}
		if batch.Status != platformconst.StatusGenerated {
			return ErrChannelSettlementBatchInvalidState
		}
		batch.Status = platformconst.StatusConfirmed
		batch.ConfirmedAt = &now
		if input.Metadata != "" {
			batch.Metadata = input.Metadata
		}
		batch.UpdatedAt = now
		if err := tx.Save(&batch).Error; err != nil {
			return err
		}
		return tx.Model(&models.ChannelSettlementItem{}).
			Where("settlement_batch_id = ? AND status = ?", batchID, platformconst.StatusPending).
			Updates(map[string]any{"status": platformconst.StatusConfirmed, "updated_at": now}).Error
	}); err != nil {
		return nil, err
	}
	return s.GetChannelSettlementBatchDetail(batchID)
}

func (s *Service) ProcessChannelSettlementBatch(batchID string, input UpdateChannelSettlementBatchInput) (*ChannelSettlementBatchDetail, error) {
	now := time.Now()
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var batch models.ChannelSettlementBatch
		if err := tx.Where("id = ?", batchID).First(&batch).Error; err != nil {
			return err
		}
		if batch.Status != platformconst.StatusConfirmed {
			return ErrChannelSettlementBatchInvalidState
		}
		batch.Status = platformconst.StatusProcessing
		if input.Metadata != "" {
			batch.Metadata = input.Metadata
		}
		batch.UpdatedAt = now
		if err := tx.Save(&batch).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.ChannelSettlementItem{}).
			Where("settlement_batch_id = ? AND status IN ?", batchID, []string{platformconst.StatusPending, platformconst.StatusConfirmed}).
			Updates(map[string]any{"status": platformconst.StatusProcessing, "updated_at": now}).Error; err != nil {
			return err
		}
		var links []models.ChannelSettlementItemLedger
		if err := tx.Where("settlement_batch_id = ?", batchID).Find(&links).Error; err != nil {
			return err
		}
		if len(links) > 0 {
			ledgerIDs := make([]string, 0, len(links))
			for _, link := range links {
				ledgerIDs = append(ledgerIDs, link.CommissionLedgerID)
			}
			if err := tx.Model(&models.ChannelCommissionLedger{}).
				Where("id IN ? AND status = ?", ledgerIDs, platformconst.CommissionStatusEarned).
				Updates(map[string]any{"status": platformconst.StatusSettlementInProgress, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		var adjustmentLinks []models.ChannelSettlementItemAdjustment
		if err := tx.Where("settlement_batch_id = ?", batchID).Find(&adjustmentLinks).Error; err != nil {
			return err
		}
		if len(adjustmentLinks) > 0 {
			adjustmentIDs := make([]string, 0, len(adjustmentLinks))
			for _, link := range adjustmentLinks {
				adjustmentIDs = append(adjustmentIDs, link.AdjustmentLedgerID)
			}
			if err := tx.Model(&models.ChannelCommissionAdjustmentLedger{}).
				Where("id IN ? AND status = ?", adjustmentIDs, platformconst.StatusPending).
				Updates(map[string]any{"status": platformconst.StatusSettlementInProgress, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.GetChannelSettlementBatchDetail(batchID)
}

func (s *Service) CloseChannelSettlementBatch(batchID string, input UpdateChannelSettlementBatchInput) (*ChannelSettlementBatchDetail, error) {
	now := time.Now()
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var batch models.ChannelSettlementBatch
		if err := tx.Where("id = ?", batchID).First(&batch).Error; err != nil {
			return err
		}
		if batch.Status != platformconst.StatusProcessing {
			return ErrChannelSettlementBatchInvalidState
		}
		batch.Status = platformconst.StatusClosed
		batch.ClosedAt = &now
		if input.Metadata != "" {
			batch.Metadata = input.Metadata
		}
		batch.UpdatedAt = now
		if err := tx.Save(&batch).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.ChannelSettlementItem{}).
			Where("settlement_batch_id = ?", batchID).
			Updates(map[string]any{"status": platformconst.StatusCompleted, "updated_at": now}).Error; err != nil {
			return err
		}
		var links []models.ChannelSettlementItemLedger
		if err := tx.Where("settlement_batch_id = ?", batchID).Find(&links).Error; err != nil {
			return err
		}
		if len(links) > 0 {
			ledgerIDs := make([]string, 0, len(links))
			for _, link := range links {
				ledgerIDs = append(ledgerIDs, link.CommissionLedgerID)
			}
			if err := tx.Model(&models.ChannelCommissionLedger{}).
				Where("id IN ? AND status IN ?", ledgerIDs, []string{platformconst.CommissionStatusEarned, platformconst.StatusSettlementInProgress}).
				Updates(map[string]any{"status": platformconst.SettlementStatusSettled, "settled_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		var clawbackLinks []models.ChannelSettlementItemClawback
		if err := tx.Where("settlement_batch_id = ?", batchID).Find(&clawbackLinks).Error; err != nil {
			return err
		}
		if len(clawbackLinks) > 0 {
			clawbackIDs := make([]string, 0, len(clawbackLinks))
			for _, link := range clawbackLinks {
				clawbackIDs = append(clawbackIDs, link.ClawbackLedgerID)
			}
			if err := tx.Model(&models.ChannelClawbackLedger{}).
				Where("id IN ? AND status = ?", clawbackIDs, platformconst.StatusPending).
				Updates(map[string]any{"status": platformconst.StatusApplied, "applied_settlement_batch_id": batchID, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		var adjustmentLinks []models.ChannelSettlementItemAdjustment
		if err := tx.Where("settlement_batch_id = ?", batchID).Find(&adjustmentLinks).Error; err != nil {
			return err
		}
		if len(adjustmentLinks) > 0 {
			adjustmentIDs := make([]string, 0, len(adjustmentLinks))
			for _, link := range adjustmentLinks {
				adjustmentIDs = append(adjustmentIDs, link.AdjustmentLedgerID)
			}
			if err := tx.Model(&models.ChannelCommissionAdjustmentLedger{}).
				Where("id IN ? AND status IN ?", adjustmentIDs, []string{platformconst.StatusPending, platformconst.StatusSettlementInProgress}).
				Updates(map[string]any{"status": platformconst.StatusApplied, "applied_settlement_batch_id": batchID, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.GetChannelSettlementBatchDetail(batchID)
}

func (s *Service) CancelChannelSettlementBatch(batchID string, input UpdateChannelSettlementBatchInput) (*ChannelSettlementBatchDetail, error) {
	now := time.Now()
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var batch models.ChannelSettlementBatch
		if err := tx.Where("id = ?", batchID).First(&batch).Error; err != nil {
			return err
		}
		if batch.Status != platformconst.StatusGenerated && batch.Status != platformconst.StatusConfirmed {
			return ErrChannelSettlementBatchInvalidState
		}
		batch.Status = platformconst.StatusCanceled
		if input.Metadata != "" {
			batch.Metadata = input.Metadata
		}
		if input.Reason != "" {
			batch.Metadata = mergeReasonMetadata(batch.Metadata, input.Reason)
		}
		batch.UpdatedAt = now
		if err := tx.Save(&batch).Error; err != nil {
			return err
		}
		return tx.Model(&models.ChannelSettlementItem{}).
			Where("settlement_batch_id = ?", batchID).
			Updates(map[string]any{"status": platformconst.StatusCanceled, "updated_at": now}).Error
	}); err != nil {
		return nil, err
	}
	return s.GetChannelSettlementBatchDetail(batchID)
}

func (s *Service) matureChannelCommissionsTx(db *gorm.DB, productCode, channelProgramID string, asOf time.Time) (int, error) {
	items, err := s.repo.ListMatureableChannelCommissionLedgers(productCode, channelProgramID, asOf)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if err := db.Model(&models.ChannelCommissionLedger{}).
		Where("id IN ? AND status = ?", ids, platformconst.StatusPending).
		Updates(map[string]any{"status": platformconst.CommissionStatusEarned, "earned_at": asOf, "updated_at": asOf}).Error; err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *Service) buildChannelSettlementBatchDetail(batch *models.ChannelSettlementBatch) (*ChannelSettlementBatchDetail, error) {
	items, err := s.repo.ListChannelSettlementItems(batch.ID, "", "")
	if err != nil {
		return nil, err
	}
	ledgerLinks, err := s.repo.ListChannelSettlementItemLedgers(batch.ID, "")
	if err != nil {
		return nil, err
	}
	clawbackLinks, err := s.repo.ListChannelSettlementItemClawbacks(batch.ID, "")
	if err != nil {
		return nil, err
	}
	adjustmentLinks, err := s.repo.ListChannelSettlementItemAdjustments(batch.ID, "")
	if err != nil {
		return nil, err
	}
	ledgerMap := make(map[string][]string)
	for _, link := range ledgerLinks {
		ledgerMap[link.SettlementItemID] = append(ledgerMap[link.SettlementItemID], link.CommissionLedgerID)
	}
	clawbackMap := make(map[string][]string)
	for _, link := range clawbackLinks {
		clawbackMap[link.SettlementItemID] = append(clawbackMap[link.SettlementItemID], link.ClawbackLedgerID)
	}
	adjustmentMap := make(map[string][]string)
	for _, link := range adjustmentLinks {
		adjustmentMap[link.SettlementItemID] = append(adjustmentMap[link.SettlementItemID], link.AdjustmentLedgerID)
	}
	details := make([]ChannelSettlementItemDetail, 0, len(items))
	for _, item := range items {
		details = append(details, ChannelSettlementItemDetail{
			Item:                item,
			CommissionLedgerIDs: ledgerMap[item.ID],
			ClawbackLedgerIDs:   clawbackMap[item.ID],
			AdjustmentLedgerIDs: adjustmentMap[item.ID],
		})
	}
	return &ChannelSettlementBatchDetail{Batch: *batch, Items: details}, nil
}

type settlementGroup struct {
	partnerID        string
	currency         string
	commissionAmount int64
	clawbackAmount   int64
	adjustmentAmount int64
	ledgerIDs        []string
	clawbackIDs      []string
	adjustmentIDs    []string
}

func groupSettlementEntries(commissions []models.ChannelCommissionLedger, clawbacks []models.ChannelClawbackLedger, adjustments []models.ChannelCommissionAdjustmentLedger) []settlementGroup {
	groupMap := map[string]*settlementGroup{}
	order := make([]string, 0)
	ensure := func(partnerID, currency string) *settlementGroup {
		key := partnerID + "|" + currency
		if existing, ok := groupMap[key]; ok {
			return existing
		}
		group := &settlementGroup{partnerID: partnerID, currency: currency}
		groupMap[key] = group
		order = append(order, key)
		return group
	}
	for _, ledger := range commissions {
		group := ensure(ledger.ChannelPartnerID, ledger.Currency)
		group.commissionAmount += ledger.SettleableAmount
		group.ledgerIDs = append(group.ledgerIDs, ledger.ID)
	}
	for _, clawback := range clawbacks {
		group := ensure(clawback.ChannelPartnerID, clawback.Currency)
		group.clawbackAmount += clawback.ClawbackAmount
		group.clawbackIDs = append(group.clawbackIDs, clawback.ID)
	}
	for _, adjustment := range adjustments {
		group := ensure(adjustment.ChannelPartnerID, adjustment.Currency)
		group.adjustmentAmount += adjustment.AdjustmentAmount
		group.adjustmentIDs = append(group.adjustmentIDs, adjustment.ID)
	}
	out := make([]settlementGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *groupMap[key])
	}
	return out
}

func buildSettlementItemSnapshot(partnerID, currency string, ledgerIDs, clawbackIDs, adjustmentIDs []string, periodStart, periodEnd time.Time) (string, error) {
	payload := map[string]any{
		"channel_partner_id":    partnerID,
		"currency":              currency,
		"period_start":          periodStart.Format(time.RFC3339),
		"period_end":            periodEnd.Format(time.RFC3339),
		"commission_ledger_ids": ledgerIDs,
		"clawback_ledger_ids":   clawbackIDs,
		"adjustment_ledger_ids": adjustmentIDs,
		"commission_item_count": len(ledgerIDs),
		"clawback_item_count":   len(clawbackIDs),
		"adjustment_item_count": len(adjustmentIDs),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func resolveBatchCurrency(groups []settlementGroup) string {
	for _, group := range groups {
		if group.currency != "" {
			return group.currency
		}
	}
	return "CNY"
}

func mergeReasonMetadata(existing, reason string) string {
	if reason == "" {
		return existing
	}
	payload := map[string]any{"reason": reason}
	if existing != "" {
		payload["previous_metadata"] = existing
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"reason":%q}`, reason)
	}
	return string(raw)
}
