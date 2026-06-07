package migration

import (
	"errors"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

func seedMenuOfferings(db *gorm.DB) error {
	now := time.Now().UTC()

	product := models.Product{
		ID:        "prod_menu",
		Code:      "menu",
		Name:      "Menu",
		Status:    platformconst.StatusActive,
		OwnerTeam: "v-menu-backend",
		Metadata:  `{"business":"menu","workspace_label":"当前工作区 product","description":"Menu 商业化主命名空间"}`,
		CreatedAt: now,
		UpdatedAt: now,
	}
	productRecord, err := upsertProduct(db, product)
	if err != nil {
		return err
	}

	billableItem := models.BillableItem{
		ID:              "billable_menu_render_call",
		ProductID:       productRecord.ID,
		Code:            "menu.render.call",
		Name:            "Menu Render Call",
		MeterUnit:       "call",
		BillingScope:    "organization",
		SettlementMode:  platformconst.SettlementModeIncludedThenOverage,
		PricingBehavior: "quota_first",
		Status:          platformconst.StatusActive,
		Metadata:        `{"description":"默认按次消费","default_unit_price_credits":10}`,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	billableRecord, err := upsertBillableItem(db, billableItem)
	if err != nil {
		return err
	}
	if _, err := upsertRateCard(db, models.RateCard{
		ID:          "rc_menu_render_call_v1",
		ProductID:   productRecord.ID,
		Code:        "menu.render.call.v1",
		TargetType:  "billable_item",
		TargetID:    billableRecord.ID,
		PriceModel:  "flat",
		Currency:    "MENU_CREDIT",
		PriceConfig: `{"unit_amount":10}`,
		Version:     1,
		Status:      platformconst.StatusActive,
		Metadata:    `{"scenario":"default_overage"}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		return err
	}

	assets := []models.AssetDefinition{
		{
			AssetCode:         "MENU_CASH",
			ProductCode:       productRecord.Code,
			AssetType:         "cash_balance",
			LifecycleType:     platformconst.WalletLifecyclePermanent,
			DefaultExpireDays: 0,
			Status:            platformconst.StatusActive,
			Description:       "Menu 现金支付余额",
			Metadata:          `{"kind":"payment_cash_balance","currency":"CNY"}`,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			AssetCode:         "MENU_CREDIT",
			ProductCode:       productRecord.Code,
			AssetType:         platformconst.WalletAssetTypeCredit,
			LifecycleType:     platformconst.WalletLifecyclePermanent,
			DefaultExpireDays: 0,
			Status:            platformconst.StatusActive,
			Description:       "Menu 永久可消费余额",
			Metadata:          `{"kind":"permanent_pack_balance"}`,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			AssetCode:         "MENU_PROMO_CREDIT",
			ProductCode:       productRecord.Code,
			AssetType:         platformconst.WalletAssetTypeRewardCredit,
			LifecycleType:     platformconst.WalletLifecycleExpiring,
			DefaultExpireDays: 90,
			Status:            platformconst.StatusActive,
			Description:       "Menu 限期资源包或活动赠送余额",
			Metadata:          `{"kind":"expiring_pack_balance","expiry_source":"package_override_or_default"}`,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			AssetCode:     "MENU_MONTHLY_ALLOWANCE",
			ProductCode:   productRecord.Code,
			AssetType:     platformconst.WalletAssetTypeSubscriptionAllow,
			LifecycleType: platformconst.WalletLifecycleCycleReset,
			ResetCycle:    "monthly",
			Status:        platformconst.StatusActive,
			Description:   "Menu 订阅月额度",
			Metadata:      `{"kind":"subscription_allowance"}`,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}
	for _, asset := range assets {
		if err := upsertAssetDefinition(db, asset); err != nil {
			return err
		}
	}

	skuAndPackages := []struct {
		sku  models.SKU
		pkg  models.CommercialPackage
		rate models.RateCard
	}{
		{
			sku: models.SKU{
				ID: "sku_menu_sub_basic_monthly", ProductID: productRecord.ID, Code: "menu.sku.sub.basic.monthly", Name: "Menu Basic Monthly",
				SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.sub.basic.monthly","tier":"basic","landing_description_i18n":{"zh":"立即开始使用","th":"เริ่มต้นใช้งานได้ทันที","en":"Get started instantly"},"landing_features_i18n":{"zh":["AI 文案生成","AI 图片生成","图片增强优化","菜单排版生成"],"th":["AI เขียนคำโฆษณา","สร้างภาพ AI","เพิ่มคุณภาพภาพ","จัดเลย์เอาต์เมนู"],"en":["AI Copywriting","AI Image Generation","Image Enhancement","Menu Layout Design"]}}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_sub_basic_monthly", ProductID: productRecord.ID, Code: "menu.pkg.sub.basic.monthly", Name: "Basic Monthly Package",
				PackageType: "subscription", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.sub.basic.monthly","monthly_calls":300,"landing_description_i18n":{"zh":"立即开始使用","th":"เริ่มต้นใช้งานได้ทันที","en":"Get started instantly"},"landing_features_i18n":{"zh":["AI 文案生成","AI 图片生成","图片增强优化","菜单排版生成"],"th":["AI เขียนคำโฆษณา","สร้างภาพ AI","เพิ่มคุณภาพภาพ","จัดเลย์เอาต์เมนู"],"en":["AI Copywriting","AI Image Generation","Image Enhancement","Menu Layout Design"]}}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_sub_basic_monthly", ProductID: productRecord.ID, Code: "menu.sku.sub.basic.monthly.v1", TargetType: "sku", TargetID: "sku_menu_sub_basic_monthly",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.sub.basic.monthly"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_sub_pro_monthly", ProductID: productRecord.ID, Code: "menu.sku.sub.pro.monthly", Name: "Menu Pro Monthly",
				SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 24900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.sub.pro.monthly","tier":"pro","landing_description_i18n":{"zh":"适合成长中的店铺","th":"สำหรับร้านค้าที่เติบโต","en":"For growing businesses"},"landing_features_i18n":{"zh":["社媒模板导出","评价海报生成","多图组合拼接","节日主题模板","外卖适配模板"],"th":["ส่งออกโซเชียลมีเดีย","โปสเตอร์รีวิว","คอลลาจหลายภาพ","แม่แบบเทศกาล","แม่แบบเดลิเวอรี"],"en":["Social Media Export","Review Poster","Multi-Image Collage","Festival Templates","Delivery Templates"]}}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_sub_pro_monthly", ProductID: productRecord.ID, Code: "menu.pkg.sub.pro.monthly", Name: "Pro Monthly Package",
				PackageType: "subscription", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.sub.pro.monthly","monthly_calls":3000,"landing_description_i18n":{"zh":"适合成长中的店铺","th":"สำหรับร้านค้าที่เติบโต","en":"For growing businesses"},"landing_features_i18n":{"zh":["社媒模板导出","评价海报生成","多图组合拼接","节日主题模板","外卖适配模板"],"th":["ส่งออกโซเชียลมีเดีย","โปสเตอร์รีวิว","คอลลาจหลายภาพ","แม่แบบเทศกาล","แม่แบบเดลิเวอรี"],"en":["Social Media Export","Review Poster","Multi-Image Collage","Festival Templates","Delivery Templates"]}}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_sub_pro_monthly", ProductID: productRecord.ID, Code: "menu.sku.sub.pro.monthly.v1", TargetType: "sku", TargetID: "sku_menu_sub_pro_monthly",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":24900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.sub.pro.monthly"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_sub_growth_monthly", ProductID: productRecord.ID, Code: "menu.sku.sub.growth.monthly", Name: "Menu Growth Monthly",
				SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 49900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.sub.growth.monthly","tier":"growth","landing_description_i18n":{"zh":"适合追求增长的企业","th":"สำหรับธุรกิจที่ต้องการเติบโต","en":"For businesses scaling up"},"landing_features_i18n":{"zh":["泰国节日主题模板","外卖定向适配模板","4K 导出","优先客服支持","推荐好友可达 10 人"],"th":["แม่แบบเทศกาลไทย","แม่แบบเดลิเวอรีเฉพาะทาง","ส่งออก 4K","สนับสนุนลำดับความสำคัญ","แนะนำเพื่อนได้ 10 คน"],"en":["Thai Festival Templates","Delivery-Optimized Templates","4K Export","Priority Support","Up to 10 referrals"]}}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_sub_growth_monthly", ProductID: productRecord.ID, Code: "menu.pkg.sub.growth.monthly", Name: "Growth Monthly Package",
				PackageType: "subscription", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.sub.growth.monthly","monthly_calls":8000,"landing_description_i18n":{"zh":"适合追求增长的企业","th":"สำหรับธุรกิจที่ต้องการเติบโต","en":"For businesses scaling up"},"landing_features_i18n":{"zh":["泰国节日主题模板","外卖定向适配模板","4K 导出","优先客服支持","推荐好友可达 10 人"],"th":["แม่แบบเทศกาลไทย","แม่แบบเดลิเวอรีเฉพาะทาง","ส่งออก 4K","สนับสนุนลำดับความสำคัญ","แนะนำเพื่อนได้ 10 คน"],"en":["Thai Festival Templates","Delivery-Optimized Templates","4K Export","Priority Support","Up to 10 referrals"]}}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_sub_growth_monthly", ProductID: productRecord.ID, Code: "menu.sku.sub.growth.monthly.v1", TargetType: "sku", TargetID: "sku_menu_sub_growth_monthly",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":49900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.sub.growth.monthly"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_permanent_basic", ProductID: productRecord.ID, Code: "menu.sku.pack.permanent.basic", Name: "Menu Permanent Basic Pack",
				SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 1900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.permanent.basic","quota_units":100}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_permanent_basic", ProductID: productRecord.ID, Code: "menu.pkg.pack.permanent.basic", Name: "Permanent Basic Pack",
				PackageType: "permanent_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.permanent.basic","quota_units":100}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_permanent_basic", ProductID: productRecord.ID, Code: "menu.sku.pack.permanent.basic.v1", TargetType: "sku", TargetID: "sku_menu_pack_permanent_basic",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":1900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.permanent.basic"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_permanent_pro", ProductID: productRecord.ID, Code: "menu.sku.pack.permanent.pro", Name: "Menu Permanent Pro Pack",
				SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 19900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.permanent.pro","quota_units":1000}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_permanent_pro", ProductID: productRecord.ID, Code: "menu.pkg.pack.permanent.pro", Name: "Permanent Pro Pack",
				PackageType: "permanent_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.permanent.pro","quota_units":1000}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_permanent_pro", ProductID: productRecord.ID, Code: "menu.sku.pack.permanent.pro.v1", TargetType: "sku", TargetID: "sku_menu_pack_permanent_pro",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":19900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.permanent.pro"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_permanent_bulk", ProductID: productRecord.ID, Code: "menu.sku.pack.permanent.bulk", Name: "Menu Permanent Bulk Pack",
				SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 69900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.permanent.bulk","quota_units":5000}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_permanent_bulk", ProductID: productRecord.ID, Code: "menu.pkg.pack.permanent.bulk", Name: "Permanent Bulk Pack",
				PackageType: "permanent_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.permanent.bulk","quota_units":5000}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_permanent_bulk", ProductID: productRecord.ID, Code: "menu.sku.pack.permanent.bulk.v1", TargetType: "sku", TargetID: "sku_menu_pack_permanent_bulk",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":69900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.permanent.bulk"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_expiring_basic", ProductID: productRecord.ID, Code: "menu.sku.pack.expiring.basic", Name: "Menu Expiring Basic Pack",
				SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 1900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.expiring.basic","quota_units":100,"expire_months":1}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_expiring_basic", ProductID: productRecord.ID, Code: "menu.pkg.pack.expiring.basic", Name: "Expiring Basic Pack",
				PackageType: "expiring_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.expiring.basic","quota_units":100,"expire_months":1}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_expiring_basic", ProductID: productRecord.ID, Code: "menu.sku.pack.expiring.basic.v1", TargetType: "sku", TargetID: "sku_menu_pack_expiring_basic",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":1900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.expiring.basic"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_expiring_pro", ProductID: productRecord.ID, Code: "menu.sku.pack.expiring.pro", Name: "Menu Expiring Pro Pack",
				SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 19900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.expiring.pro","quota_units":1000,"expire_months":1}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_expiring_pro", ProductID: productRecord.ID, Code: "menu.pkg.pack.expiring.pro", Name: "Expiring Pro Pack",
				PackageType: "expiring_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.expiring.pro","quota_units":1000,"expire_months":1}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_expiring_pro", ProductID: productRecord.ID, Code: "menu.sku.pack.expiring.pro.v1", TargetType: "sku", TargetID: "sku_menu_pack_expiring_pro",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":19900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.expiring.pro"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_expiring_bulk", ProductID: productRecord.ID, Code: "menu.sku.pack.expiring.bulk", Name: "Menu Expiring Bulk Pack",
				SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 69900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.expiring.bulk","quota_units":5000,"expire_months":1}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_expiring_bulk", ProductID: productRecord.ID, Code: "menu.pkg.pack.expiring.bulk", Name: "Expiring Bulk Pack",
				PackageType: "expiring_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.expiring.bulk","quota_units":5000,"expire_months":1}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_expiring_bulk", ProductID: productRecord.ID, Code: "menu.sku.pack.expiring.bulk.v1", TargetType: "sku", TargetID: "sku_menu_pack_expiring_bulk",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":69900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.expiring.bulk"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_promo_basic", ProductID: productRecord.ID, Code: "menu.sku.pack.promo.basic", Name: "Menu Promo Basic Pack",
				SKUType: "promo_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.promo.basic","quota_units":100,"campaign":"limited_discount","original_price":1900}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_promo_basic", ProductID: productRecord.ID, Code: "menu.pkg.pack.promo.basic", Name: "Promo Basic Pack",
				PackageType: "promo_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.promo.basic","quota_units":100,"campaign":"limited_discount"}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_promo_basic", ProductID: productRecord.ID, Code: "menu.sku.pack.promo.basic.v1", TargetType: "sku", TargetID: "sku_menu_pack_promo_basic",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":900,"original_unit_amount":1900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"campaign":"menu_limited_discount","package_code":"menu.pkg.pack.promo.basic"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_promo_pro", ProductID: productRecord.ID, Code: "menu.sku.pack.promo.pro", Name: "Menu Promo Pro Pack",
				SKUType: "promo_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 9900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.promo.pro","quota_units":1000,"campaign":"limited_discount","original_price":19900}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_promo_pro", ProductID: productRecord.ID, Code: "menu.pkg.pack.promo.pro", Name: "Promo Pro Pack",
				PackageType: "promo_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.promo.pro","quota_units":1000,"campaign":"limited_discount"}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_promo_pro", ProductID: productRecord.ID, Code: "menu.sku.pack.promo.pro.v1", TargetType: "sku", TargetID: "sku_menu_pack_promo_pro",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":9900,"original_unit_amount":19900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"campaign":"menu_limited_discount","package_code":"menu.pkg.pack.promo.pro"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			sku: models.SKU{
				ID: "sku_menu_pack_promo_bulk", ProductID: productRecord.ID, Code: "menu.sku.pack.promo.bulk", Name: "Menu Promo Bulk Pack",
				SKUType: "promo_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 39900, Status: platformconst.StatusActive,
				Metadata: `{"package_code":"menu.pkg.pack.promo.bulk","quota_units":5000,"campaign":"limited_discount","original_price":69900}`, CreatedAt: now, UpdatedAt: now,
			},
			pkg: models.CommercialPackage{
				ID: "pkg_menu_pack_promo_bulk", ProductID: productRecord.ID, Code: "menu.pkg.pack.promo.bulk", Name: "Promo Bulk Pack",
				PackageType: "promo_pack", Status: platformconst.StatusActive,
				Metadata:  `{"sku_code":"menu.sku.pack.promo.bulk","quota_units":5000,"campaign":"limited_discount"}`,
				CreatedAt: now, UpdatedAt: now,
			},
			rate: models.RateCard{
				ID: "rc_menu_pack_promo_bulk", ProductID: productRecord.ID, Code: "menu.sku.pack.promo.bulk.v1", TargetType: "sku", TargetID: "sku_menu_pack_promo_bulk",
				PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":39900,"original_unit_amount":69900}`, Version: 1, Status: platformconst.StatusActive,
				Metadata: `{"campaign":"menu_limited_discount","package_code":"menu.pkg.pack.promo.bulk"}`, CreatedAt: now, UpdatedAt: now,
			},
		},
	}

	for _, item := range skuAndPackages {
		skuRecord, err := upsertSKU(db, item.sku)
		if err != nil {
			return err
		}
		if _, err := upsertPackage(db, item.pkg); err != nil {
			return err
		}
		item.rate.TargetID = skuRecord.ID
		if _, err := upsertRateCard(db, item.rate); err != nil {
			return err
		}
	}
	if _, err := upsertPackage(db, models.CommercialPackage{
		ID:          "pkg_menu_trial_signup",
		ProductID:   productRecord.ID,
		Code:        "menu.pkg.trial.signup",
		Name:        "Menu Signup Trial Package",
		PackageType: "trial",
		Status:      platformconst.StatusActive,
		Metadata:    `{"signup_trial":true,"quota_units":5,"activation_reason":"signup_trial"}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		return err
	}
	quotaPolicies := []models.QuotaGrantPolicy{
		{ID: "quota_policy_menu_trial_signup", ProductCode: productRecord.Code, PackageCode: "menu.pkg.trial.signup", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 5, Status: platformconst.StatusActive, Metadata: `{"tier":"trial","signup_trial":true}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_sub_basic_monthly", ProductCode: productRecord.Code, PackageCode: "menu.pkg.sub.basic.monthly", BillableItemCode: "menu.render.call", GrantMode: "cycle_reset", Units: 300, ResetCycle: "monthly", Status: platformconst.StatusActive, Metadata: `{"tier":"basic"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_sub_pro_monthly", ProductCode: productRecord.Code, PackageCode: "menu.pkg.sub.pro.monthly", BillableItemCode: "menu.render.call", GrantMode: "cycle_reset", Units: 3000, ResetCycle: "monthly", Status: platformconst.StatusActive, Metadata: `{"tier":"pro"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_sub_growth_monthly", ProductCode: productRecord.Code, PackageCode: "menu.pkg.sub.growth.monthly", BillableItemCode: "menu.render.call", GrantMode: "cycle_reset", Units: 8000, ResetCycle: "monthly", Status: platformconst.StatusActive, Metadata: `{"tier":"growth"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_permanent_basic", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.permanent.basic", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 100, Status: platformconst.StatusActive, Metadata: `{"tier":"pack_basic"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_permanent_pro", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.permanent.pro", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 1000, Status: platformconst.StatusActive, Metadata: `{"tier":"pack_pro"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_permanent_bulk", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.permanent.bulk", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 5000, Status: platformconst.StatusActive, Metadata: `{"tier":"pack_bulk"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_expiring_basic", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.expiring.basic", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 100, Status: platformconst.StatusActive, Metadata: `{"tier":"expiring_basic","expire_months":1}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_expiring_pro", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.expiring.pro", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 1000, Status: platformconst.StatusActive, Metadata: `{"tier":"expiring_pro","expire_months":1}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_expiring_bulk", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.expiring.bulk", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 5000, Status: platformconst.StatusActive, Metadata: `{"tier":"expiring_bulk","expire_months":1}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_promo_basic", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.promo.basic", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 100, Status: platformconst.StatusActive, Metadata: `{"tier":"promo_basic"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_promo_pro", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.promo.pro", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 1000, Status: platformconst.StatusActive, Metadata: `{"tier":"promo_pro"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_menu_pack_promo_bulk", ProductCode: productRecord.Code, PackageCode: "menu.pkg.pack.promo.bulk", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 5000, Status: platformconst.StatusActive, Metadata: `{"tier":"promo_bulk"}`, CreatedAt: now, UpdatedAt: now},
	}
	for _, item := range quotaPolicies {
		if _, err := upsertQuotaGrantPolicy(db, item); err != nil {
			return err
		}
	}
	capabilityPolicies := []models.PackageCapabilityPolicy{
		{ID: "cap_policy_menu_trial_signup_template_scope", ProductCode: productRecord.Code, PackageCode: "menu.pkg.trial.signup", CapabilityCode: "template_scope", GrantValue: "free_templates", Status: platformconst.StatusActive, Metadata: `{"tier":"trial","signup_trial":true}`, CreatedAt: now, UpdatedAt: now},
		{ID: "cap_policy_menu_sub_basic_template_scope", ProductCode: productRecord.Code, PackageCode: "menu.pkg.sub.basic.monthly", CapabilityCode: "template_scope", GrantValue: "free_templates", Status: platformconst.StatusActive, Metadata: `{"tier":"basic"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "cap_policy_menu_sub_pro_template_scope", ProductCode: productRecord.Code, PackageCode: "menu.pkg.sub.pro.monthly", CapabilityCode: "template_scope", GrantValue: "official_templates", Status: platformconst.StatusActive, Metadata: `{"tier":"pro"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "cap_policy_menu_sub_growth_template_scope", ProductCode: productRecord.Code, PackageCode: "menu.pkg.sub.growth.monthly", CapabilityCode: "template_scope", GrantValue: "all_templates", Status: platformconst.StatusActive, Metadata: `{"tier":"growth"}`, CreatedAt: now, UpdatedAt: now},
	}
	for _, item := range capabilityPolicies {
		if _, err := upsertPackageCapabilityPolicy(db, item); err != nil {
			return err
		}
	}
	return nil
}

func upsertProduct(db *gorm.DB, item models.Product) (*models.Product, error) {
	var existing models.Product
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &item, db.Create(&item).Error
		}
		return nil, err
	}
	existing.Name = item.Name
	existing.Status = item.Status
	existing.OwnerTeam = item.OwnerTeam
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return &existing, db.Save(&existing).Error
}

func upsertSKU(db *gorm.DB, item models.SKU) (*models.SKU, error) {
	var existing models.SKU
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &item, db.Create(&item).Error
		}
		return nil, err
	}
	existing.ProductID = item.ProductID
	existing.Name = item.Name
	existing.SKUType = item.SKUType
	existing.BillingMode = item.BillingMode
	existing.Currency = item.Currency
	existing.ListPrice = item.ListPrice
	existing.Status = item.Status
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return &existing, db.Save(&existing).Error
}

func upsertPackage(db *gorm.DB, item models.CommercialPackage) (*models.CommercialPackage, error) {
	var existing models.CommercialPackage
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &item, db.Create(&item).Error
		}
		return nil, err
	}
	existing.ProductID = item.ProductID
	existing.Name = item.Name
	existing.PackageType = item.PackageType
	existing.Status = item.Status
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return &existing, db.Save(&existing).Error
}

func upsertBillableItem(db *gorm.DB, item models.BillableItem) (*models.BillableItem, error) {
	var existing models.BillableItem
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &item, db.Create(&item).Error
		}
		return nil, err
	}
	existing.ProductID = item.ProductID
	existing.Name = item.Name
	existing.MeterUnit = item.MeterUnit
	existing.BillingScope = item.BillingScope
	existing.SettlementMode = item.SettlementMode
	existing.PricingBehavior = item.PricingBehavior
	existing.Status = item.Status
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return &existing, db.Save(&existing).Error
}

func upsertRateCard(db *gorm.DB, item models.RateCard) (*models.RateCard, error) {
	var existing models.RateCard
	if err := db.Where("code = ?", item.Code).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &item, db.Create(&item).Error
		}
		return nil, err
	}
	existing.ProductID = item.ProductID
	existing.TargetType = item.TargetType
	existing.TargetID = item.TargetID
	existing.PriceModel = item.PriceModel
	existing.Currency = item.Currency
	existing.PriceConfig = item.PriceConfig
	existing.EffectiveFrom = item.EffectiveFrom
	existing.EffectiveTo = item.EffectiveTo
	existing.Version = item.Version
	existing.Status = item.Status
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return &existing, db.Save(&existing).Error
}

func upsertAssetDefinition(db *gorm.DB, item models.AssetDefinition) error {
	var existing models.AssetDefinition
	if err := db.Where("asset_code = ?", item.AssetCode).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Create(&item).Error
		}
		return err
	}
	existing.ProductCode = item.ProductCode
	existing.AssetType = item.AssetType
	existing.LifecycleType = item.LifecycleType
	existing.DefaultExpireDays = item.DefaultExpireDays
	existing.ResetCycle = item.ResetCycle
	existing.Status = item.Status
	existing.Description = item.Description
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return db.Save(&existing).Error
}

func upsertAllowancePolicy(db *gorm.DB, item models.AllowancePolicy) error {
	var existing models.AllowancePolicy
	if err := db.Where("id = ?", item.ID).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Create(&item).Error
		}
		return err
	}
	existing.ProductCode = item.ProductCode
	existing.BillingSubjectType = item.BillingSubjectType
	existing.BillingSubjectID = item.BillingSubjectID
	existing.AssetCode = item.AssetCode
	existing.Amount = item.Amount
	existing.ResetCycle = item.ResetCycle
	existing.Status = item.Status
	existing.EffectiveFrom = item.EffectiveFrom
	existing.EffectiveTo = item.EffectiveTo
	existing.Metadata = item.Metadata
	existing.UpdatedAt = item.UpdatedAt
	return db.Save(&existing).Error
}
