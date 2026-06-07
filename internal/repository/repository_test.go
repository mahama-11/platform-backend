package repository

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("repository-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.OrganizationMember{},
		&models.RolePermission{},
		&models.Product{},
		&models.SKU{},
		&models.BillableItem{},
		&models.CommercialEntity{},
		&models.BillingProfile{},
		&models.CommercialPackage{},
		&models.RateCard{},
		&models.OrgBillingProfile{},
		&models.QuotaLedger{},
		&models.CreditsLedger{},
		&models.ResourceReservation{},
		&models.QuotaGrantPolicy{},
		&models.PackageCapabilityPolicy{},
		&models.CapabilityGrant{},
		&models.AuditLog{},
		&models.RuntimeProviderDefinition{},
		&models.RuntimeProductEndpoint{},
		&models.RuntimeProviderBinding{},
		&models.StorageBinding{},
		&models.StorageAsset{},
		&models.RuntimeJob{},
		&models.RuntimeAttempt{},
		&models.ChargeSession{},
		&models.RuntimeCallbackDelivery{},
		&models.AssetDefinition{},
		&models.AllowancePolicy{},
		&models.SettlementRecord{},
		&models.WalletAccount{},
		&models.WalletBucket{},
		&models.WalletLedger{},
		&models.DiscountLedger{},
		&models.RewardLedger{},
		&models.CommissionLedger{},
		&models.ReferralProgram{},
		&models.ReferralCode{},
		&models.ReferralConversion{},
		&models.ChannelPartner{},
		&models.ChannelProgram{},
		&models.ChannelPartnerBinding{},
		&models.ChannelCommissionPolicy{},
		&models.ChannelCommissionPolicyVersion{},
		&models.ChannelCommissionPolicyAssignment{},
		&models.ChannelProfitSnapshot{},
		&models.ChannelPolicyResolutionAudit{},
		&models.ChannelCommissionLedger{},
		&models.ChannelClawbackLedger{},
		&models.ChannelCommissionAdjustmentLedger{},
		&models.ChannelSettlementBatch{},
		&models.ChannelSettlementItem{},
		&models.ChannelSettlementItemLedger{},
		&models.ChannelSettlementItemClawback{},
		&models.ChannelSettlementItemAdjustment{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestCoreCommercialAndControlRepositories(t *testing.T) {
	db := newRepositoryTestDB(t)
	core := NewCoreRepository(db)
	commercial := NewCommercialRepository(db)
	control := NewControlRepository(db)

	user := &models.User{ID: "user-1", Email: "user@example.com", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	org := &models.Organization{ID: "org-1", Name: "Org 1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member := &models.OrganizationMember{ID: "m1", OrganizationID: org.ID, UserID: user.ID, Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	rolePermission := &models.RolePermission{RoleID: "owner", PermissionID: "platform.admin"}
	_ = db.Create(user).Error
	_ = db.Create(org).Error
	_ = db.Create(member).Error
	_ = db.Create(rolePermission).Error
	if _, err := core.FindUserByEmail(user.Email); err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	if _, err := core.FindActiveUserByID(user.ID); err != nil {
		t.Fatalf("FindActiveUserByID: %v", err)
	}
	if _, err := core.FindOrganizationByID(org.ID); err != nil {
		t.Fatalf("FindOrganizationByID: %v", err)
	}
	if _, err := core.FindMembership(org.ID, user.ID); err != nil {
		t.Fatalf("FindMembership: %v", err)
	}
	if memberships, err := core.ListMemberships(user.ID); err != nil || len(memberships) != 1 {
		t.Fatalf("ListMemberships: %+v err=%v", memberships, err)
	}
	if orgs, err := core.ListOrganizationsByIDs([]string{org.ID}); err != nil || len(orgs) != 1 {
		t.Fatalf("ListOrganizationsByIDs: %+v err=%v", orgs, err)
	}
	if ids, err := core.ListRolePermissionIDs("owner"); err != nil || len(ids) != 1 {
		t.Fatalf("ListRolePermissionIDs: %+v err=%v", ids, err)
	}

	product := &models.Product{ID: "prod-1", Code: "ecommerce", Name: "Ecommerce", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := commercial.CreateProduct(product); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if products, err := commercial.ListProducts(); err != nil || len(products) != 1 {
		t.Fatalf("ListProducts: %+v err=%v", products, err)
	}
	sku := &models.SKU{ID: "sku-1", ProductID: product.ID, Code: "sku-1", Name: "SKU 1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := commercial.CreateSKU(sku); err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}
	if _, err := commercial.FindSKUByID(sku.ID); err != nil {
		t.Fatalf("FindSKUByID: %v", err)
	}
	sku.Name = "SKU Updated"
	if err := commercial.SaveSKU(sku); err != nil {
		t.Fatalf("SaveSKU: %v", err)
	}
	updatedSKU, err := commercial.FindSKUByID(sku.ID)
	if err != nil || updatedSKU.Name != "SKU Updated" {
		t.Fatalf("updated SKU mismatch: %+v err=%v", updatedSKU, err)
	}
	if items, err := commercial.ListSKUs(product.ID); err != nil || len(items) != 1 {
		t.Fatalf("ListSKUs: %+v err=%v", items, err)
	}
	item := &models.BillableItem{ID: "item-1", ProductID: product.ID, Code: "IMAGE_GENERATION", Name: "Image Generation", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := commercial.CreateBillableItem(item); err != nil {
		t.Fatalf("CreateBillableItem: %v", err)
	}
	if gotItem, err := commercial.FindBillableItemByID(item.ID); err != nil || gotItem.Code != item.Code {
		t.Fatalf("FindBillableItemByID: %+v err=%v", gotItem, err)
	}
	if _, err := commercial.FindBillableItemByCode(item.Code); err != nil {
		t.Fatalf("FindBillableItemByCode: %v", err)
	}
	item.Name = "Image Generation Updated"
	if err := commercial.SaveBillableItem(item); err != nil {
		t.Fatalf("SaveBillableItem: %v", err)
	}
	if items, err := commercial.ListBillableItems(product.ID); err != nil || len(items) != 1 || items[0].Name != "Image Generation Updated" {
		t.Fatalf("ListBillableItems: %+v err=%v", items, err)
	}
	entity := &models.CommercialEntity{ID: "ce-1", Code: "ecom-cn", Name: "Ecom CN", EntityType: "product_operator", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := commercial.CreateCommercialEntity(entity); err != nil {
		t.Fatalf("CreateCommercialEntity: %v", err)
	}
	if gotEntity, err := commercial.FindCommercialEntityByID(entity.ID); err != nil || gotEntity.Code != entity.Code {
		t.Fatalf("FindCommercialEntityByID: %+v err=%v", gotEntity, err)
	}
	entity.Name = "Ecom CN Updated"
	if err := commercial.SaveCommercialEntity(entity); err != nil {
		t.Fatalf("SaveCommercialEntity: %v", err)
	}
	if entities, err := commercial.ListCommercialEntities(); err != nil || len(entities) != 1 {
		t.Fatalf("ListCommercialEntities: %+v err=%v", entities, err)
	}
	profile := &models.BillingProfile{ID: "bp-1", Code: "bp-default", ProductID: product.ID, CommercialEntityID: entity.ID, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := commercial.CreateBillingProfile(profile); err != nil {
		t.Fatalf("CreateBillingProfile: %v", err)
	}
	if gotProfile, err := commercial.FindBillingProfileByID(profile.ID); err != nil || gotProfile.Code != profile.Code {
		t.Fatalf("FindBillingProfileByID: %+v err=%v", gotProfile, err)
	}
	if _, err := commercial.FindBillingProfileByCode(profile.Code); err != nil {
		t.Fatalf("FindBillingProfileByCode: %v", err)
	}
	profile.Status = "inactive"
	if err := commercial.SaveBillingProfile(profile); err != nil {
		t.Fatalf("SaveBillingProfile: %v", err)
	}
	if profiles, err := commercial.ListBillingProfiles(product.ID); err != nil || len(profiles) != 1 || profiles[0].Status != "inactive" {
		t.Fatalf("ListBillingProfiles: %+v err=%v", profiles, err)
	}
	pkg := &models.CommercialPackage{ID: "pkg-1", ProductID: product.ID, Code: "pkg-1", Name: "Package", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := commercial.CreatePackage(pkg); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	if gotPackage, err := commercial.FindPackageByID(pkg.ID); err != nil || gotPackage.Code != pkg.Code {
		t.Fatalf("FindPackageByID: %+v err=%v", gotPackage, err)
	}
	pkg.Name = "Package Updated"
	if err := commercial.SavePackage(pkg); err != nil {
		t.Fatalf("SavePackage: %v", err)
	}
	if packages, err := commercial.ListPackages(product.ID); err != nil || len(packages) != 1 || packages[0].Name != "Package Updated" {
		t.Fatalf("ListPackages: %+v err=%v", packages, err)
	}
	rateCard := &models.RateCard{ID: "rc-1", ProductID: product.ID, TargetType: "product", TargetID: product.ID, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := commercial.CreateRateCard(rateCard); err != nil {
		t.Fatalf("CreateRateCard: %v", err)
	}
	if gotRateCard, err := commercial.FindRateCardByID(rateCard.ID); err != nil || gotRateCard.ID != rateCard.ID {
		t.Fatalf("FindRateCardByID: %+v err=%v", gotRateCard, err)
	}
	if _, err := commercial.FindActiveRateCard("product", product.ID, nil); err != nil {
		t.Fatalf("FindActiveRateCard: %v", err)
	}
	rateCard.Status = "inactive"
	if err := commercial.SaveRateCard(rateCard); err != nil {
		t.Fatalf("SaveRateCard: %v", err)
	}
	if cards, err := commercial.ListRateCards(product.ID, "product"); err != nil || len(cards) != 1 || cards[0].Status != "inactive" {
		t.Fatalf("ListRateCards: %+v err=%v", cards, err)
	}

	quota := &models.QuotaLedger{ID: "ql-1", BillingSubjectType: "organization", BillingSubjectID: org.ID, BillableItemCode: item.Code, Direction: "credit", Units: 10, CreatedAt: time.Now()}
	reservationKey := "reserve-1"
	reservation := &models.ResourceReservation{ID: "rr-1", ReservationKey: &reservationKey, ResourceType: "quota", BillingSubjectType: "organization", BillingSubjectID: org.ID, BillableItemCode: item.Code, Status: "reserved", Units: 3, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := control.CreateQuotaLedger(quota); err != nil {
		t.Fatalf("CreateQuotaLedger: %v", err)
	}
	if err := control.CreateReservation(reservation); err != nil {
		t.Fatalf("CreateReservation: %v", err)
	}
	if _, err := control.FindReservationByID(reservation.ID); err != nil {
		t.Fatalf("FindReservationByID: %v", err)
	}
	if _, err := control.FindReservationByKey(reservationKey); err != nil {
		t.Fatalf("FindReservationByKey: %v", err)
	}
	if sum, err := control.SumQuotaDirection("organization", org.ID, item.Code, "credit"); err != nil || sum != 10 {
		t.Fatalf("SumQuotaDirection: %d err=%v", sum, err)
	}
	if sum, err := control.SumReserved("quota", "organization", org.ID, item.Code); err != nil || sum != 3 {
		t.Fatalf("SumReserved: %d err=%v", sum, err)
	}
}

func TestAuditRuntimeAndFinanceRepositories(t *testing.T) {
	db := newRepositoryTestDB(t)
	auditRepo := NewAuditRepository(db)
	runtimeRepo := NewRuntimeRepository(db)
	finance := NewFinanceRepository(db)

	auditLog := &models.AuditLog{ID: "audit-1", RequestID: "req-1", Action: "catalog.update", CreatedAt: time.Now()}
	if err := auditRepo.Create(auditLog); err != nil {
		t.Fatalf("Create audit log: %v", err)
	}
	var auditCount int64
	if err := db.Model(&models.AuditLog{}).Count(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatalf("Count audit logs: %d err=%v", auditCount, err)
	}

	def := &models.RuntimeProviderDefinition{ID: "def-1", Code: "comfyui_bridge", Name: "Comfy", ProviderType: "image_generation", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	endpoint := &models.RuntimeProductEndpoint{ID: "ep-1", ProductCode: "ecommerce", CallbackKind: "ecommerce_internal", Status: "active", BaseURL: "http://localhost", Secret: "secret", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	binding := &models.RuntimeProviderBinding{ID: "bind-1", ProductCode: "ecommerce", TaskType: "image_generation", ProviderCode: "comfyui_bridge", Priority: 50, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	storageBinding := &models.StorageBinding{ID: "sb-1", ProductCode: "ecommerce", Category: "ecommerce-assets", ProviderCode: "local", LocalBaseDir: "data/storage", Priority: 100, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	storageAsset := &models.StorageAsset{ID: "sa-1", ProductCode: "ecommerce", Category: "ecommerce-assets", SourceType: "template", SourceRef: "tmpl-1", StorageKey: "ecommerce/ecommerce-assets/file.png", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	job := &models.RuntimeJob{ID: "job-1", ProductCode: "ecommerce", TaskType: "image_generation", SourceType: "template", SourceID: "tmpl-1", Status: "queued", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	attempt := &models.RuntimeAttempt{ID: "att-1", RuntimeJobID: job.ID, AttemptNo: 1, ProviderCode: "comfyui_bridge", Status: "processing", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	session := &models.ChargeSession{ID: "cs-1", SourceType: "runtime_job", SourceID: job.ID, ProductCode: "ecommerce", Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := runtimeRepo.CreateProviderDefinition(def); err != nil {
		t.Fatalf("CreateProviderDefinition: %v", err)
	}
	if err := runtimeRepo.CreateProductEndpoint(endpoint); err != nil {
		t.Fatalf("CreateProductEndpoint: %v", err)
	}
	if err := runtimeRepo.CreateProviderBinding(binding); err != nil {
		t.Fatalf("CreateProviderBinding: %v", err)
	}
	if err := runtimeRepo.CreateStorageBinding(storageBinding); err != nil {
		t.Fatalf("CreateStorageBinding: %v", err)
	}
	if err := runtimeRepo.CreateStorageAsset(storageAsset); err != nil {
		t.Fatalf("CreateStorageAsset: %v", err)
	}
	if err := runtimeRepo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	if err := runtimeRepo.CreateRuntimeAttempt(attempt); err != nil {
		t.Fatalf("CreateRuntimeAttempt: %v", err)
	}
	if err := runtimeRepo.CreateChargeSession(session); err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	if _, err := runtimeRepo.FindProviderDefinitionByCode(def.Code); err != nil {
		t.Fatalf("FindProviderDefinitionByCode: %v", err)
	}
	if _, err := runtimeRepo.FindActiveProductEndpoint(endpoint.ProductCode); err != nil {
		t.Fatalf("FindActiveProductEndpoint: %v", err)
	}
	if items, err := runtimeRepo.ListProviderBindings(binding.ProductCode, binding.TaskType); err != nil || len(items) != 1 {
		t.Fatalf("ListProviderBindings: %+v err=%v", items, err)
	}
	if _, err := runtimeRepo.FindPreferredStorageBinding(storageBinding.ProductCode, storageBinding.Category); err != nil {
		t.Fatalf("FindPreferredStorageBinding: %v", err)
	}
	if _, err := runtimeRepo.FindStorageAssetBySource(storageAsset.ProductCode, storageAsset.Category, storageAsset.SourceType, storageAsset.SourceRef); err != nil {
		t.Fatalf("FindStorageAssetBySource: %v", err)
	}
	if _, err := runtimeRepo.FindRuntimeJobByID(job.ID); err != nil {
		t.Fatalf("FindRuntimeJobByID: %v", err)
	}
	if items, err := runtimeRepo.ListRuntimeAttempts(job.ID); err != nil || len(items) != 1 {
		t.Fatalf("ListRuntimeAttempts: %+v err=%v", items, err)
	}
	if _, err := runtimeRepo.FindChargeSessionBySource("runtime_job", job.ID); err != nil {
		t.Fatalf("FindChargeSessionBySource: %v", err)
	}

	definition := &models.AssetDefinition{AssetCode: "ECOM_CREDIT", ProductCode: "ecommerce", AssetType: "reward_credit", LifecycleType: "expiring", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	account := &models.WalletAccount{ID: "wa-1", BillingSubjectType: "organization", BillingSubjectID: "org-1", AssetCode: definition.AssetCode, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	bucket := &models.WalletBucket{ID: "wb-1", WalletAccountID: account.ID, Balance: 10, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	ledger := &models.WalletLedger{ID: "wl-1", WalletAccountID: account.ID, Direction: "credit", Amount: 10, Status: "posted", CreatedAt: time.Now()}
	settlement := &models.SettlementRecord{ID: "sr-1", EventID: "evt-1", BillingSubjectType: "organization", BillingSubjectID: "org-1", ProductCode: "ecommerce", Status: "settled", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	policy := &models.AllowancePolicy{ID: "ap-1", ProductCode: "ecommerce", BillingSubjectType: "organization", BillingSubjectID: "org-1", AssetCode: definition.AssetCode, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := finance.CreateAssetDefinition(definition); err != nil {
		t.Fatalf("CreateAssetDefinition: %v", err)
	}
	if err := finance.CreateWalletAccount(account); err != nil {
		t.Fatalf("CreateWalletAccount: %v", err)
	}
	if err := finance.CreateWalletBucket(bucket); err != nil {
		t.Fatalf("CreateWalletBucket: %v", err)
	}
	if err := finance.CreateWalletLedger(ledger); err != nil {
		t.Fatalf("CreateWalletLedger: %v", err)
	}
	if err := finance.CreateSettlementRecord(settlement); err != nil {
		t.Fatalf("CreateSettlementRecord: %v", err)
	}
	if err := finance.CreateAllowancePolicy(policy); err != nil {
		t.Fatalf("CreateAllowancePolicy: %v", err)
	}
	if _, err := finance.FindAssetDefinition(definition.AssetCode); err != nil {
		t.Fatalf("FindAssetDefinition: %v", err)
	}
	if items, err := finance.ListWalletAccounts("organization", "org-1"); err != nil || len(items) != 1 {
		t.Fatalf("ListWalletAccounts: %+v err=%v", items, err)
	}
	if _, err := finance.FindWalletBucketByID(bucket.ID); err != nil {
		t.Fatalf("FindWalletBucketByID: %v", err)
	}
	if items, err := finance.ListSettlementRecords("organization", "org-1", "ecommerce"); err != nil || len(items) != 1 {
		t.Fatalf("ListSettlementRecords: %+v err=%v", items, err)
	}
	discount := &models.DiscountLedger{ID: "dl-1", ProductCode: "ecommerce", BillingSubjectType: "organization", BillingSubjectID: "org-1", DiscountType: "campaign", Amount: 10, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	reward := &models.RewardLedger{ID: "rl-1", ProductCode: "ecommerce", BeneficiarySubjectType: "organization", BeneficiarySubjectID: "org-1", RewardType: "campaign_reward", AssetCode: definition.AssetCode, Amount: 10, Status: "issued", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	commission := &models.CommissionLedger{ID: "cl-1", ProductCode: "ecommerce", BeneficiarySubjectType: "organization", BeneficiarySubjectID: "org-1", CommissionType: "referral", Amount: 12, Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	program := &models.ReferralProgram{ID: "rp-1", ProductCode: "ecommerce", ProgramCode: "signup", Name: "Signup", TriggerType: "signup", CommissionPolicy: "fixed_amount", CommissionCurrency: definition.AssetCode, CommissionFixedAmount: 10, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	code := &models.ReferralCode{ID: "rc-1", ProgramID: program.ID, ProductCode: "ecommerce", Code: "HELLO50", PromoterSubjectType: "organization", PromoterSubjectID: "org-1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	conversion := &models.ReferralConversion{ID: "conv-1", ProgramID: program.ID, ReferralCodeID: code.ID, ProductCode: "ecommerce", TriggerType: "signup", PromoterSubjectType: "organization", PromoterSubjectID: "org-1", ReferredSubjectType: "organization", ReferredSubjectID: "org-2", Status: "completed", ReferenceType: "order", ReferenceID: "order-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := finance.CreateDiscountLedger(discount); err != nil {
		t.Fatalf("CreateDiscountLedger: %v", err)
	}
	if err := finance.CreateRewardLedger(reward); err != nil {
		t.Fatalf("CreateRewardLedger: %v", err)
	}
	if err := finance.CreateCommissionLedger(commission); err != nil {
		t.Fatalf("CreateCommissionLedger: %v", err)
	}
	if err := finance.CreateReferralProgram(program); err != nil {
		t.Fatalf("CreateReferralProgram: %v", err)
	}
	if err := finance.CreateReferralCode(code); err != nil {
		t.Fatalf("CreateReferralCode: %v", err)
	}
	if err := finance.CreateReferralConversion(conversion); err != nil {
		t.Fatalf("CreateReferralConversion: %v", err)
	}
	if items, err := finance.ListDiscountLedgers("ecommerce", "organization", "org-1"); err != nil || len(items) != 1 {
		t.Fatalf("ListDiscountLedgers: %+v err=%v", items, err)
	}
	if _, err := finance.FindRewardLedgerByID(reward.ID); err != nil {
		t.Fatalf("FindRewardLedgerByID: %v", err)
	}
	reward.Status = "redeemed"
	if err := finance.SaveRewardLedger(reward); err != nil {
		t.Fatalf("SaveRewardLedger: %v", err)
	}
	updatedReward, err := finance.FindRewardLedgerByID(reward.ID)
	if err != nil || updatedReward.Status != "redeemed" {
		t.Fatalf("updated reward mismatch: %+v err=%v", updatedReward, err)
	}
	if items, err := finance.ListRewardLedgers("ecommerce", "organization", "org-1"); err != nil || len(items) != 1 {
		t.Fatalf("ListRewardLedgers: %+v err=%v", items, err)
	}
	if _, err := finance.FindCommissionLedgerByID(commission.ID); err != nil {
		t.Fatalf("FindCommissionLedgerByID: %v", err)
	}
	commission.Status = "earned"
	if err := finance.SaveCommissionLedger(commission); err != nil {
		t.Fatalf("SaveCommissionLedger: %v", err)
	}
	updatedCommission, err := finance.FindCommissionLedgerByID(commission.ID)
	if err != nil || updatedCommission.Status != "earned" {
		t.Fatalf("updated commission mismatch: %+v err=%v", updatedCommission, err)
	}
	if items, err := finance.ListCommissionLedgers("ecommerce", "organization", "org-1", "earned"); err != nil || len(items) != 1 {
		t.Fatalf("ListCommissionLedgers: %+v err=%v", items, err)
	}
	if _, err := finance.FindReferralProgramByID(program.ID); err != nil {
		t.Fatalf("FindReferralProgramByID: %v", err)
	}
	if _, err := finance.FindReferralProgramByCode(program.ProgramCode); err != nil {
		t.Fatalf("FindReferralProgramByCode: %v", err)
	}
	if items, err := finance.ListReferralPrograms("ecommerce", "active"); err != nil || len(items) != 1 {
		t.Fatalf("ListReferralPrograms: %+v err=%v", items, err)
	}
	if _, err := finance.FindReferralCodeByCode(code.Code); err != nil {
		t.Fatalf("FindReferralCodeByCode: %v", err)
	}
	code.Status = "disabled"
	if err := finance.SaveReferralCode(code); err != nil {
		t.Fatalf("SaveReferralCode: %v", err)
	}
	updatedCode, err := finance.FindReferralCodeByCode(code.Code)
	if err != nil || updatedCode.Status != "disabled" {
		t.Fatalf("updated referral code mismatch: %+v err=%v", updatedCode, err)
	}
	if items, err := finance.ListReferralCodes("", "organization", "org-1", "disabled"); err != nil || len(items) != 1 {
		t.Fatalf("ListReferralCodes: %+v err=%v", items, err)
	}
	if _, err := finance.FindReferralConversionByID(conversion.ID); err != nil {
		t.Fatalf("FindReferralConversionByID: %v", err)
	}
	if _, err := finance.FindReferralConversionByReference(conversion.ReferenceType, conversion.ReferenceID); err != nil {
		t.Fatalf("FindReferralConversionByReference: %v", err)
	}
	conversion.Status = "reward_issued"
	if err := finance.SaveReferralConversion(conversion); err != nil {
		t.Fatalf("SaveReferralConversion: %v", err)
	}
	updatedConversion, err := finance.FindReferralConversionByID(conversion.ID)
	if err != nil || updatedConversion.Status != "reward_issued" {
		t.Fatalf("updated conversion mismatch: %+v err=%v", updatedConversion, err)
	}
	if items, err := finance.ListReferralConversions("ecommerce", "organization", "org-1", "reward_issued"); err != nil || len(items) != 1 {
		t.Fatalf("ListReferralConversions: %+v err=%v", items, err)
	}
}

func TestFinanceChannelRepositories(t *testing.T) {
	db := newRepositoryTestDB(t)
	finance := NewFinanceRepository(db)
	now := time.Now().UTC()
	later := now.Add(24 * time.Hour)

	partner := &models.ChannelPartner{ID: "cp-1", Code: "PARTNER_A", Name: "Partner A", PartnerType: "channel", Status: "active", RiskLevel: "low", DefaultCurrency: "CNY", CreatedAt: now, UpdatedAt: now}
	program := &models.ChannelProgram{ID: "cprog-1", ProductCode: "menu_ai", ProgramCode: "PROGRAM_A", Name: "Program A", ProgramType: "revenue_share", Status: "active", DefaultSettlementCycle: "monthly", CreatedAt: now, UpdatedAt: now}
	binding := &models.ChannelPartnerBinding{ID: "bind-1", ProductCode: "menu_ai", OrgID: "org-1", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, BindingSource: "signup_code", BindingScope: "organization", Status: "active", CreatedAt: now, UpdatedAt: now}
	policy := &models.ChannelCommissionPolicy{ID: "policy-1", ChannelProgramID: program.ID, ProductCode: "menu_ai", PolicyCode: "POLICY_A", Status: "active", AppliesTo: "usage_charge", TriggerType: "charge_recorded", CommissionBase: "net_collected_amount", RateType: "fixed_rate", FixedRateBps: 1000, CooldownDays: 3, SettlementCycle: "monthly", Priority: 1, EffectiveFrom: &now, EffectiveTo: &later, CreatedAt: now, UpdatedAt: now}
	version := &models.ChannelCommissionPolicyVersion{ID: "version-1", PolicyID: policy.ID, VersionCode: "VERSION_A", Status: "active", AppliesTo: "usage_charge", TriggerType: "charge_recorded", CommissionBase: "net_collected_amount", RateType: "fixed_rate", FixedRateBps: 1200, CooldownDays: 5, SettlementCycle: "monthly", EffectiveFrom: &now, EffectiveTo: &later, CreatedAt: now, UpdatedAt: now}
	assignment := &models.ChannelCommissionPolicyAssignment{ID: "assign-1", PolicyVersionID: version.ID, AssignmentLevel: "channel_partner", ChannelPartnerID: partner.ID, ProductCode: "menu_ai", Currency: "CNY", Priority: 10, Status: "active", EffectiveFrom: &now, EffectiveTo: &later, CreatedAt: now, UpdatedAt: now}
	profit := &models.ChannelProfitSnapshot{ID: "profit-1", SourceEventID: "evt-profit-1", ProductCode: "menu_ai", OrgID: "org-1", SourceChargeID: "charge-1", Currency: "CNY", NetCollectedAmount: 10000, DistributableProfitAmount: 9000, CommissionRecognitionAt: now, CreatedAt: now, UpdatedAt: now}
	audit := &models.ChannelPolicyResolutionAudit{ID: "audit-1", CalculationTraceID: "trace-1", EventID: "evt-profit-1", ProductCode: "menu_ai", OrgID: "org-1", BindingID: binding.ID, ChannelPartnerID: partner.ID, SourceChargeID: "charge-1", AppliesTo: "usage_charge", PolicyID: policy.ID, PolicyVersionID: version.ID, AssignmentID: assignment.ID, AssignmentLevel: assignment.AssignmentLevel, MatchedRuleCode: "partner", ResolutionStatus: "matched", CreatedAt: now}
	commissionAvailable := now.Add(-2 * time.Hour)
	commissionEarned := now.Add(-time.Hour)
	commission := &models.ChannelCommissionLedger{ID: "cledger-1", LedgerNo: "LEDGER-1", ProductCode: "menu_ai", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, BindingID: binding.ID, PolicyID: policy.ID, PolicyVersionID: version.ID, ProfitSnapshotID: profit.ID, AssignmentLevel: assignment.AssignmentLevel, MatchedRuleCode: "partner", CalculationTraceID: audit.CalculationTraceID, SettlementSubjectType: "organization", SettlementSubjectID: "org-1", SourceEventID: profit.SourceEventID, SourceChargeID: profit.SourceChargeID, AppliesTo: "usage_charge", Currency: "CNY", NetCollectedAmount: 10000, CommissionableAmount: 9000, CommissionRateBps: 1200, CommissionAmount: 1080, SettleableAmount: 1080, Status: "earned", AvailableAt: &commissionAvailable, EarnedAt: &commissionEarned, CreatedAt: now, UpdatedAt: now}
	eligibleCommission := &models.ChannelCommissionLedger{ID: "cledger-2", LedgerNo: "LEDGER-2", ProductCode: "menu_ai", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, BindingID: binding.ID, PolicyID: policy.ID, PolicyVersionID: version.ID, ProfitSnapshotID: profit.ID, AssignmentLevel: assignment.AssignmentLevel, MatchedRuleCode: "partner", CalculationTraceID: "trace-2", SettlementSubjectType: "organization", SettlementSubjectID: "org-1", SourceEventID: "evt-profit-2", SourceChargeID: "charge-2", AppliesTo: "usage_charge", Currency: "CNY", NetCollectedAmount: 5000, CommissionableAmount: 4500, CommissionRateBps: 1200, CommissionAmount: 540, SettleableAmount: 540, Status: "earned", AvailableAt: &commissionAvailable, EarnedAt: &commissionEarned, CreatedAt: now, UpdatedAt: now}
	clawback := &models.ChannelClawbackLedger{ID: "claw-1", ProductCode: "menu_ai", ChannelPartnerID: partner.ID, SourceCommissionLedgerID: commission.ID, SourceRefundEventID: "refund-evt-1", SourceRefundID: "refund-1", ClawbackType: "full_refund", Currency: "CNY", ClawbackAmount: 1080, Status: "pending", CreatedAt: now, UpdatedAt: now}
	eligibleClawback := &models.ChannelClawbackLedger{ID: "claw-2", ProductCode: "menu_ai", ChannelPartnerID: partner.ID, SourceCommissionLedgerID: eligibleCommission.ID, SourceRefundEventID: "refund-evt-2", SourceRefundID: "refund-2", ClawbackType: "partial_refund", Currency: "CNY", ClawbackAmount: 200, Status: "pending", CreatedAt: now, UpdatedAt: now}
	adjustmentEffective := now.Add(-30 * time.Minute)
	adjustment := &models.ChannelCommissionAdjustmentLedger{ID: "adj-1", ProductCode: "menu_ai", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, SourceCommissionLedgerID: commission.ID, AdjustmentType: "manual_credit", Currency: "CNY", AdjustmentAmount: 100, ReasonCode: "ops", Status: "pending", EffectiveAt: &adjustmentEffective, CreatedAt: now, UpdatedAt: now}
	eligibleAdjustment := &models.ChannelCommissionAdjustmentLedger{ID: "adj-2", ProductCode: "menu_ai", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, SourceCommissionLedgerID: eligibleCommission.ID, AdjustmentType: "manual_credit", Currency: "CNY", AdjustmentAmount: 80, ReasonCode: "ops", Status: "pending", EffectiveAt: &adjustmentEffective, CreatedAt: now, UpdatedAt: now}
	batch := &models.ChannelSettlementBatch{ID: "batch-1", BatchNo: "BATCH-1", ProductCode: "menu_ai", ChannelProgramID: program.ID, SettlementCycle: "monthly", PeriodStart: now.Add(-30 * 24 * time.Hour), PeriodEnd: later, Currency: "CNY", Status: "generated", CreatedAt: now, UpdatedAt: now}
	item := &models.ChannelSettlementItem{ID: "item-1", SettlementBatchID: batch.ID, ChannelPartnerID: partner.ID, Currency: "CNY", CommissionAmount: 1080, ClawbackAmount: 1080, AdjustmentAmount: 100, NetAmount: 100, Status: "generated", CreatedAt: now, UpdatedAt: now}
	itemLedger := &models.ChannelSettlementItemLedger{ID: "item-ledger-1", SettlementBatchID: batch.ID, SettlementItemID: item.ID, CommissionLedgerID: commission.ID, CreatedAt: now}
	itemClawback := &models.ChannelSettlementItemClawback{ID: "item-claw-1", SettlementBatchID: batch.ID, SettlementItemID: item.ID, ClawbackLedgerID: clawback.ID, CreatedAt: now}
	itemAdjustment := &models.ChannelSettlementItemAdjustment{ID: "item-adj-1", SettlementBatchID: batch.ID, SettlementItemID: item.ID, AdjustmentLedgerID: adjustment.ID, CreatedAt: now}

	for name, fn := range map[string]func() error{
		"CreateChannelPartner":                    func() error { return finance.CreateChannelPartner(partner) },
		"CreateChannelProgram":                    func() error { return finance.CreateChannelProgram(program) },
		"CreateChannelBinding":                    func() error { return finance.CreateChannelBinding(binding) },
		"CreateChannelCommissionPolicy":           func() error { return finance.CreateChannelCommissionPolicy(policy) },
		"CreateChannelCommissionPolicyVersion":    func() error { return finance.CreateChannelCommissionPolicyVersion(version) },
		"CreateChannelCommissionPolicyAssignment": func() error { return finance.CreateChannelCommissionPolicyAssignment(assignment) },
		"CreateChannelProfitSnapshot":             func() error { return finance.CreateChannelProfitSnapshot(profit) },
		"CreateChannelPolicyResolutionAudit":      func() error { return finance.CreateChannelPolicyResolutionAudit(audit) },
		"CreateChannelCommissionLedger":           func() error { return finance.CreateChannelCommissionLedger(commission) },
		"CreateEligibleChannelCommissionLedger":   func() error { return finance.CreateChannelCommissionLedger(eligibleCommission) },
		"CreateChannelClawbackLedger":             func() error { return finance.CreateChannelClawbackLedger(clawback) },
		"CreateEligibleChannelClawbackLedger":     func() error { return finance.CreateChannelClawbackLedger(eligibleClawback) },
		"CreateChannelCommissionAdjustmentLedger": func() error { return finance.CreateChannelCommissionAdjustmentLedger(adjustment) },
		"CreateEligibleChannelAdjustmentLedger":   func() error { return finance.CreateChannelCommissionAdjustmentLedger(eligibleAdjustment) },
	} {
		if err := fn(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := db.Create(batch).Error; err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	if err := db.Create(itemLedger).Error; err != nil {
		t.Fatalf("create item ledger: %v", err)
	}
	if err := db.Create(itemClawback).Error; err != nil {
		t.Fatalf("create item clawback: %v", err)
	}
	if err := db.Create(itemAdjustment).Error; err != nil {
		t.Fatalf("create item adjustment: %v", err)
	}

	if got, err := finance.FindChannelPartnerByID(partner.ID); err != nil || got.Code != partner.Code {
		t.Fatalf("FindChannelPartnerByID: %+v err=%v", got, err)
	}
	if got, err := finance.FindChannelPartnerByCode(partner.Code); err != nil || got.ID != partner.ID {
		t.Fatalf("FindChannelPartnerByCode: %+v err=%v", got, err)
	}
	if items, err := finance.ListChannelPartners("active"); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelPartners: %+v err=%v", items, err)
	}
	if got, err := finance.FindChannelProgramByID(program.ID); err != nil || got.ProgramCode != program.ProgramCode {
		t.Fatalf("FindChannelProgramByID: %+v err=%v", got, err)
	}
	if got, err := finance.FindChannelProgramByCode(program.ProgramCode); err != nil || got.ID != program.ID {
		t.Fatalf("FindChannelProgramByCode: %+v err=%v", got, err)
	}
	if items, err := finance.ListChannelPrograms(program.ProductCode, "active"); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelPrograms: %+v err=%v", items, err)
	}
	if got, err := finance.FindActiveChannelBinding(binding.ProductCode, binding.OrgID, now); err != nil || got.ID != binding.ID {
		t.Fatalf("FindActiveChannelBinding: %+v err=%v", got, err)
	}
	binding.ReasonCode = "manual_review"
	if err := finance.SaveChannelBinding(binding); err != nil {
		t.Fatalf("SaveChannelBinding: %v", err)
	}
	if items, err := finance.ListChannelBindings(binding.ProductCode, binding.OrgID, "active"); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelBindings: %+v err=%v", items, err)
	}
	if got, err := finance.FindChannelCommissionPolicyByCode(policy.PolicyCode); err != nil || got.ID != policy.ID {
		t.Fatalf("FindChannelCommissionPolicyByCode: %+v err=%v", got, err)
	}
	if got, err := finance.FindChannelCommissionPolicyByID(policy.ID); err != nil || got.PolicyCode != policy.PolicyCode {
		t.Fatalf("FindChannelCommissionPolicyByID: %+v err=%v", got, err)
	}
	if got, err := finance.FindApplicableChannelCommissionPolicy(program.ID, program.ProductCode, policy.AppliesTo, now); err != nil || got.ID != policy.ID {
		t.Fatalf("FindApplicableChannelCommissionPolicy: %+v err=%v", got, err)
	}
	if items, err := finance.ListChannelCommissionPolicies(program.ID, program.ProductCode, "active"); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelCommissionPolicies: %+v err=%v", items, err)
	}
	if got, err := finance.FindChannelCommissionPolicyVersionByID(version.ID); err != nil || got.VersionCode != version.VersionCode {
		t.Fatalf("FindChannelCommissionPolicyVersionByID: %+v err=%v", got, err)
	}
	if got, err := finance.FindChannelCommissionPolicyVersionByCode(version.VersionCode); err != nil || got.ID != version.ID {
		t.Fatalf("FindChannelCommissionPolicyVersionByCode: %+v err=%v", got, err)
	}
	if items, err := finance.ListChannelCommissionPolicyVersions(policy.ID, "active"); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelCommissionPolicyVersions: %+v err=%v", items, err)
	}
	if items, err := finance.ListChannelCommissionPolicyAssignments(version.ID, program.ProductCode, "active"); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelCommissionPolicyAssignments: %+v err=%v", items, err)
	}
	if items, err := finance.ListCandidateChannelCommissionPolicyAssignments(program.ProductCode, now); err != nil || len(items) != 1 {
		t.Fatalf("ListCandidateChannelCommissionPolicyAssignments: %+v err=%v", items, err)
	}
	if got, err := finance.FindChannelProfitSnapshotBySourceEventID(profit.SourceEventID); err != nil || got.ID != profit.ID {
		t.Fatalf("FindChannelProfitSnapshotBySourceEventID: %+v err=%v", got, err)
	}
	if items, err := finance.ListChannelProfitSnapshots(profit.ProductCode, profit.OrgID); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelProfitSnapshots: %+v err=%v", items, err)
	}
	if got, err := finance.FindChannelCommissionLedgerByID(commission.ID); err != nil || got.LedgerNo != commission.LedgerNo {
		t.Fatalf("FindChannelCommissionLedgerByID: %+v err=%v", got, err)
	}
	if got, err := finance.FindChannelCommissionLedgerBySourceEventID(commission.SourceEventID); err != nil || got.ID != commission.ID {
		t.Fatalf("FindChannelCommissionLedgerBySourceEventID: %+v err=%v", got, err)
	}
	revID := "refund-evt-2"
	commission.ReversalEventID = &revID
	if err := finance.SaveChannelCommissionLedger(commission); err != nil {
		t.Fatalf("SaveChannelCommissionLedger: %v", err)
	}
	if got, err := finance.FindChannelCommissionLedgerByReversalEventID(revID); err != nil || got.ID != commission.ID {
		t.Fatalf("FindChannelCommissionLedgerByReversalEventID: %+v err=%v", got, err)
	}
	if got, err := finance.FindChannelCommissionLedgerBySourceChargeID(commission.ProductCode, commission.SourceChargeID); err != nil || got.ID != commission.ID {
		t.Fatalf("FindChannelCommissionLedgerBySourceChargeID: %+v err=%v", got, err)
	}
	if items, err := finance.ListChannelCommissionLedgers(commission.ProductCode, partner.ID, "earned"); err != nil || len(items) != 2 {
		t.Fatalf("ListChannelCommissionLedgers: %+v err=%v", items, err)
	}
	if got, err := finance.FindChannelClawbackLedgerBySourceRefundEventID(clawback.SourceRefundEventID); err != nil || got.ID != clawback.ID {
		t.Fatalf("FindChannelClawbackLedgerBySourceRefundEventID: %+v err=%v", got, err)
	}
	if items, err := finance.ListChannelClawbackLedgers(clawback.ProductCode, partner.ID, "pending"); err != nil || len(items) != 2 {
		t.Fatalf("ListChannelClawbackLedgers: %+v err=%v", items, err)
	}
	if items, err := finance.ListMatureableChannelCommissionLedgers(commission.ProductCode, commission.ChannelProgramID, now); err != nil || len(items) != 0 {
		t.Fatalf("ListMatureableChannelCommissionLedgers before pending: %+v err=%v", items, err)
	}
	commission.Status = "pending"
	if err := finance.SaveChannelCommissionLedger(commission); err != nil {
		t.Fatalf("SaveChannelCommissionLedger pending: %v", err)
	}
	if items, err := finance.ListMatureableChannelCommissionLedgers(commission.ProductCode, commission.ChannelProgramID, now); err != nil || len(items) != 1 {
		t.Fatalf("ListMatureableChannelCommissionLedgers: %+v err=%v", items, err)
	}
	commission.Status = "earned"
	if err := finance.SaveChannelCommissionLedger(commission); err != nil {
		t.Fatalf("SaveChannelCommissionLedger earned: %v", err)
	}
	if items, err := finance.ListEligibleChannelCommissionLedgers(commission.ProductCode, commission.ChannelProgramID, commission.Currency, later); err != nil || len(items) != 1 || items[0].ID != eligibleCommission.ID {
		t.Fatalf("ListEligibleChannelCommissionLedgers: %+v err=%v", items, err)
	}
	if items, err := finance.ListEligibleChannelClawbackLedgers(clawback.ProductCode, commission.ChannelProgramID, clawback.Currency, later); err != nil || len(items) != 1 || items[0].ID != eligibleClawback.ID {
		t.Fatalf("ListEligibleChannelClawbackLedgers: %+v err=%v", items, err)
	}
	if items, err := finance.ListChannelCommissionAdjustmentLedgers(adjustment.ProductCode, partner.ID, "pending"); err != nil || len(items) != 2 {
		t.Fatalf("ListChannelCommissionAdjustmentLedgers: %+v err=%v", items, err)
	}
	if items, err := finance.ListEligibleChannelCommissionAdjustmentLedgers(adjustment.ProductCode, adjustment.ChannelProgramID, adjustment.Currency, later); err != nil || len(items) != 1 || items[0].ID != eligibleAdjustment.ID {
		t.Fatalf("ListEligibleChannelCommissionAdjustmentLedgers: %+v err=%v", items, err)
	}
	if got, err := finance.FindChannelSettlementBatchByID(batch.ID); err != nil || got.BatchNo != batch.BatchNo {
		t.Fatalf("FindChannelSettlementBatchByID: %+v err=%v", got, err)
	}
	if got, err := finance.FindChannelSettlementBatchByPeriod(batch.ProductCode, batch.ChannelProgramID, batch.PeriodStart, batch.PeriodEnd); err != nil || got.ID != batch.ID {
		t.Fatalf("FindChannelSettlementBatchByPeriod: %+v err=%v", got, err)
	}
	if items, err := finance.ListChannelSettlementBatches(batch.ProductCode, batch.ChannelProgramID, "generated"); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelSettlementBatches: %+v err=%v", items, err)
	}
	if items, err := finance.ListChannelSettlementItems(batch.ID, partner.ID, "generated"); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelSettlementItems: %+v err=%v", items, err)
	}
	if items, err := finance.ListChannelSettlementItemLedgers(batch.ID, item.ID); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelSettlementItemLedgers: %+v err=%v", items, err)
	}
	if items, err := finance.ListChannelSettlementItemClawbacks(batch.ID, item.ID); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelSettlementItemClawbacks: %+v err=%v", items, err)
	}
	if items, err := finance.ListChannelSettlementItemAdjustments(batch.ID, item.ID); err != nil || len(items) != 1 {
		t.Fatalf("ListChannelSettlementItemAdjustments: %+v err=%v", items, err)
	}
}
