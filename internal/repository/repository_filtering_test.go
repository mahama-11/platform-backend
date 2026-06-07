package repository

import (
	"testing"
	"time"

	"platform-service/internal/models"
)

func TestRepositoryFilteringMutationAndFallbackBranches(t *testing.T) {
	db := newRepositoryTestDB(t)
	now := time.Now().UTC()
	core := NewCoreRepository(db)
	commercial := NewCommercialRepository(db)
	control := NewControlRepository(db)
	runtimeRepo := NewRuntimeRepository(db)
	finance := NewFinanceRepository(db)

	orgA := &models.Organization{ID: "org-filter-a", Name: "Alpha Store", BillingEmail: "alpha@example.com", Status: "active", CreatedAt: now, UpdatedAt: now}
	orgB := &models.Organization{ID: "org-filter-b", Name: "Beta Store", BillingEmail: "beta@example.com", Status: "disabled", CreatedAt: now.Add(time.Second), UpdatedAt: now}
	userA := &models.User{ID: "user-filter-a", Email: "alpha@example.com", FullName: "Alpha Owner", Name: "Alpha", CurrentOrgID: orgA.ID, Status: "active", CreatedAt: now, UpdatedAt: now}
	userB := &models.User{ID: "user-filter-b", Email: "beta@example.com", FullName: "Beta Owner", Name: "Beta", CurrentOrgID: orgB.ID, Status: "disabled", CreatedAt: now.Add(time.Second), UpdatedAt: now}
	members := []models.OrganizationMember{
		{ID: "member-filter-a", OrganizationID: orgA.ID, UserID: userA.ID, Role: "owner", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "member-filter-b", OrganizationID: orgB.ID, UserID: userB.ID, Role: "member", Status: "disabled", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create([]*models.Organization{orgA, orgB}).Error; err != nil {
		t.Fatalf("seed orgs: %v", err)
	}
	if err := db.Create([]*models.User{userA, userB}).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatalf("seed members: %v", err)
	}
	if orgs, total, err := core.ListOrganizations(OrganizationListFilter{Query: "alpha", Status: "active", Limit: -1, Offset: -10}); err != nil || total != 1 || len(orgs) != 1 || orgs[0].ID != orgA.ID {
		t.Fatalf("ListOrganizations filter/defaults: orgs=%+v total=%d err=%v", orgs, total, err)
	}
	if users, total, err := core.ListUsers(UserListFilter{Query: "Beta", Status: "disabled", Limit: 5000, Offset: -1}); err != nil || total != 1 || len(users) != 1 || users[0].ID != userB.ID {
		t.Fatalf("ListUsers filter/defaults: users=%+v total=%d err=%v", users, total, err)
	}
	if orgs, err := core.ListOrganizationsByIDs(nil); err != nil || len(orgs) != 0 {
		t.Fatalf("ListOrganizationsByIDs empty: %+v err=%v", orgs, err)
	}
	if members, err := core.ListMembershipsByOrgIDs([]string{orgA.ID, orgB.ID}); err != nil || len(members) != 1 || members[0].OrganizationID != orgA.ID {
		t.Fatalf("ListMembershipsByOrgIDs: %+v err=%v", members, err)
	}
	if users, err := core.ListUsersByIDs([]string{userA.ID, userB.ID}); err != nil || len(users) != 2 {
		t.Fatalf("ListUsersByIDs: %+v err=%v", users, err)
	}
	if users, err := core.ListUsersByIDs(nil); err != nil || len(users) != 0 {
		t.Fatalf("ListUsersByIDs empty: %+v err=%v", users, err)
	}
	if members, err := core.ListMembershipsByUserIDs([]string{userA.ID, userB.ID}); err != nil || len(members) != 1 || members[0].UserID != userA.ID {
		t.Fatalf("ListMembershipsByUserIDs: %+v err=%v", members, err)
	}

	product := &models.Product{ID: "prod-filter", Code: "filter_product", Name: "Filter Product", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := commercial.CreateProduct(product); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if got, err := commercial.FindProductByID(product.ID); err != nil || got.Code != product.Code {
		t.Fatalf("FindProductByID: %+v err=%v", got, err)
	}
	if got, err := commercial.FindProductByCode(product.Code); err != nil || got.ID != product.ID {
		t.Fatalf("FindProductByCode: %+v err=%v", got, err)
	}
	product.Name = "Filter Product Updated"
	if err := commercial.SaveProduct(product); err != nil {
		t.Fatalf("SaveProduct: %v", err)
	}
	if err := commercial.DeleteProduct("missing-product"); err != nil {
		t.Fatalf("DeleteProduct missing: %v", err)
	}
	sku := &models.SKU{ID: "sku-filter", ProductID: product.ID, Code: "sku-filter", Name: "SKU", Status: "active", CreatedAt: now, UpdatedAt: now}
	item := &models.BillableItem{ID: "billable-filter", ProductID: product.ID, Code: "FILTER_UNITS", Name: "Filter Units", Status: "active", CreatedAt: now, UpdatedAt: now}
	entity := &models.CommercialEntity{ID: "entity-filter", Code: "entity-filter", Name: "Entity", EntityType: "internal", Status: "active", CreatedAt: now, UpdatedAt: now}
	profile := &models.BillingProfile{ID: "profile-filter", Code: "profile-filter", ProductID: product.ID, CommercialEntityID: entity.ID, Status: "active", CreatedAt: now, UpdatedAt: now}
	pkg := &models.CommercialPackage{ID: "package-filter", ProductID: product.ID, Code: "package-filter", Name: "Package", Status: "active", CreatedAt: now, UpdatedAt: now}
	rateCard := &models.RateCard{ID: "rate-filter", ProductID: product.ID, TargetType: "product", TargetID: product.ID, Status: "active", CreatedAt: now, UpdatedAt: now}
	for name, fn := range map[string]func() error{
		"sku": func() error { return commercial.CreateSKU(sku) }, "item": func() error { return commercial.CreateBillableItem(item) }, "entity": func() error { return commercial.CreateCommercialEntity(entity) }, "profile": func() error { return commercial.CreateBillingProfile(profile) }, "package": func() error { return commercial.CreatePackage(pkg) }, "rate": func() error { return commercial.CreateRateCard(rateCard) },
	} {
		if err := fn(); err != nil {
			t.Fatalf("seed commercial %s: %v", name, err)
		}
	}
	if items, err := commercial.ListSKUs(""); err != nil || len(items) != 1 {
		t.Fatalf("ListSKUs all: %+v err=%v", items, err)
	}
	if items, err := commercial.ListBillableItems(""); err != nil || len(items) != 1 {
		t.Fatalf("ListBillableItems all: %+v err=%v", items, err)
	}
	if profiles, err := commercial.ListBillingProfiles(""); err != nil || len(profiles) != 1 {
		t.Fatalf("ListBillingProfiles all: %+v err=%v", profiles, err)
	}
	if packages, err := commercial.ListPackages(""); err != nil || len(packages) != 1 {
		t.Fatalf("ListPackages all: %+v err=%v", packages, err)
	}
	if cards, err := commercial.ListRateCards("", ""); err != nil || len(cards) != 1 {
		t.Fatalf("ListRateCards all: %+v err=%v", cards, err)
	}
	for name, fn := range map[string]func() error{
		"DeleteSKU": func() error { return commercial.DeleteSKU("missing") }, "DeleteBillableItem": func() error { return commercial.DeleteBillableItem("missing") }, "DeleteCommercialEntity": func() error { return commercial.DeleteCommercialEntity("missing") }, "DeleteBillingProfile": func() error { return commercial.DeleteBillingProfile("missing") }, "DeletePackage": func() error { return commercial.DeletePackage("missing") }, "DeleteRateCard": func() error { return commercial.DeleteRateCard("missing") },
	} {
		if err := fn(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	if err := control.LockPackageActivationReference(product.Code, "organization", orgA.ID, ""); err != nil {
		t.Fatalf("LockPackageActivationReference sqlite/empty: %v", err)
	}
	quotaRef := &models.QuotaLedger{ID: "quota-filter", BillingSubjectType: "organization", BillingSubjectID: orgA.ID, BillableItemCode: item.Code, Direction: "credit", ReferenceID: "ref-1", Units: 7, CreatedAt: now}
	grantPolicy := &models.QuotaGrantPolicy{ID: "qgp-filter", ProductCode: product.Code, PackageCode: pkg.Code, BillableItemCode: item.Code, Units: 10, Status: "active", CreatedAt: now, UpdatedAt: now}
	capPolicy := &models.PackageCapabilityPolicy{ID: "pcp-filter", ProductCode: product.Code, PackageCode: pkg.Code, CapabilityCode: "capability.image", GrantValue: "enabled", Status: "active", CreatedAt: now, UpdatedAt: now}
	capGrant := &models.CapabilityGrant{ID: "cap-grant-filter", ProductCode: product.Code, BillingSubjectType: "organization", BillingSubjectID: orgA.ID, CapabilityCode: capPolicy.CapabilityCode, SourceType: "package", SourceID: pkg.ID, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := control.CreateQuotaLedger(quotaRef); err != nil {
		t.Fatalf("CreateQuotaLedger: %v", err)
	}
	if err := control.CreateQuotaGrantPolicy(grantPolicy); err != nil {
		t.Fatalf("CreateQuotaGrantPolicy: %v", err)
	}
	if err := control.CreatePackageCapabilityPolicy(capPolicy); err != nil {
		t.Fatalf("CreatePackageCapabilityPolicy: %v", err)
	}
	if err := control.CreateCapabilityGrant(capGrant); err != nil {
		t.Fatalf("CreateCapabilityGrant: %v", err)
	}
	if got, err := control.FindQuotaLedgerByReference("organization", orgA.ID, item.Code, "credit", "ref-1"); err != nil || got.ID != quotaRef.ID {
		t.Fatalf("FindQuotaLedgerByReference: %+v err=%v", got, err)
	}
	if policies, err := control.ListQuotaGrantPolicies(product.Code, pkg.Code); err != nil || len(policies) != 1 {
		t.Fatalf("ListQuotaGrantPolicies: %+v err=%v", policies, err)
	}
	if got, err := control.FindQuotaGrantPolicyByID(grantPolicy.ID); err != nil || got.ID != grantPolicy.ID {
		t.Fatalf("FindQuotaGrantPolicyByID: %+v err=%v", got, err)
	}
	if got, err := control.FindQuotaGrantPolicyByKey(product.Code, pkg.Code, item.Code); err != nil || got.ID != grantPolicy.ID {
		t.Fatalf("FindQuotaGrantPolicyByKey: %+v err=%v", got, err)
	}
	if got, err := control.FindActiveCommercialPackage(product.Code, pkg.Code); err != nil || got.ID != pkg.ID {
		t.Fatalf("FindActiveCommercialPackage: %+v err=%v", got, err)
	}
	grantPolicy.Units = 11
	if err := control.SaveQuotaGrantPolicy(grantPolicy); err != nil {
		t.Fatalf("SaveQuotaGrantPolicy: %v", err)
	}
	if policies, err := control.ListPackageCapabilityPolicies(product.Code, pkg.Code); err != nil || len(policies) != 1 {
		t.Fatalf("ListPackageCapabilityPolicies: %+v err=%v", policies, err)
	}
	if got, err := control.FindPackageCapabilityPolicyByID(capPolicy.ID); err != nil || got.ID != capPolicy.ID {
		t.Fatalf("FindPackageCapabilityPolicyByID: %+v err=%v", got, err)
	}
	if got, err := control.FindPackageCapabilityPolicyByKey(product.Code, pkg.Code, capPolicy.CapabilityCode); err != nil || got.ID != capPolicy.ID {
		t.Fatalf("FindPackageCapabilityPolicyByKey: %+v err=%v", got, err)
	}
	capPolicy.Status = "disabled"
	if err := control.SavePackageCapabilityPolicy(capPolicy); err != nil {
		t.Fatalf("SavePackageCapabilityPolicy: %v", err)
	}
	if grants, err := control.ListCapabilityGrants(product.Code, "organization", orgA.ID, capGrant.CapabilityCode); err != nil || len(grants) != 1 {
		t.Fatalf("ListCapabilityGrants: %+v err=%v", grants, err)
	}
	if got, err := control.FindCapabilityGrantBySource(product.Code, "organization", orgA.ID, capGrant.CapabilityCode, capGrant.SourceType, capGrant.SourceID); err != nil || got.ID != capGrant.ID {
		t.Fatalf("FindCapabilityGrantBySource: %+v err=%v", got, err)
	}
	capGrant.Status = "revoked"
	if err := control.SaveCapabilityGrant(capGrant); err != nil {
		t.Fatalf("SaveCapabilityGrant: %v", err)
	}
	if err := control.DeleteQuotaGrantPolicy("missing"); err != nil {
		t.Fatalf("DeleteQuotaGrantPolicy: %v", err)
	}
	if err := control.DeletePackageCapabilityPolicy("missing"); err != nil {
		t.Fatalf("DeletePackageCapabilityPolicy: %v", err)
	}
	if sum, err := control.SumReserved("quota", "organization", orgA.ID, ""); err != nil || sum != 0 {
		t.Fatalf("SumReserved without item: %d err=%v", sum, err)
	}

	fallbackBinding := &models.StorageBinding{ID: "storage-fallback", ProductCode: product.Code, Category: "*", ProviderCode: "local", LocalBaseDir: "data/default", Priority: 99, Enabled: true, CreatedAt: now, UpdatedAt: now}
	directBinding := &models.StorageBinding{ID: "storage-direct", ProductCode: product.Code, Category: "images", ProviderCode: "local", LocalBaseDir: "data/images", Priority: 1, Enabled: true, CreatedAt: now, UpdatedAt: now}
	providerA := &models.RuntimeProviderBinding{ID: "provider-a", ProductCode: product.Code, TaskType: "image_generation", ProviderCode: "provider_a", Priority: 20, Enabled: true, CreatedAt: now.Add(time.Second), UpdatedAt: now}
	providerB := &models.RuntimeProviderBinding{ID: "provider-b", ProductCode: product.Code, TaskType: "image_generation", ProviderCode: "provider_b", Priority: 10, Enabled: true, CreatedAt: now, UpdatedAt: now}
	providerDisabled := &models.RuntimeProviderBinding{ID: "provider-disabled", ProductCode: product.Code, TaskType: "image_generation", ProviderCode: "provider_disabled", Priority: 30, Enabled: false, CreatedAt: now, UpdatedAt: now}
	if err := runtimeRepo.CreateStorageBinding(fallbackBinding); err != nil {
		t.Fatalf("CreateStorageBinding fallback: %v", err)
	}
	if err := runtimeRepo.CreateStorageBinding(directBinding); err != nil {
		t.Fatalf("CreateStorageBinding direct: %v", err)
	}
	for _, b := range []*models.RuntimeProviderBinding{providerA, providerB, providerDisabled} {
		if err := runtimeRepo.CreateProviderBinding(b); err != nil {
			t.Fatalf("CreateProviderBinding %s: %v", b.ID, err)
		}
	}
	if got, err := runtimeRepo.FindProviderBinding(product.Code, "image_generation", providerA.ProviderCode); err != nil || got.ID != providerA.ID {
		t.Fatalf("FindProviderBinding: %+v err=%v", got, err)
	}
	if got, err := runtimeRepo.FindPreferredProviderBinding(product.Code, "image_generation"); err != nil || got.ID != providerB.ID {
		t.Fatalf("FindPreferredProviderBinding: %+v err=%v", got, err)
	}
	if items, err := runtimeRepo.ListAllProviderBindings(product.Code, "image_generation"); err != nil || len(items) != 3 {
		t.Fatalf("ListAllProviderBindings: %+v err=%v", items, err)
	}
	if got, err := runtimeRepo.FindPreferredStorageBinding(product.Code, "documents"); err != nil || got.ID != fallbackBinding.ID {
		t.Fatalf("FindPreferredStorageBinding fallback: %+v err=%v", got, err)
	}
	if bindings, err := runtimeRepo.ListStorageBindings(product.Code); err != nil || len(bindings) != 2 {
		t.Fatalf("ListStorageBindings: %+v err=%v", bindings, err)
	}
	asset := &models.StorageAsset{ID: "storage-asset-filter", ProductCode: product.Code, Category: "images", SourceType: "runtime_job", SourceRef: "job-filter", StorageKey: "images/job-filter.png", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := runtimeRepo.CreateStorageAsset(asset); err != nil {
		t.Fatalf("CreateStorageAsset: %v", err)
	}
	asset.Status = "archived"
	if err := runtimeRepo.UpdateStorageAsset(asset); err != nil {
		t.Fatalf("UpdateStorageAsset: %v", err)
	}
	if got, err := runtimeRepo.FindStorageAssetByStorageKey(asset.StorageKey); err != nil || got.Status != "archived" {
		t.Fatalf("FindStorageAssetByStorageKey: %+v err=%v", got, err)
	}
	idem := "idem-filter"
	job := &models.RuntimeJob{ID: "runtime-filter-job", ProductCode: product.Code, TaskType: "image_generation", ProviderCode: providerB.ProviderCode, ProviderMode: "sync", OrganizationID: orgA.ID, SourceType: "ecommerce_job", SourceID: "src-filter", IdempotencyKey: &idem, Status: "processing", Stage: "dispatch", CreatedAt: now, UpdatedAt: now}
	if err := runtimeRepo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	job.Stage = "poll"
	if err := runtimeRepo.SaveRuntimeJob(job); err != nil {
		t.Fatalf("SaveRuntimeJob: %v", err)
	}
	if got, err := runtimeRepo.FindRuntimeJobByIdempotencyKey(product.Code, orgA.ID, job.SourceType, job.SourceID, job.TaskType, idem); err != nil || got.ID != job.ID {
		t.Fatalf("FindRuntimeJobByIdempotencyKey: %+v err=%v", got, err)
	}
	if jobs, total, err := runtimeRepo.ListRuntimeJobs(RuntimeJobListFilter{OrganizationID: orgA.ID, Status: "processing", Stage: "poll", Query: "src-filter", Limit: 10}); err != nil || total != 1 || len(jobs) != 1 {
		t.Fatalf("ListRuntimeJobs filter: jobs=%+v total=%d err=%v", jobs, total, err)
	}
	delivery := &models.RuntimeCallbackDelivery{ID: "delivery-filter", RuntimeJobID: job.ID, ProductCode: product.Code, SourceID: job.SourceID, CallbackType: "result", Status: "pending", PayloadJSON: "{}", CreatedAt: now, UpdatedAt: now}
	if err := runtimeRepo.CreateCallbackDelivery(delivery); err != nil {
		t.Fatalf("CreateCallbackDelivery: %v", err)
	}
	delivery.Status = "delivered"
	if err := runtimeRepo.SaveCallbackDelivery(delivery); err != nil {
		t.Fatalf("SaveCallbackDelivery: %v", err)
	}
	if got, err := runtimeRepo.FindCallbackDeliveryByID(delivery.ID); err != nil || got.Status != "delivered" {
		t.Fatalf("FindCallbackDeliveryByID: %+v err=%v", got, err)
	}
	session := &models.ChargeSession{ID: "charge-filter", SourceType: job.SourceType, SourceID: job.SourceID, ProductCode: product.Code, OrganizationID: orgA.ID, BillingSubjectType: "organization", BillingSubjectID: orgA.ID, BillableItemCode: item.Code, ResourceType: "quota", Status: "reserved", ReservationKey: "reservation-filter", ReservationID: "reservation-id", EventID: "event-filter", SettlementID: "settlement-filter", FinalizationID: "final-filter", CreatedAt: now, UpdatedAt: now}
	if err := runtimeRepo.CreateChargeSession(session); err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	session.Status = "settled"
	if err := runtimeRepo.SaveChargeSession(session); err != nil {
		t.Fatalf("SaveChargeSession: %v", err)
	}
	if got, err := runtimeRepo.FindChargeSessionByID(session.ID); err != nil || got.Status != "settled" {
		t.Fatalf("FindChargeSessionByID: %+v err=%v", got, err)
	}
	if got, err := runtimeRepo.FindChargeSessionByReservationKey(session.ReservationKey); err != nil || got.ID != session.ID {
		t.Fatalf("FindChargeSessionByReservationKey: %+v err=%v", got, err)
	}
	if sessions, total, err := runtimeRepo.ListChargeSessions(ChargeSessionListFilter{OrganizationID: orgA.ID, Status: "settled", ProductCode: product.Code, Query: "reservation-filter", Limit: 5}); err != nil || total != 1 || len(sessions) != 1 {
		t.Fatalf("ListChargeSessions: sessions=%+v total=%d err=%v", sessions, total, err)
	}

	assetDef := &models.AssetDefinition{AssetCode: "FILTER_CREDIT", ProductCode: product.Code, AssetType: "reward_credit", LifecycleType: "expiring", Status: "active", CreatedAt: now, UpdatedAt: now}
	policy := &models.AllowancePolicy{ID: "allowance-filter", ProductCode: product.Code, BillingSubjectType: "organization", BillingSubjectID: orgA.ID, AssetCode: assetDef.AssetCode, Amount: 100, Status: "active", CreatedAt: now, UpdatedAt: now}
	account := &models.WalletAccount{ID: "wallet-filter", BillingSubjectType: "organization", BillingSubjectID: orgA.ID, AssetCode: assetDef.AssetCode, Balance: 100, Status: "active", CreatedAt: now, UpdatedAt: now}
	expires := now.Add(-time.Hour)
	bucket := &models.WalletBucket{ID: "bucket-filter", WalletAccountID: account.ID, AssetCode: assetDef.AssetCode, Balance: 50, Status: "active", ExpiresAt: &expires, CreatedAt: now, UpdatedAt: now}
	ledger := &models.WalletLedger{ID: "wallet-ledger-filter", WalletAccountID: account.ID, BillingSubjectType: "organization", BillingSubjectID: orgA.ID, AssetCode: assetDef.AssetCode, Direction: "credit", Amount: 50, ReferenceType: "grant", ReferenceID: "grant-filter", Status: "posted", CreatedAt: now}
	settlement := &models.SettlementRecord{ID: "settlement-filter", EventID: "event-filter", BillingSubjectType: "organization", BillingSubjectID: orgA.ID, ProductCode: product.Code, Status: "settled", CreatedAt: now, UpdatedAt: now}
	for name, fn := range map[string]func() error{
		"assetDef": func() error { return finance.CreateAssetDefinition(assetDef) }, "policy": func() error { return finance.CreateAllowancePolicy(policy) }, "account": func() error { return finance.CreateWalletAccount(account) }, "bucket": func() error { return finance.CreateWalletBucket(bucket) }, "ledger": func() error { return finance.CreateWalletLedger(ledger) }, "settlement": func() error { return finance.CreateSettlementRecord(settlement) },
	} {
		if err := fn(); err != nil {
			t.Fatalf("seed finance %s: %v", name, err)
		}
	}
	assetDef.Status = "disabled"
	if err := finance.SaveAssetDefinition(assetDef); err != nil {
		t.Fatalf("SaveAssetDefinition: %v", err)
	}
	if defs, err := finance.ListAssetDefinitions(product.Code, "expiring", "disabled"); err != nil || len(defs) != 1 {
		t.Fatalf("ListAssetDefinitions: %+v err=%v", defs, err)
	}
	if got, err := finance.FindAllowancePolicy(product.Code, "organization", orgA.ID, assetDef.AssetCode); err != nil || got.ID != policy.ID {
		t.Fatalf("FindAllowancePolicy: %+v err=%v", got, err)
	}
	if got, err := finance.FindAllowancePolicyByID(policy.ID); err != nil || got.ID != policy.ID {
		t.Fatalf("FindAllowancePolicyByID: %+v err=%v", got, err)
	}
	policy.Amount = 200
	if err := finance.SaveAllowancePolicy(policy); err != nil {
		t.Fatalf("SaveAllowancePolicy: %v", err)
	}
	if policies, err := finance.ListAllowancePolicies(product.Code, assetDef.AssetCode, "active"); err != nil || len(policies) != 1 {
		t.Fatalf("ListAllowancePolicies: %+v err=%v", policies, err)
	}
	account.Balance = 90
	if err := finance.SaveWalletAccount(account); err != nil {
		t.Fatalf("SaveWalletAccount: %v", err)
	}
	if got, err := finance.FindWalletAccountByID(account.ID); err != nil || got.Balance != 90 {
		t.Fatalf("FindWalletAccountByID: %+v err=%v", got, err)
	}
	if got, err := finance.FindWalletAccount("organization", orgA.ID, assetDef.AssetCode); err != nil || got.ID != account.ID {
		t.Fatalf("FindWalletAccount: %+v err=%v", got, err)
	}
	if got, err := finance.FindWalletBucketByCycle(account.ID, "missing-cycle"); err == nil || got != nil {
		t.Fatalf("expected missing cycle bucket, got=%+v err=%v", got, err)
	}
	bucket.Status = "expired"
	if err := finance.SaveWalletBucket(bucket); err != nil {
		t.Fatalf("SaveWalletBucket: %v", err)
	}
	if buckets, err := finance.ListWalletBuckets(account.ID, "expired"); err != nil || len(buckets) != 1 {
		t.Fatalf("ListWalletBuckets: %+v err=%v", buckets, err)
	}
	bucket.Status = "active"
	if err := finance.SaveWalletBucket(bucket); err != nil {
		t.Fatalf("SaveWalletBucket active: %v", err)
	}
	if buckets, err := finance.ListSpendableWalletBuckets(account.ID, now.Add(-2*time.Hour)); err != nil || len(buckets) != 1 {
		t.Fatalf("ListSpendableWalletBuckets: %+v err=%v", buckets, err)
	}
	if buckets, err := finance.ListExpirableWalletBuckets(now, assetDef.AssetCode); err != nil || len(buckets) != 1 {
		t.Fatalf("ListExpirableWalletBuckets: %+v err=%v", buckets, err)
	}
	if got, err := finance.FindWalletLedgerByReference("organization", orgA.ID, assetDef.AssetCode, "credit", "grant", "grant-filter"); err != nil || got.ID != ledger.ID {
		t.Fatalf("FindWalletLedgerByReference: %+v err=%v", got, err)
	}
	if ledgerRows, err := finance.ListWalletLedger(account.ID); err != nil || len(ledgerRows) != 1 {
		t.Fatalf("ListWalletLedger account: %+v err=%v", ledgerRows, err)
	}
	if got, err := finance.FindSettlementRecordByEventID(settlement.EventID); err != nil || got.ID != settlement.ID {
		t.Fatalf("FindSettlementRecordByEventID: %+v err=%v", got, err)
	}
	if err := finance.DeleteAssetDefinition("missing"); err != nil {
		t.Fatalf("DeleteAssetDefinition: %v", err)
	}
	if err := finance.DeleteAllowancePolicy("missing"); err != nil {
		t.Fatalf("DeleteAllowancePolicy: %v", err)
	}

	program := &models.ReferralProgram{ID: "program-filter", ProductCode: product.Code, ProgramCode: "program-filter", Name: "Program", TriggerType: "signup", CommissionPolicy: "fixed_amount", CommissionCurrency: assetDef.AssetCode, CommissionFixedAmount: 10, Status: "active", CreatedAt: now, UpdatedAt: now}
	refCode := &models.ReferralCode{ID: "code-filter", ProgramID: program.ID, ProductCode: product.Code, Code: "CODEFILTER", PromoterSubjectType: "organization", PromoterSubjectID: orgA.ID, Status: "active", CreatedAt: now, UpdatedAt: now}
	conversion := &models.ReferralConversion{ID: "conversion-filter", ProgramID: program.ID, ReferralCodeID: refCode.ID, ProductCode: product.Code, TriggerType: "signup", PromoterSubjectType: "organization", PromoterSubjectID: orgA.ID, ReferredSubjectType: "organization", ReferredSubjectID: orgB.ID, Status: "completed", ReferenceType: "order", ReferenceID: "order-filter", CreatedAt: now, UpdatedAt: now}
	if err := finance.CreateReferralProgram(program); err != nil {
		t.Fatalf("CreateReferralProgram: %v", err)
	}
	program.Status = "paused"
	if err := finance.SaveReferralProgram(program); err != nil {
		t.Fatalf("SaveReferralProgram: %v", err)
	}
	if err := finance.CreateReferralCode(refCode); err != nil {
		t.Fatalf("CreateReferralCode: %v", err)
	}
	if err := finance.CreateReferralConversion(conversion); err != nil {
		t.Fatalf("CreateReferralConversion: %v", err)
	}
	if got, err := finance.FindReferralConversionByTriggerAndSubject(product.Code, "signup", "organization", orgB.ID); err != nil || got.ID != conversion.ID {
		t.Fatalf("FindReferralConversionByTriggerAndSubject: %+v err=%v", got, err)
	}
}
