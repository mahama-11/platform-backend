package repository

import (
	"platform-service/internal/models"
	"time"

	"platform-service/pkg/platformconst"
	"gorm.io/gorm"
)


func (r *FinanceRepository) CreateChannelPartner(item *models.ChannelPartner) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) FindChannelPartnerByID(id string) (*models.ChannelPartner, error) {
	var item models.ChannelPartner
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindChannelPartnerByCode(code string) (*models.ChannelPartner, error) {
	var item models.ChannelPartner
	if err := r.db.Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListChannelPartners(status string) ([]models.ChannelPartner, error) {
	var items []models.ChannelPartner
	q := r.db.Order("created_at desc")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateChannelProgram(item *models.ChannelProgram) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) FindChannelProgramByID(id string) (*models.ChannelProgram, error) {
	var item models.ChannelProgram
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindChannelProgramByCode(code string) (*models.ChannelProgram, error) {
	var item models.ChannelProgram
	if err := r.db.Where("program_code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListChannelPrograms(productCode, status string) ([]models.ChannelProgram, error) {
	var items []models.ChannelProgram
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateChannelBinding(item *models.ChannelPartnerBinding) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveChannelBinding(item *models.ChannelPartnerBinding) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindActiveChannelBinding(productCode, orgID string, now time.Time) (*models.ChannelPartnerBinding, error) {
	var item models.ChannelPartnerBinding
	err := r.db.
		Where("product_code = ? AND org_id = ? AND status = ?", productCode, orgID, platformconst.StatusActive).
		Where("(effective_from IS NULL OR effective_from <= ?)", now).
		Where("(effective_to IS NULL OR effective_to >= ?)", now).
		Order("created_at desc").
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListChannelBindings(productCode, orgID, status string) ([]models.ChannelPartnerBinding, error) {
	var items []models.ChannelPartnerBinding
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if orgID != "" {
		q = q.Where("org_id = ?", orgID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateChannelCommissionPolicy(item *models.ChannelCommissionPolicy) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) FindChannelCommissionPolicyByCode(policyCode string) (*models.ChannelCommissionPolicy, error) {
	var item models.ChannelCommissionPolicy
	if err := r.db.Where("policy_code = ?", policyCode).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindChannelCommissionPolicyByID(id string) (*models.ChannelCommissionPolicy, error) {
	var item models.ChannelCommissionPolicy
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindApplicableChannelCommissionPolicy(channelProgramID, productCode, appliesTo string, now time.Time) (*models.ChannelCommissionPolicy, error) {
	var item models.ChannelCommissionPolicy
	err := r.db.
		Where("channel_program_id = ? AND product_code = ? AND applies_to = ? AND status = ?", channelProgramID, productCode, appliesTo, platformconst.StatusActive).
		Where("(effective_from IS NULL OR effective_from <= ?)", now).
		Where("(effective_to IS NULL OR effective_to >= ?)", now).
		Order("priority asc, created_at asc").
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListChannelCommissionPolicies(channelProgramID, productCode, status string) ([]models.ChannelCommissionPolicy, error) {
	var items []models.ChannelCommissionPolicy
	q := r.db.Order("priority asc, created_at asc")
	if channelProgramID != "" {
		q = q.Where("channel_program_id = ?", channelProgramID)
	}
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateChannelCommissionLedger(item *models.ChannelCommissionLedger) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveChannelCommissionLedger(item *models.ChannelCommissionLedger) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindChannelCommissionLedgerByID(id string) (*models.ChannelCommissionLedger, error) {
	var item models.ChannelCommissionLedger
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindChannelCommissionLedgerBySourceEventID(eventID string) (*models.ChannelCommissionLedger, error) {
	var item models.ChannelCommissionLedger
	if err := r.db.Where("source_event_id = ?", eventID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindChannelCommissionLedgerByReversalEventID(eventID string) (*models.ChannelCommissionLedger, error) {
	var item models.ChannelCommissionLedger
	if err := r.db.Where("reversal_event_id = ?", eventID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindChannelCommissionLedgerBySourceChargeID(productCode, sourceChargeID string) (*models.ChannelCommissionLedger, error) {
	var item models.ChannelCommissionLedger
	if err := r.db.Where("product_code = ? AND source_charge_id = ?", productCode, sourceChargeID).Order("created_at desc").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListChannelCommissionLedgers(productCode, channelPartnerID, status string) ([]models.ChannelCommissionLedger, error) {
	var items []models.ChannelCommissionLedger
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

func (r *FinanceRepository) CreateChannelClawbackLedger(item *models.ChannelClawbackLedger) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) FindChannelClawbackLedgerBySourceRefundEventID(eventID string) (*models.ChannelClawbackLedger, error) {
	var item models.ChannelClawbackLedger
	if err := r.db.Where("source_refund_event_id = ?", eventID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func isNotFound(err error) bool {
	return err != nil && err == gorm.ErrRecordNotFound
}
