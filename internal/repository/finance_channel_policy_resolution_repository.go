package repository

import (
	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
	"time"
)

func (r *FinanceRepository) CreateChannelCommissionPolicyVersion(item *models.ChannelCommissionPolicyVersion) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) FindChannelCommissionPolicyVersionByID(id string) (*models.ChannelCommissionPolicyVersion, error) {
	var item models.ChannelCommissionPolicyVersion
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindChannelCommissionPolicyVersionByCode(versionCode string) (*models.ChannelCommissionPolicyVersion, error) {
	var item models.ChannelCommissionPolicyVersion
	if err := r.db.Where("version_code = ?", versionCode).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListChannelCommissionPolicyVersions(policyID, status string) ([]models.ChannelCommissionPolicyVersion, error) {
	var items []models.ChannelCommissionPolicyVersion
	q := r.db.Order("created_at desc")
	if policyID != "" {
		q = q.Where("policy_id = ?", policyID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateChannelCommissionPolicyAssignment(item *models.ChannelCommissionPolicyAssignment) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) ListChannelCommissionPolicyAssignments(policyVersionID, productCode, status string) ([]models.ChannelCommissionPolicyAssignment, error) {
	var items []models.ChannelCommissionPolicyAssignment
	q := r.db.Order("priority desc, created_at desc")
	if policyVersionID != "" {
		q = q.Where("policy_version_id = ?", policyVersionID)
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

func (r *FinanceRepository) ListCandidateChannelCommissionPolicyAssignments(productCode string, now time.Time) ([]models.ChannelCommissionPolicyAssignment, error) {
	var items []models.ChannelCommissionPolicyAssignment
	err := r.db.
		Where("product_code = ? AND status = ?", productCode, platformconst.StatusActive).
		Where("(effective_from IS NULL OR effective_from <= ?)", now).
		Where("(effective_to IS NULL OR effective_to >= ?)", now).
		Order("priority desc, created_at desc").
		Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateChannelProfitSnapshot(item *models.ChannelProfitSnapshot) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) FindChannelProfitSnapshotBySourceEventID(eventID string) (*models.ChannelProfitSnapshot, error) {
	var item models.ChannelProfitSnapshot
	if err := r.db.Where("source_event_id = ?", eventID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListChannelProfitSnapshots(productCode, orgID string) ([]models.ChannelProfitSnapshot, error) {
	var items []models.ChannelProfitSnapshot
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if orgID != "" {
		q = q.Where("org_id = ?", orgID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateChannelPolicyResolutionAudit(item *models.ChannelPolicyResolutionAudit) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) CreateChannelCommissionAdjustmentLedger(item *models.ChannelCommissionAdjustmentLedger) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) ListChannelCommissionAdjustmentLedgers(productCode, channelPartnerID, status string) ([]models.ChannelCommissionAdjustmentLedger, error) {
	var items []models.ChannelCommissionAdjustmentLedger
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
