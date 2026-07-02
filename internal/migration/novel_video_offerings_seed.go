package migration

import (
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

func seedNovelVideoOfferings(db *gorm.DB) error {
	now := time.Now().UTC()

	product := models.Product{
		ID:        "prod_novel_video",
		Code:      "novel_video",
		Name:      "Novel Video",
		Status:    platformconst.StatusActive,
		OwnerTeam: "v-novel-video-backend",
		Metadata:  `{"business":"novel_video","description":"AI 视频生成商业化主命名空间","primary_billable_item":"novel_video_generation"}`,
		CreatedAt: now,
		UpdatedAt: now,
	}
	productRecord, err := upsertProduct(db, product)
	if err != nil {
		return err
	}

	billableItems := []models.BillableItem{
		{ID: "billable_novel_video_generation", ProductID: productRecord.ID, Code: "novel_video_generation", Name: "Novel Video Generation", MeterUnit: "credit", BillingScope: "organization", SettlementMode: platformconst.SettlementModeIncludedThenOverage, PricingBehavior: "quota_first", Status: platformconst.StatusActive, Metadata: `{"description":"统一视频生成额度；标准 720p/5s 约 45 点"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "billable_novel_video_text_to_video", ProductID: productRecord.ID, Code: "video_text_to_video", Name: "Novel Video Text To Video", MeterUnit: "credit", BillingScope: "organization", SettlementMode: "credits", PricingBehavior: "standard", Status: platformconst.StatusActive, Metadata: `{"capability":"text_to_video","mapped_to":"novel_video_generation"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "billable_novel_video_image_to_video", ProductID: productRecord.ID, Code: "video_image_to_video", Name: "Novel Video Image To Video", MeterUnit: "credit", BillingScope: "organization", SettlementMode: "credits", PricingBehavior: "standard", Status: platformconst.StatusActive, Metadata: `{"capability":"image_to_video","mapped_to":"novel_video_generation"}`, CreatedAt: now, UpdatedAt: now},
	}
	var generationItem *models.BillableItem
	for _, item := range billableItems {
		record, err := upsertBillableItem(db, item)
		if err != nil {
			return err
		}
		if item.Code == "novel_video_generation" {
			generationItem = record
		}
	}
	if generationItem != nil {
		if _, err := upsertRateCard(db, models.RateCard{ID: "rc_novel_video_generation_v1", ProductID: productRecord.ID, Code: "novel_video.generation.v1", TargetType: "billable_item", TargetID: generationItem.ID, PriceModel: "flat", Currency: "CREDIT", PriceConfig: `{"unit_amount":1,"unit":"credit"}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"scenario":"quota_debit"}`, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
	}

	skuAndPackages := []struct {
		sku  models.SKU
		pkg  models.CommercialPackage
		rate models.RateCard
	}{
		{
			sku:  models.SKU{ID: "sku_novel_trial_signup", ProductID: productRecord.ID, Code: "novel.sku.trial.signup", Name: "Novel Video Signup Trial", SKUType: "trial", BillingMode: "one_time", Currency: "CNY", ListPrice: 0, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.trial.signup","quota_units":180,"recommended":"首次注册赠送，可完成约 4 次标准 5 秒 720p 生成"}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_novel_trial_signup", ProductID: productRecord.ID, Code: "novel.pkg.trial.signup", Name: "Novel Video Signup Trial Package", PackageType: "trial", Status: platformconst.StatusActive, Metadata: `{"signup_trial":true,"quota_units":180,"activation_reason":"signup_trial","description":"首次注册赠送约 4 次标准生成"}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_novel_trial_signup", ProductID: productRecord.ID, Code: "novel.sku.trial.signup.v1", TargetType: "sku", TargetID: "sku_novel_trial_signup", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":0}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.trial.signup"}`, CreatedAt: now, UpdatedAt: now},
		},
		{
			sku:  models.SKU{ID: "sku_novel_sub_basic_monthly", ProductID: productRecord.ID, Code: "novel.sku.sub.basic.monthly", Name: "Novel Video Basic Monthly", SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 2900, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.sub.basic.monthly","quota_units":450,"landing_description_i18n":{"zh":"适合轻量短视频创作"},"landing_features_i18n":{"zh":["每月 450 点","约 10 次标准 5 秒生成","支持文本/图片生成视频"]}}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_novel_sub_basic_monthly", ProductID: productRecord.ID, Code: "novel.pkg.sub.basic.monthly", Name: "Basic Monthly Package", PackageType: "subscription", Status: platformconst.StatusActive, Metadata: `{"sku_code":"novel.sku.sub.basic.monthly","monthly_credits":450,"landing_description_i18n":{"zh":"适合轻量短视频创作"},"landing_features_i18n":{"zh":["每月 450 点","约 10 次标准 5 秒生成","支持文本/图片生成视频"]}}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_novel_sub_basic_monthly", ProductID: productRecord.ID, Code: "novel.sku.sub.basic.monthly.v1", TargetType: "sku", TargetID: "sku_novel_sub_basic_monthly", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":2900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.sub.basic.monthly"}`, CreatedAt: now, UpdatedAt: now},
		},
		{
			sku:  models.SKU{ID: "sku_novel_sub_pro_monthly", ProductID: productRecord.ID, Code: "novel.sku.sub.pro.monthly", Name: "Novel Video Pro Monthly", SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 9900, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.sub.pro.monthly","quota_units":1800,"landing_description_i18n":{"zh":"适合稳定内容生产"},"landing_features_i18n":{"zh":["每月 1800 点","约 40 次标准 5 秒生成","优先支持多素材工作流"]}}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_novel_sub_pro_monthly", ProductID: productRecord.ID, Code: "novel.pkg.sub.pro.monthly", Name: "Pro Monthly Package", PackageType: "subscription", Status: platformconst.StatusActive, Metadata: `{"sku_code":"novel.sku.sub.pro.monthly","monthly_credits":1800,"landing_description_i18n":{"zh":"适合稳定内容生产"},"landing_features_i18n":{"zh":["每月 1800 点","约 40 次标准 5 秒生成","优先支持多素材工作流"]}}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_novel_sub_pro_monthly", ProductID: productRecord.ID, Code: "novel.sku.sub.pro.monthly.v1", TargetType: "sku", TargetID: "sku_novel_sub_pro_monthly", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":9900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.sub.pro.monthly"}`, CreatedAt: now, UpdatedAt: now},
		},
		{
			sku:  models.SKU{ID: "sku_novel_pack_credits_basic", ProductID: productRecord.ID, Code: "novel.sku.pack.credits.basic", Name: "Novel Video 450 Credits Pack", SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 3900, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.pack.credits.basic","quota_units":450}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_novel_pack_credits_basic", ProductID: productRecord.ID, Code: "novel.pkg.pack.credits.basic", Name: "450 Credits Pack", PackageType: "permanent_pack", Status: platformconst.StatusActive, Metadata: `{"sku_code":"novel.sku.pack.credits.basic","quota_units":450}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_novel_pack_credits_basic", ProductID: productRecord.ID, Code: "novel.sku.pack.credits.basic.v1", TargetType: "sku", TargetID: "sku_novel_pack_credits_basic", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":3900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.pack.credits.basic"}`, CreatedAt: now, UpdatedAt: now},
		},
		{
			sku:  models.SKU{ID: "sku_novel_pack_credits_pro", ProductID: productRecord.ID, Code: "novel.sku.pack.credits.pro", Name: "Novel Video 1800 Credits Pack", SKUType: "resource_pack", BillingMode: "one_time", Currency: "CNY", ListPrice: 12900, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.pack.credits.pro","quota_units":1800}`, CreatedAt: now, UpdatedAt: now},
			pkg:  models.CommercialPackage{ID: "pkg_novel_pack_credits_pro", ProductID: productRecord.ID, Code: "novel.pkg.pack.credits.pro", Name: "1800 Credits Pack", PackageType: "permanent_pack", Status: platformconst.StatusActive, Metadata: `{"sku_code":"novel.sku.pack.credits.pro","quota_units":1800}`, CreatedAt: now, UpdatedAt: now},
			rate: models.RateCard{ID: "rc_novel_pack_credits_pro", ProductID: productRecord.ID, Code: "novel.sku.pack.credits.pro.v1", TargetType: "sku", TargetID: "sku_novel_pack_credits_pro", PriceModel: "flat", Currency: "CNY", PriceConfig: `{"unit_amount":12900}`, Version: 1, Status: platformconst.StatusActive, Metadata: `{"package_code":"novel.pkg.pack.credits.pro"}`, CreatedAt: now, UpdatedAt: now},
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

	quotaPolicies := []models.QuotaGrantPolicy{
		{ID: "quota_policy_novel_trial_signup", ProductCode: productRecord.Code, PackageCode: "novel.pkg.trial.signup", BillableItemCode: "novel_video_generation", GrantMode: "one_time", Units: 180, Status: platformconst.StatusActive, Metadata: `{"tier":"trial","signup_trial":true}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_novel_sub_basic_monthly", ProductCode: productRecord.Code, PackageCode: "novel.pkg.sub.basic.monthly", BillableItemCode: "novel_video_generation", GrantMode: "cycle_reset", Units: 450, ResetCycle: "monthly", Status: platformconst.StatusActive, Metadata: `{"tier":"basic"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_novel_sub_pro_monthly", ProductCode: productRecord.Code, PackageCode: "novel.pkg.sub.pro.monthly", BillableItemCode: "novel_video_generation", GrantMode: "cycle_reset", Units: 1800, ResetCycle: "monthly", Status: platformconst.StatusActive, Metadata: `{"tier":"pro"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_novel_pack_credits_basic", ProductCode: productRecord.Code, PackageCode: "novel.pkg.pack.credits.basic", BillableItemCode: "novel_video_generation", GrantMode: "one_time", Units: 450, Status: platformconst.StatusActive, Metadata: `{"tier":"pack_basic"}`, CreatedAt: now, UpdatedAt: now},
		{ID: "quota_policy_novel_pack_credits_pro", ProductCode: productRecord.Code, PackageCode: "novel.pkg.pack.credits.pro", BillableItemCode: "novel_video_generation", GrantMode: "one_time", Units: 1800, Status: platformconst.StatusActive, Metadata: `{"tier":"pack_pro"}`, CreatedAt: now, UpdatedAt: now},
	}
	for _, item := range quotaPolicies {
		if _, err := upsertQuotaGrantPolicy(db, item); err != nil {
			return err
		}
	}
	return nil
}
