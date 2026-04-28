package repository

import (
	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

type ControlRepository struct {
	db *gorm.DB
}

func NewControlRepository(db *gorm.DB) *ControlRepository {
	return &ControlRepository{db: db}
}

func (r *ControlRepository) DB() *gorm.DB { return r.db }

func (r *ControlRepository) CreateQuotaLedger(item *models.QuotaLedger) error {
	return r.db.Create(item).Error
}

func (r *ControlRepository) FindQuotaLedgerByReference(subjectType, subjectID, billableItemCode, direction, referenceID string) (*models.QuotaLedger, error) {
	var item models.QuotaLedger
	if err := r.db.Where("billing_subject_type = ? AND billing_subject_id = ? AND billable_item_code = ? AND direction = ? AND reference_id = ?", subjectType, subjectID, billableItemCode, direction, referenceID).
		Order("created_at desc").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ControlRepository) CreateReservation(item *models.ResourceReservation) error {
	return r.db.Create(item).Error
}

func (r *ControlRepository) ListQuotaGrantPolicies(productCode, packageCode string) ([]models.QuotaGrantPolicy, error) {
	var items []models.QuotaGrantPolicy
	query := r.db.Model(&models.QuotaGrantPolicy{}).Order("created_at ASC")
	if productCode != "" {
		query = query.Where("product_code = ?", productCode)
	}
	if packageCode != "" {
		query = query.Where("package_code = ?", packageCode)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ControlRepository) FindQuotaGrantPolicyByID(id string) (*models.QuotaGrantPolicy, error) {
	var item models.QuotaGrantPolicy
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ControlRepository) FindQuotaGrantPolicyByKey(productCode, packageCode, billableItemCode string) (*models.QuotaGrantPolicy, error) {
	var item models.QuotaGrantPolicy
	if err := r.db.Where("product_code = ? AND package_code = ? AND billable_item_code = ?", productCode, packageCode, billableItemCode).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ControlRepository) CreateQuotaGrantPolicy(item *models.QuotaGrantPolicy) error {
	return r.db.Create(item).Error
}

func (r *ControlRepository) SaveQuotaGrantPolicy(item *models.QuotaGrantPolicy) error {
	return r.db.Save(item).Error
}

func (r *ControlRepository) DeleteQuotaGrantPolicy(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.QuotaGrantPolicy{}).Error
}

func (r *ControlRepository) ListPackageCapabilityPolicies(productCode, packageCode string) ([]models.PackageCapabilityPolicy, error) {
	var items []models.PackageCapabilityPolicy
	query := r.db.Model(&models.PackageCapabilityPolicy{}).Order("created_at ASC")
	if productCode != "" {
		query = query.Where("product_code = ?", productCode)
	}
	if packageCode != "" {
		query = query.Where("package_code = ?", packageCode)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ControlRepository) FindPackageCapabilityPolicyByID(id string) (*models.PackageCapabilityPolicy, error) {
	var item models.PackageCapabilityPolicy
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ControlRepository) FindPackageCapabilityPolicyByKey(productCode, packageCode, capabilityCode string) (*models.PackageCapabilityPolicy, error) {
	var item models.PackageCapabilityPolicy
	if err := r.db.Where("product_code = ? AND package_code = ? AND capability_code = ?", productCode, packageCode, capabilityCode).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ControlRepository) CreatePackageCapabilityPolicy(item *models.PackageCapabilityPolicy) error {
	return r.db.Create(item).Error
}

func (r *ControlRepository) SavePackageCapabilityPolicy(item *models.PackageCapabilityPolicy) error {
	return r.db.Save(item).Error
}

func (r *ControlRepository) DeletePackageCapabilityPolicy(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.PackageCapabilityPolicy{}).Error
}

func (r *ControlRepository) CreateCapabilityGrant(item *models.CapabilityGrant) error {
	return r.db.Create(item).Error
}

func (r *ControlRepository) SaveCapabilityGrant(item *models.CapabilityGrant) error {
	return r.db.Save(item).Error
}

func (r *ControlRepository) ListCapabilityGrants(productCode, subjectType, subjectID, capabilityCode string) ([]models.CapabilityGrant, error) {
	var items []models.CapabilityGrant
	query := r.db.Model(&models.CapabilityGrant{}).
		Where("billing_subject_type = ? AND billing_subject_id = ? AND status = ?", subjectType, subjectID, platformconst.StatusActive).
		Order("updated_at DESC, created_at DESC")
	if productCode != "" {
		query = query.Where("product_code = ?", productCode)
	}
	if capabilityCode != "" {
		query = query.Where("capability_code = ?", capabilityCode)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ControlRepository) FindCapabilityGrantBySource(productCode, subjectType, subjectID, capabilityCode, sourceType, sourceID string) (*models.CapabilityGrant, error) {
	var item models.CapabilityGrant
	if err := r.db.Where("product_code = ? AND billing_subject_type = ? AND billing_subject_id = ? AND capability_code = ? AND source_type = ? AND source_id = ?", productCode, subjectType, subjectID, capabilityCode, sourceType, sourceID).
		Order("created_at desc").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ControlRepository) FindReservationByID(id string) (*models.ResourceReservation, error) {
	var item models.ResourceReservation
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ControlRepository) FindReservationByKey(key string) (*models.ResourceReservation, error) {
	var item models.ResourceReservation
	if err := r.db.Where("reservation_key = ?", key).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ControlRepository) SaveReservation(item *models.ResourceReservation) error {
	return r.db.Save(item).Error
}

func (r *ControlRepository) SumQuotaDirection(subjectType, subjectID, billableItemCode, direction string) (int64, error) {
	var total int64
	err := r.db.Model(&models.QuotaLedger{}).
		Select("COALESCE(SUM(units), 0)").
		Where("billing_subject_type = ? AND billing_subject_id = ? AND billable_item_code = ? AND direction = ?", subjectType, subjectID, billableItemCode, direction).
		Scan(&total).Error
	return total, err
}

func (r *ControlRepository) SumReserved(resourceType, subjectType, subjectID, billableItemCode string) (int64, error) {
	q := r.db.Model(&models.ResourceReservation{}).
		Select("COALESCE(SUM(units), 0)").
		Where("resource_type = ? AND billing_subject_type = ? AND billing_subject_id = ? AND status = ?", resourceType, subjectType, subjectID, platformconst.ReservationStatusReserved)
	if billableItemCode != "" {
		q = q.Where("billable_item_code = ?", billableItemCode)
	}
	var total int64
	err := q.Scan(&total).Error
	return total, err
}
