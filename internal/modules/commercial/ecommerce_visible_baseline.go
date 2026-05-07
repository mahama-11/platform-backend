package commercial

import (
	"errors"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

const EcommerceVisibleBaselineCode = "ecommerce"

// SeedEcommerceVisibleBaseline installs the platform-owned local/dev commercial
// surface for Agent Ecommerce. It is intentionally idempotent so a local
// startup, explicit migration, or repair rerun converges the visible catalog
// without duplicating rows.
func SeedEcommerceVisibleBaseline(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		product, err := upsertVisibleProduct(tx, models.Product{
			ID:        "prd_ecommerce",
			Code:      EcommerceVisibleBaselineCode,
			Name:      "Agent Ecommerce",
			Status:    platformconst.StatusActive,
			OwnerTeam: "agent-ecommerce",
			Metadata:  `{"display_name":"Agent Ecommerce","category":"commerce","local_visible_baseline":true}`,
			CreatedAt: now,
			UpdatedAt: now,
		})
		if err != nil {
			return err
		}

		assets := []models.AssetDefinition{
			{AssetCode: "ECOMMERCE_CASH", ProductCode: product.Code, AssetType: "cash_balance", LifecycleType: platformconst.WalletLifecyclePermanent, DefaultExpireDays: 0, Status: platformconst.StatusActive, Description: "Ecommerce cash balance", Metadata: `{"currency":"CNY"}`, CreatedAt: now, UpdatedAt: now},
			{AssetCode: "ECOMMERCE_CREDIT", ProductCode: product.Code, AssetType: "wallet_credit", LifecycleType: platformconst.WalletLifecyclePermanent, DefaultExpireDays: 0, Status: platformconst.StatusActive, Description: "Ecommerce permanent credits", Metadata: `{"kind":"permanent_credit"}`, CreatedAt: now, UpdatedAt: now},
			{AssetCode: "ECOMMERCE_PROMO_CREDIT", ProductCode: product.Code, AssetType: "reward_credit", LifecycleType: platformconst.WalletLifecycleExpiring, DefaultExpireDays: 90, Status: platformconst.StatusActive, Description: "Ecommerce promotional credits", Metadata: `{"kind":"promo_credit"}`, CreatedAt: now, UpdatedAt: now},
			{AssetCode: "ECOMMERCE_MONTHLY_ALLOWANCE", ProductCode: product.Code, AssetType: "subscription_allowance", LifecycleType: platformconst.WalletLifecycleCycleReset, DefaultExpireDays: 0, ResetCycle: "monthly", Status: platformconst.StatusActive, Description: "Ecommerce monthly allowance", Metadata: `{"kind":"monthly_allowance"}`, CreatedAt: now, UpdatedAt: now},
		}
		for i := range assets {
			if err := upsertVisibleAssetDefinition(tx, assets[i]); err != nil {
				return err
			}
		}

		billable, err := upsertVisibleBillableItem(tx, models.BillableItem{
			ID: "billable_ecommerce_image_generate", ProductID: product.ID, Code: "ecommerce.image.generate", Name: "Ecommerce Image Generation",
			MeterUnit: "request", BillingScope: "organization", SettlementMode: "included_then_overage", PricingBehavior: "quota_first",
			Status: platformconst.StatusActive, Metadata: `{"description":"Generated ecommerce image request"}`, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}

		type visibleOffer struct {
			skuID       string
			skuCode     string
			skuName     string
			skuType     string
			billingMode string
			currency    string
			listPrice   int64
			packageID   string
			packageCode string
			packageName string
			packageType string
			metadata    string
			quotaUnits  int64
			grantMode   string
			resetCycle  string
			rateID      string
			rateConfig  string
		}
		offers := []visibleOffer{
			{skuID: "sku_ecommerce_sub_basic_monthly", skuCode: "ecommerce.sku.sub.basic.monthly", skuName: "Ecommerce Basic Monthly", skuType: "subscription", billingMode: "recurring", currency: "CNY", listPrice: 900, packageID: "pkg_ecommerce_sub_basic_monthly", packageCode: "ecommerce.pkg.sub.basic.monthly", packageName: "Basic Monthly", packageType: "subscription", metadata: `{"tier":"basic","monthly_quota":300}`, quotaUnits: 300, grantMode: "cycle_reset", resetCycle: "monthly", rateID: "rc_ecommerce_sub_basic_monthly", rateConfig: `{"unit_amount":900}`},
			{skuID: "sku_ecommerce_sub_pro_monthly", skuCode: "ecommerce.sku.sub.pro.monthly", skuName: "Ecommerce Pro Monthly", skuType: "subscription", billingMode: "recurring", currency: "CNY", listPrice: 9900, packageID: "pkg_ecommerce_sub_pro_monthly", packageCode: "ecommerce.pkg.sub.pro.monthly", packageName: "Pro Monthly", packageType: "subscription", metadata: `{"tier":"pro","monthly_quota":3000}`, quotaUnits: 3000, grantMode: "cycle_reset", resetCycle: "monthly", rateID: "rc_ecommerce_sub_pro_monthly", rateConfig: `{"unit_amount":9900}`},
			{skuID: "sku_ecommerce_sub_growth_monthly", skuCode: "ecommerce.sku.sub.growth.monthly", skuName: "Ecommerce Growth Monthly", skuType: "subscription", billingMode: "recurring", currency: "CNY", listPrice: 29900, packageID: "pkg_ecommerce_sub_growth_monthly", packageCode: "ecommerce.pkg.sub.growth.monthly", packageName: "Growth Monthly", packageType: "subscription", metadata: `{"tier":"growth","monthly_quota":10000}`, quotaUnits: 10000, grantMode: "cycle_reset", resetCycle: "monthly", rateID: "rc_ecommerce_sub_growth_monthly", rateConfig: `{"unit_amount":29900}`},
			{skuID: "sku_ecommerce_pack_permanent_basic", skuCode: "ecommerce.sku.pack.permanent.basic", skuName: "Ecommerce Permanent Basic Pack", skuType: "resource_pack", billingMode: "one_time", currency: "CNY", listPrice: 1900, packageID: "pkg_ecommerce_pack_permanent_basic", packageCode: "ecommerce.pkg.pack.permanent.basic", packageName: "Permanent Basic Pack", packageType: "permanent_pack", metadata: `{"tier":"permanent_basic","quota_units":200}`, quotaUnits: 200, grantMode: "one_time", rateID: "rc_ecommerce_pack_permanent_basic", rateConfig: `{"unit_amount":1900}`},
			{skuID: "sku_ecommerce_pack_promo_basic", skuCode: "ecommerce.sku.pack.promo.basic", skuName: "Ecommerce Promo Basic Pack", skuType: "promo_pack", billingMode: "one_time", currency: "CNY", listPrice: 900, packageID: "pkg_ecommerce_pack_promo_basic", packageCode: "ecommerce.pkg.pack.promo.basic", packageName: "Promo Basic Pack", packageType: "promo_pack", metadata: `{"tier":"promo_basic","quota_units":100,"campaign":"launch_discount"}`, quotaUnits: 100, grantMode: "one_time", rateID: "rc_ecommerce_pack_promo_basic", rateConfig: `{"unit_amount":900,"original_unit_amount":1900}`},
		}
		for _, offer := range offers {
			sku, err := upsertVisibleSKU(tx, models.SKU{ID: offer.skuID, ProductID: product.ID, Code: offer.skuCode, Name: offer.skuName, SKUType: offer.skuType, BillingMode: offer.billingMode, Currency: offer.currency, ListPrice: offer.listPrice, Status: platformconst.StatusActive, Metadata: `{"package_code":"` + offer.packageCode + `"}`, CreatedAt: now, UpdatedAt: now})
			if err != nil {
				return err
			}
			pkg, err := upsertVisiblePackage(tx, models.CommercialPackage{ID: offer.packageID, ProductID: product.ID, Code: offer.packageCode, Name: offer.packageName, PackageType: offer.packageType, Status: platformconst.StatusActive, Metadata: offer.metadata, CreatedAt: now, UpdatedAt: now})
			if err != nil {
				return err
			}
			if _, err := upsertVisibleRateCard(tx, models.RateCard{ID: offer.rateID, ProductID: product.ID, Code: offer.skuCode + ".v1", TargetType: "sku", TargetID: sku.ID, PriceModel: "flat", Currency: offer.currency, PriceConfig: offer.rateConfig, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"` + pkg.Code + `"}`, CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			if _, err := upsertVisibleQuotaGrantPolicy(tx, models.QuotaGrantPolicy{ID: "quota_policy_" + pkg.ID, ProductCode: product.Code, PackageCode: pkg.Code, BillableItemCode: billable.Code, GrantMode: offer.grantMode, Units: offer.quotaUnits, ResetCycle: offer.resetCycle, Status: platformconst.StatusActive, Metadata: offer.metadata, CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
		}
		_, err = upsertVisibleRateCard(tx, models.RateCard{
			ID: "rc_ecommerce_image_generate_overage_v1", ProductID: product.ID, Code: "ecommerce.image.generate.overage.v1",
			TargetType: "billable_item", TargetID: billable.ID, PriceModel: "flat", Currency: "ECOMMERCE_CREDIT", PriceConfig: `{"unit_amount":10}`,
			Version: 1, Status: platformconst.StatusActive, Metadata: `{"wallet_asset_code":"ECOMMERCE_CREDIT"}`, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
}

func upsertVisibleProduct(db *gorm.DB, item models.Product) (*models.Product, error) {
	var existing models.Product
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err == nil {
		existing.Name = item.Name
		existing.Status = item.Status
		existing.OwnerTeam = item.OwnerTeam
		existing.Metadata = item.Metadata
		existing.UpdatedAt = item.UpdatedAt
		return &existing, db.Save(&existing).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &item, db.Create(&item).Error
}

func upsertVisibleSKU(db *gorm.DB, item models.SKU) (*models.SKU, error) {
	var existing models.SKU
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err == nil {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		return &item, db.Save(&item).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &item, db.Create(&item).Error
}

func upsertVisiblePackage(db *gorm.DB, item models.CommercialPackage) (*models.CommercialPackage, error) {
	var existing models.CommercialPackage
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err == nil {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		return &item, db.Save(&item).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &item, db.Create(&item).Error
}

func upsertVisibleBillableItem(db *gorm.DB, item models.BillableItem) (*models.BillableItem, error) {
	var existing models.BillableItem
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err == nil {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		return &item, db.Save(&item).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &item, db.Create(&item).Error
}

func upsertVisibleRateCard(db *gorm.DB, item models.RateCard) (*models.RateCard, error) {
	var existing models.RateCard
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err == nil {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		return &item, db.Save(&item).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &item, db.Create(&item).Error
}

func upsertVisibleAssetDefinition(db *gorm.DB, item models.AssetDefinition) error {
	var existing models.AssetDefinition
	if err := db.Where("asset_code = ?", item.AssetCode).First(&existing).Error; err == nil {
		item.CreatedAt = existing.CreatedAt
		return db.Save(&item).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&item).Error
}

func upsertVisibleQuotaGrantPolicy(db *gorm.DB, item models.QuotaGrantPolicy) (*models.QuotaGrantPolicy, error) {
	var existing models.QuotaGrantPolicy
	err := db.Where("product_code = ? AND package_code = ? AND billable_item_code = ?", item.ProductCode, item.PackageCode, item.BillableItemCode).First(&existing).Error
	if err == nil {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		return &item, db.Save(&item).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &item, db.Create(&item).Error
}
