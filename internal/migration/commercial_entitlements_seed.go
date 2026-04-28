package migration

import (
	"errors"

	"platform-service/internal/models"

	"gorm.io/gorm"
)

func upsertQuotaGrantPolicy(db *gorm.DB, item models.QuotaGrantPolicy) (*models.QuotaGrantPolicy, error) {
	if err := ensureCommercialEntitlementSchema(db); err != nil {
		return nil, err
	}
	var existing models.QuotaGrantPolicy
	if err := db.Where("package_code = ? AND billable_item_code = ?", item.PackageCode, item.BillableItemCode).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &item, db.Create(&item).Error
		}
		return nil, err
	}
	existing.ProductCode = item.ProductCode
	existing.GrantMode = item.GrantMode
	existing.Units = item.Units
	existing.ResetCycle = item.ResetCycle
	existing.Status = item.Status
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return &existing, db.Save(&existing).Error
}

func upsertPackageCapabilityPolicy(db *gorm.DB, item models.PackageCapabilityPolicy) (*models.PackageCapabilityPolicy, error) {
	if err := ensureCommercialEntitlementSchema(db); err != nil {
		return nil, err
	}
	var existing models.PackageCapabilityPolicy
	if err := db.Where("package_code = ? AND capability_code = ?", item.PackageCode, item.CapabilityCode).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &item, db.Create(&item).Error
		}
		return nil, err
	}
	existing.ProductCode = item.ProductCode
	existing.GrantValue = item.GrantValue
	existing.Status = item.Status
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return &existing, db.Save(&existing).Error
}

func ensureCommercialEntitlementSchema(db *gorm.DB) error {
	return db.AutoMigrate(&models.QuotaGrantPolicy{}, &models.PackageCapabilityPolicy{}, &models.CapabilityGrant{})
}
