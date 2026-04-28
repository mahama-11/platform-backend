package migration

import (
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

func seedEcommerceOfferings(db *gorm.DB) error {
	now := time.Now().UTC()
	product := models.Product{
		ID:        "prd_ecommerce",
		Code:      "ecommerce",
		Name:      "Ecommerce",
		Status:    platformconst.StatusActive,
		OwnerTeam: "growth",
		Metadata:  `{"display_name":"Ecommerce","category":"commerce"}`,
		CreatedAt: now,
		UpdatedAt: now,
	}
	savedProduct, err := upsertProduct(db, product)
	if err != nil {
		return err
	}
	if savedProduct != nil && savedProduct.ID != "" {
		product.ID = savedProduct.ID
	}
	assets := []models.AssetDefinition{
		{AssetCode: "ECOMMERCE_CASH", ProductCode: product.Code, AssetType: "cash_balance", LifecycleType: platformconst.WalletLifecyclePermanent, DefaultExpireDays: 0, Status: platformconst.StatusActive, Description: "Ecommerce 现金余额", Metadata: `{"currency":"CNY"}`, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "ECOMMERCE_CREDIT", ProductCode: product.Code, AssetType: "wallet_credit", LifecycleType: platformconst.WalletLifecyclePermanent, DefaultExpireDays: 0, Status: platformconst.StatusActive, Description: "Ecommerce 永久积分", Metadata: `{"kind":"permanent_credit"}`, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "ECOMMERCE_PROMO_CREDIT", ProductCode: product.Code, AssetType: "reward_credit", LifecycleType: platformconst.WalletLifecycleExpiring, DefaultExpireDays: 90, Status: platformconst.StatusActive, Description: "Ecommerce 活动积分", Metadata: `{"kind":"promo_credit"}`, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "ECOMMERCE_MONTHLY_ALLOWANCE", ProductCode: product.Code, AssetType: "subscription_allowance", LifecycleType: platformconst.WalletLifecycleCycleReset, DefaultExpireDays: 0, ResetCycle: "monthly", Status: platformconst.StatusActive, Description: "Ecommerce 月套餐额度", Metadata: `{"kind":"monthly_allowance"}`, CreatedAt: now, UpdatedAt: now},
	}
	for i := range assets {
		if err := upsertAssetDefinition(db, assets[i]); err != nil {
			return err
		}
	}
	items := []models.BillableItem{
		{ID: "billable_ecommerce_image_generate", ProductID: product.ID, Code: "ecommerce.image.generate", Name: "Ecommerce Image Generation", MeterUnit: "request", BillingScope: "organization", SettlementMode: "included_then_overage", PricingBehavior: "quota_first", Status: platformconst.StatusActive, Metadata: `{"description":"Generated ecommerce image request"}`, CreatedAt: now, UpdatedAt: now},
	}
	for i := range items {
		if _, err := upsertBillableItem(db, items[i]); err != nil {
			return err
		}
	}
	skus := []struct {
		sku  models.SKU
		pkg  models.CommercialPackage
		rate models.RateCard
	}{
		{
			sku:  models.SKU{ID: "sku_ecommerce_sub_basic_monthly", ProductID: product.ID, Code: "ecommerce.sku.sub.basic.monthly", Name: "Ecommerce Basic Monthly", SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 900, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.sub.basic.monthly","payment_asset_code":"ECOMMERCE_CASH"}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_ecommerce_sub_basic_monthly", ProductID: product.ID, Code: "ecommerce.pkg.sub.basic.monthly", Name: "Basic Monthly", PackageType: "subscription", Status: platformconst.StatusActive, Metadata: `{"sku_code":"ecommerce.sku.sub.basic.monthly","monthly_quota":300}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_ecommerce_sub_basic_monthly", ProductID: product.ID, Code: "ecommerce.sku.sub.basic.monthly.v1", TargetType: "sku", TargetID: "sku_ecommerce_sub_basic_monthly", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.sub.basic.monthly"}`, CreatedAt: now, UpdatedAt: now},
		},
		{
			sku:  models.SKU{ID: "sku_ecommerce_sub_pro_monthly", ProductID: product.ID, Code: "ecommerce.sku.sub.pro.monthly", Name: "Ecommerce Pro Monthly", SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 9900, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.sub.pro.monthly"}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_ecommerce_sub_pro_monthly", ProductID: product.ID, Code: "ecommerce.pkg.sub.pro.monthly", Name: "Pro Monthly", PackageType: "subscription", Status: platformconst.StatusActive, Metadata: `{"sku_code":"ecommerce.sku.sub.pro.monthly","monthly_quota":3000}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_ecommerce_sub_pro_monthly", ProductID: product.ID, Code: "ecommerce.sku.sub.pro.monthly.v1", TargetType: "sku", TargetID: "sku_ecommerce_sub_pro_monthly", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":9900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.sub.pro.monthly"}`, CreatedAt: now, UpdatedAt: now},
		},
		{
			sku:  models.SKU{ID: "sku_ecommerce_sub_growth_monthly", ProductID: product.ID, Code: "ecommerce.sku.sub.growth.monthly", Name: "Ecommerce Growth Monthly", SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 29900, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.sub.growth.monthly"}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_ecommerce_sub_growth_monthly", ProductID: product.ID, Code: "ecommerce.pkg.sub.growth.monthly", Name: "Growth Monthly", PackageType: "subscription", Status: platformconst.StatusActive, Metadata: `{"sku_code":"ecommerce.sku.sub.growth.monthly","monthly_quota":10000}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_ecommerce_sub_growth_monthly", ProductID: product.ID, Code: "ecommerce.sku.sub.growth.monthly.v1", TargetType: "sku", TargetID: "sku_ecommerce_sub_growth_monthly", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":29900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.sub.growth.monthly"}`, CreatedAt: now, UpdatedAt: now},
		},
		{
			sku:  models.SKU{ID: "sku_ecommerce_pack_permanent_basic", ProductID: product.ID, Code: "ecommerce.sku.pack.permanent.basic", Name: "Ecommerce Permanent Basic Pack", SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 1900, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.pack.permanent.basic","quota_units":200}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_ecommerce_pack_permanent_basic", ProductID: product.ID, Code: "ecommerce.pkg.pack.permanent.basic", Name: "Permanent Basic Pack", PackageType: "permanent_pack", Status: platformconst.StatusActive, Metadata: `{"sku_code":"ecommerce.sku.pack.permanent.basic","quota_units":200}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_ecommerce_pack_permanent_basic", ProductID: product.ID, Code: "ecommerce.sku.pack.permanent.basic.v1", TargetType: "sku", TargetID: "sku_ecommerce_pack_permanent_basic", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":1900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.pack.permanent.basic"}`, CreatedAt: now, UpdatedAt: now},
		},
		{
			sku:  models.SKU{ID: "sku_ecommerce_pack_promo_basic", ProductID: product.ID, Code: "ecommerce.sku.pack.promo.basic", Name: "Ecommerce Promo Basic Pack", SKUType: "promo_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 900, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.pack.promo.basic","quota_units":100,"campaign":"launch_discount","original_price":1900}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_ecommerce_pack_promo_basic", ProductID: product.ID, Code: "ecommerce.pkg.pack.promo.basic", Name: "Promo Basic Pack", PackageType: "promo_pack", Status: platformconst.StatusActive, Metadata: `{"sku_code":"ecommerce.sku.pack.promo.basic","quota_units":100,"campaign":"launch_discount"}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_ecommerce_pack_promo_basic", ProductID: product.ID, Code: "ecommerce.sku.pack.promo.basic.v1", TargetType: "sku", TargetID: "sku_ecommerce_pack_promo_basic", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":900,"original_unit_amount":1900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"ecommerce.pkg.pack.promo.basic","campaign":"launch_discount"}`, CreatedAt: now, UpdatedAt: now},
		},
	}
	for i := range skus {
		if _, err := upsertSKU(db, skus[i].sku); err != nil {
			return err
		}
		if _, err := upsertPackage(db, skus[i].pkg); err != nil {
			return err
		}
		if _, err := upsertRateCard(db, skus[i].rate); err != nil {
			return err
		}
	}
	overageRate := models.RateCard{
		ID: "rc_ecommerce_image_generate_overage_v1", ProductID: product.ID, Code: "ecommerce.image.generate.overage.v1",
		TargetType: "billable_item", TargetID: "billable_ecommerce_image_generate", PriceModel: "flat", Currency: "ECOMMERCE_CREDIT",
		PriceConfig: `{"unit_amount":10}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"wallet_asset_code":"ECOMMERCE_CREDIT"}`, CreatedAt: now, UpdatedAt: now,
	}
	_, err = upsertRateCard(db, overageRate)
	if err != nil {
		return err
	}
	quotaPolicies := []models.QuotaGrantPolicy{
		{ID: "quota_policy_ecommerce_sub_basic_monthly", ProductCode: product.Code, PackageCode: "ecommerce.pkg.sub.basic.monthly", BillableItemCode: "ecommerce.image.generate", GrantMode: "cycle_reset", Units: 300, ResetCycle: "monthly", Status: platformconst.StatusActive, Metadata: `{"tier":"basic"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_ecommerce_sub_pro_monthly", ProductCode: product.Code, PackageCode: "ecommerce.pkg.sub.pro.monthly", BillableItemCode: "ecommerce.image.generate", GrantMode: "cycle_reset", Units: 3000, ResetCycle: "monthly", Status: platformconst.StatusActive, Metadata: `{"tier":"pro"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_ecommerce_sub_growth_monthly", ProductCode: product.Code, PackageCode: "ecommerce.pkg.sub.growth.monthly", BillableItemCode: "ecommerce.image.generate", GrantMode: "cycle_reset", Units: 10000, ResetCycle: "monthly", Status: platformconst.StatusActive, Metadata: `{"tier":"growth"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_ecommerce_pack_permanent_basic", ProductCode: product.Code, PackageCode: "ecommerce.pkg.pack.permanent.basic", BillableItemCode: "ecommerce.image.generate", GrantMode: "one_time", Units: 200, Status: platformconst.StatusActive, Metadata: `{"tier":"permanent_basic"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_ecommerce_pack_promo_basic", ProductCode: product.Code, PackageCode: "ecommerce.pkg.pack.promo.basic", BillableItemCode: "ecommerce.image.generate", GrantMode: "one_time", Units: 100, Status: platformconst.StatusActive, Metadata: `{"tier":"promo_basic"}`, CreatedAt: now, UpdatedAt: now},
	}
	for _, item := range quotaPolicies {
		if _, err := upsertQuotaGrantPolicy(db, item); err != nil {
			return err
		}
	}
	return nil
}
