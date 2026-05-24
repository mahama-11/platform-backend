package metering

import (
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) quotaAvailable(tx *gorm.DB, subjectType, subjectID, billableItemCode string) (int64, error) {
	balance, err := s.ensureQuotaBalanceTx(tx, subjectType, subjectID, billableItemCode)
	if err != nil {
		return 0, err
	}
	return balance.AvailableUnits, nil
}

func (s *Service) ensureQuotaBalanceTx(tx *gorm.DB, subjectType, subjectID, billableItemCode string) (*models.QuotaBalance, error) {
	now := time.Now()
	seed := &models.QuotaBalance{
		ID:                 utils.GenerateID(),
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		BillableItemCode:   billableItemCode,
		AvailableUnits:     0,
		LedgerSyncedAt:     now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(seed).Error; err != nil {
		return nil, err
	}
	var balance models.QuotaBalance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("billing_subject_type = ? AND billing_subject_id = ? AND billable_item_code = ?", subjectType, subjectID, billableItemCode).
		First(&balance).Error; err != nil {
		return nil, err
	}

	granted, err := s.sumQuotaDirection(tx, subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionGrant)
	if err != nil {
		return nil, err
	}
	consumed, err := s.sumQuotaDirection(tx, subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionConsume)
	if err != nil {
		return nil, err
	}
	refunded, err := s.sumQuotaDirection(tx, subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionRefund)
	if err != nil {
		return nil, err
	}
	reserved, err := s.sumReserved(tx, platformconst.ResourceTypeQuota, subjectType, subjectID, billableItemCode)
	if err != nil {
		return nil, err
	}
	balance.AvailableUnits = granted + refunded - consumed - reserved
	balance.LedgerSyncedAt = now
	balance.UpdatedAt = now
	if err := tx.Save(&balance).Error; err != nil {
		return nil, err
	}
	return &balance, nil
}

func (s *Service) consumeQuota(tx *gorm.DB, subjectType, subjectID, billableItemCode string, units int64, referenceID, reason string) error {
	balance, err := s.ensureQuotaBalanceTx(tx, subjectType, subjectID, billableItemCode)
	if err != nil {
		return err
	}
	if balance.AvailableUnits < units {
		return ErrInsufficientQuotaBalance
	}
	updated := tx.Model(&models.QuotaBalance{}).
		Where("id = ? AND available_units >= ?", balance.ID, units).
		Updates(map[string]any{
			"available_units": gorm.Expr("available_units - ?", units),
			"updated_at":      time.Now(),
		})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
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
