package repository

import (
	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

type RuntimeRepository struct {
	db *gorm.DB
}

type RuntimeJobListFilter struct {
	OrganizationID string
	Status         string
	Stage          string
	Query          string
	Limit          int
	Offset         int
}

type ChargeSessionListFilter struct {
	OrganizationID string
	Status         string
	ProductCode    string
	Query          string
	Limit          int
	Offset         int
}

func NewRuntimeRepository(db *gorm.DB) *RuntimeRepository {
	return &RuntimeRepository{db: db}
}

func (r *RuntimeRepository) DB() *gorm.DB { return r.db }

func (r *RuntimeRepository) CreateProviderDefinition(item *models.RuntimeProviderDefinition) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) FindProviderDefinitionByCode(code string) (*models.RuntimeProviderDefinition, error) {
	var item models.RuntimeProviderDefinition
	if err := r.db.Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) ListProviderDefinitions() ([]models.RuntimeProviderDefinition, error) {
	var items []models.RuntimeProviderDefinition
	if err := r.db.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *RuntimeRepository) CreateProductEndpoint(item *models.RuntimeProductEndpoint) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) SaveProductEndpoint(item *models.RuntimeProductEndpoint) error {
	return r.db.Save(item).Error
}

func (r *RuntimeRepository) FindActiveProductEndpoint(productCode string) (*models.RuntimeProductEndpoint, error) {
	var item models.RuntimeProductEndpoint
	if err := r.db.Where("product_code = ? AND status = ?", productCode, platformconst.StatusActive).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) CreateProviderBinding(item *models.RuntimeProviderBinding) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) SaveProviderBinding(item *models.RuntimeProviderBinding) error {
	return r.db.Save(item).Error
}

func (r *RuntimeRepository) FindProviderBinding(productCode, taskType, providerCode string) (*models.RuntimeProviderBinding, error) {
	var item models.RuntimeProviderBinding
	if err := r.db.Where("product_code = ? AND task_type = ? AND provider_code = ?", productCode, taskType, providerCode).Order("created_at ASC").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) FindPreferredProviderBinding(productCode, taskType string) (*models.RuntimeProviderBinding, error) {
	var item models.RuntimeProviderBinding
	if err := r.db.Where("product_code = ? AND task_type = ? AND enabled = ?", productCode, taskType, true).
		Order("priority ASC, created_at ASC").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) ListProviderBindings(productCode, taskType string) ([]models.RuntimeProviderBinding, error) {
	var items []models.RuntimeProviderBinding
	if err := r.db.Where("product_code = ? AND task_type = ? AND enabled = ?", productCode, taskType, true).
		Order("priority ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *RuntimeRepository) ListAllProviderBindings(productCode, taskType string) ([]models.RuntimeProviderBinding, error) {
	var items []models.RuntimeProviderBinding
	if err := r.db.Where("product_code = ? AND task_type = ?", productCode, taskType).
		Order("priority ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *RuntimeRepository) CreateStorageBinding(item *models.StorageBinding) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) SaveStorageBinding(item *models.StorageBinding) error {
	return r.db.Save(item).Error
}

func (r *RuntimeRepository) FindStorageAssetByStorageKey(storageKey string) (*models.StorageAsset, error) {
	var item models.StorageAsset
	if err := r.db.Where("storage_key = ?", storageKey).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) FindStorageAssetBySource(productCode, category, sourceType, sourceRef string) (*models.StorageAsset, error) {
	var item models.StorageAsset
	if err := r.db.
		Where("product_code = ? AND category = ? AND source_type = ? AND source_ref = ?", productCode, category, sourceType, sourceRef).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) CreateStorageAsset(item *models.StorageAsset) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) UpdateStorageAsset(item *models.StorageAsset) error {
	return r.db.Save(item).Error
}

func (r *RuntimeRepository) FindPreferredStorageBinding(productCode, category string) (*models.StorageBinding, error) {
	var item models.StorageBinding
	if err := r.db.Where("product_code = ? AND category = ? AND enabled = ?", productCode, category, true).
		Order("priority ASC, created_at ASC").
		First(&item).Error; err == nil {
		return &item, nil
	}
	if err := r.db.Where("product_code = ? AND category = ? AND enabled = ?", productCode, "*", true).
		Order("priority ASC, created_at ASC").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) ListStorageBindings(productCode string) ([]models.StorageBinding, error) {
	var items []models.StorageBinding
	if err := r.db.Where("product_code = ? AND enabled = ?", productCode, true).
		Order("priority ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *RuntimeRepository) CreateRuntimeJob(item *models.RuntimeJob) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) FindRuntimeJobByID(id string) (*models.RuntimeJob, error) {
	var item models.RuntimeJob
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) FindRuntimeJobByIdempotencyKey(productCode, organizationID, sourceType, sourceID, taskType, key string) (*models.RuntimeJob, error) {
	var item models.RuntimeJob
	if err := r.db.Where(
		"product_code = ? AND organization_id = ? AND source_type = ? AND source_id = ? AND task_type = ? AND idempotency_key = ?",
		productCode, organizationID, sourceType, sourceID, taskType, key,
	).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) SaveRuntimeJob(item *models.RuntimeJob) error {
	return r.db.Save(item).Error
}

func (r *RuntimeRepository) ListRuntimeJobs(filter RuntimeJobListFilter) ([]models.RuntimeJob, int64, error) {
	query := r.db.Model(&models.RuntimeJob{})
	if filter.OrganizationID != "" {
		query = query.Where("organization_id = ?", filter.OrganizationID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Stage != "" {
		query = query.Where("stage = ?", filter.Stage)
	}
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		query = query.Where(
			"id LIKE ? OR provider_job_id LIKE ? OR source_id LIKE ? OR source_type LIKE ? OR task_type LIKE ? OR product_code LIKE ? OR status LIKE ? OR stage LIKE ?",
			like, like, like, like, like, like, like, like,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.RuntimeJob
	if err := query.Order("created_at DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *RuntimeRepository) ListRuntimeAttempts(runtimeJobID string) ([]models.RuntimeAttempt, error) {
	var items []models.RuntimeAttempt
	if err := r.db.Where("runtime_job_id = ?", runtimeJobID).Order("attempt_no ASC, created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *RuntimeRepository) CreateRuntimeAttempt(item *models.RuntimeAttempt) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) CreateChargeSession(item *models.ChargeSession) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) CreateCallbackDelivery(item *models.RuntimeCallbackDelivery) error {
	return r.db.Create(item).Error
}

func (r *RuntimeRepository) SaveCallbackDelivery(item *models.RuntimeCallbackDelivery) error {
	return r.db.Save(item).Error
}

func (r *RuntimeRepository) FindCallbackDeliveryByID(id string) (*models.RuntimeCallbackDelivery, error) {
	var item models.RuntimeCallbackDelivery
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) FindChargeSessionByID(id string) (*models.ChargeSession, error) {
	var item models.ChargeSession
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) FindChargeSessionByReservationKey(reservationKey string) (*models.ChargeSession, error) {
	var item models.ChargeSession
	if err := r.db.Where("reservation_key = ?", reservationKey).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) FindChargeSessionBySource(sourceType, sourceID string) (*models.ChargeSession, error) {
	var item models.ChargeSession
	if err := r.db.Where("source_type = ? AND source_id = ?", sourceType, sourceID).Order("created_at DESC").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RuntimeRepository) ListChargeSessions(filter ChargeSessionListFilter) ([]models.ChargeSession, int64, error) {
	query := r.db.Model(&models.ChargeSession{})
	if filter.OrganizationID != "" {
		query = query.Where("organization_id = ?", filter.OrganizationID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.ProductCode != "" {
		query = query.Where("product_code = ?", filter.ProductCode)
	}
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		query = query.Where(
			"id LIKE ? OR source_id LIKE ? OR source_type LIKE ? OR reservation_key LIKE ? OR event_id LIKE ? OR settlement_id LIKE ? OR reservation_id LIKE ? OR finalization_id LIKE ?",
			like, like, like, like, like, like, like, like,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.ChargeSession
	if err := query.Order("created_at DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *RuntimeRepository) SaveChargeSession(item *models.ChargeSession) error {
	return r.db.Save(item).Error
}
