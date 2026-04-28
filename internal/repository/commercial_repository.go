package repository

import (
	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

type CommercialRepository struct {
	db *gorm.DB
}

func NewCommercialRepository(db *gorm.DB) *CommercialRepository {
	return &CommercialRepository{db: db}
}

func (r *CommercialRepository) DB() *gorm.DB { return r.db }

func (r *CommercialRepository) CreateProduct(item *models.Product) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) FindProductByID(id string) (*models.Product, error) {
	var item models.Product
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) FindProductByCode(code string) (*models.Product, error) {
	var item models.Product
	if err := r.db.Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) SaveProduct(item *models.Product) error {
	return r.db.Save(item).Error
}

func (r *CommercialRepository) DeleteProduct(id string) error {
	return r.db.Delete(&models.Product{}, "id = ?", id).Error
}

func (r *CommercialRepository) ListProducts() ([]models.Product, error) {
	var items []models.Product
	err := r.db.Order("created_at desc").Find(&items).Error
	return items, err
}

func (r *CommercialRepository) CreateSKU(item *models.SKU) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) FindSKUByID(id string) (*models.SKU, error) {
	var item models.SKU
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) SaveSKU(item *models.SKU) error {
	return r.db.Save(item).Error
}

func (r *CommercialRepository) DeleteSKU(id string) error {
	return r.db.Delete(&models.SKU{}, "id = ?", id).Error
}

func (r *CommercialRepository) ListSKUs(productID string) ([]models.SKU, error) {
	var items []models.SKU
	q := r.db.Order("created_at desc")
	if productID != "" {
		q = q.Where("product_id = ?", productID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *CommercialRepository) CreateBillableItem(item *models.BillableItem) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) FindBillableItemByID(id string) (*models.BillableItem, error) {
	var item models.BillableItem
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) FindBillableItemByCode(code string) (*models.BillableItem, error) {
	var item models.BillableItem
	if err := r.db.Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) SaveBillableItem(item *models.BillableItem) error {
	return r.db.Save(item).Error
}

func (r *CommercialRepository) DeleteBillableItem(id string) error {
	return r.db.Delete(&models.BillableItem{}, "id = ?", id).Error
}

func (r *CommercialRepository) ListBillableItems(productID string) ([]models.BillableItem, error) {
	var items []models.BillableItem
	q := r.db.Order("created_at desc")
	if productID != "" {
		q = q.Where("product_id = ?", productID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *CommercialRepository) CreateCommercialEntity(item *models.CommercialEntity) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) FindCommercialEntityByID(id string) (*models.CommercialEntity, error) {
	var item models.CommercialEntity
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) SaveCommercialEntity(item *models.CommercialEntity) error {
	return r.db.Save(item).Error
}

func (r *CommercialRepository) DeleteCommercialEntity(id string) error {
	return r.db.Delete(&models.CommercialEntity{}, "id = ?", id).Error
}

func (r *CommercialRepository) ListCommercialEntities() ([]models.CommercialEntity, error) {
	var items []models.CommercialEntity
	err := r.db.Order("created_at desc").Find(&items).Error
	return items, err
}

func (r *CommercialRepository) CreateBillingProfile(item *models.BillingProfile) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) ListBillingProfiles(productID string) ([]models.BillingProfile, error) {
	var items []models.BillingProfile
	q := r.db.Order("created_at desc")
	if productID != "" {
		q = q.Where("product_id = ?", productID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *CommercialRepository) FindBillingProfileByCode(code string) (*models.BillingProfile, error) {
	var item models.BillingProfile
	if err := r.db.Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) FindBillingProfileByID(id string) (*models.BillingProfile, error) {
	var item models.BillingProfile
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) SaveBillingProfile(item *models.BillingProfile) error {
	return r.db.Save(item).Error
}

func (r *CommercialRepository) DeleteBillingProfile(id string) error {
	return r.db.Delete(&models.BillingProfile{}, "id = ?", id).Error
}

func (r *CommercialRepository) CreatePackage(item *models.CommercialPackage) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) ListPackages(productID string) ([]models.CommercialPackage, error) {
	var items []models.CommercialPackage
	q := r.db.Order("created_at desc")
	if productID != "" {
		q = q.Where("product_id = ?", productID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *CommercialRepository) FindPackageByID(id string) (*models.CommercialPackage, error) {
	var item models.CommercialPackage
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) SavePackage(item *models.CommercialPackage) error {
	return r.db.Save(item).Error
}

func (r *CommercialRepository) DeletePackage(id string) error {
	return r.db.Delete(&models.CommercialPackage{}, "id = ?", id).Error
}

func (r *CommercialRepository) CreateRateCard(item *models.RateCard) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) ListRateCards(productID, targetType string) ([]models.RateCard, error) {
	var items []models.RateCard
	q := r.db.Order("created_at desc")
	if productID != "" {
		q = q.Where("product_id = ?", productID)
	}
	if targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *CommercialRepository) FindRateCardByID(id string) (*models.RateCard, error) {
	var item models.RateCard
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) FindActiveRateCard(targetType, targetID string, at any) (*models.RateCard, error) {
	var item models.RateCard
	q := r.db.Where("target_type = ? AND target_id = ? AND status = ?", targetType, targetID, platformconst.StatusActive).
		Order("version desc, created_at desc")
	if at != nil {
		q = q.Where("(effective_from IS NULL OR effective_from <= ?) AND (effective_to IS NULL OR effective_to >= ?)", at, at)
	}
	if err := q.First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) SaveRateCard(item *models.RateCard) error {
	return r.db.Save(item).Error
}

func (r *CommercialRepository) DeleteRateCard(id string) error {
	return r.db.Delete(&models.RateCard{}, "id = ?", id).Error
}

func (r *CommercialRepository) ListRoutingPolicies(billingProfileID string) ([]models.RoutingPolicy, error) {
	var items []models.RoutingPolicy
	q := r.db.Order("priority asc, created_at asc")
	if billingProfileID != "" {
		q = q.Where("billing_profile_id = ?", billingProfileID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *CommercialRepository) CreateRoutingPolicy(item *models.RoutingPolicy) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) FindRoutingPolicyByID(id string) (*models.RoutingPolicy, error) {
	var item models.RoutingPolicy
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) SaveRoutingPolicy(item *models.RoutingPolicy) error {
	return r.db.Save(item).Error
}

func (r *CommercialRepository) DeleteRoutingPolicy(id string) error {
	return r.db.Delete(&models.RoutingPolicy{}, "id = ?", id).Error
}

func (r *CommercialRepository) FindOrgBillingProfile(orgID string) (*models.OrgBillingProfile, error) {
	var item models.OrgBillingProfile
	if err := r.db.Where("organization_id = ? AND status = ?", orgID, "active").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) CreateMeterEvent(item *models.MeterEvent) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) FindMeterEventByEventID(eventID string) (*models.MeterEvent, error) {
	var item models.MeterEvent
	if err := r.db.Where("event_id = ?", eventID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) CreateUsageRecord(item *models.UsageRecord) error {
	return r.db.Create(item).Error
}

func (r *CommercialRepository) CreateBillingLedger(item *models.BillingLedger) error {
	return r.db.Create(item).Error
}
