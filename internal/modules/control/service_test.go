package control

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platform-service/internal/models"
	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newControlTestService(t *testing.T) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("control-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.QuotaLedger{}, &models.ResourceReservation{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.AutoMigrate(&models.AssetDefinition{}, &models.WalletAccount{}, &models.WalletBucket{}, &models.WalletLedger{}); err != nil {
		t.Fatalf("auto migrate wallet: %v", err)
	}
	walletService := walletmodule.NewService(repository.NewFinanceRepository(db))
	return NewService(repository.NewControlRepository(db), walletService)
}

func TestControlServiceQuotaAndCreditsFlow(t *testing.T) {
	service := newControlTestService(t)
	if _, err := service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		Units:              10,
	}); err != nil {
		t.Fatalf("GrantQuota: %v", err)
	}
	if _, err := service.GrantCredits(GrantCreditsInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		Amount:             20,
	}); err != nil {
		t.Fatalf("GrantCredits: %v", err)
	}
	quotaBalance, err := service.QuotaBalance("organization", "org-1", "IMAGE_GENERATION")
	if err != nil || quotaBalance.Available != 10 {
		t.Fatalf("QuotaBalance: %+v err=%v", quotaBalance, err)
	}
	creditsBalance, err := service.CreditsBalance("organization", "org-1")
	if err != nil || creditsBalance.Available != 20 {
		t.Fatalf("CreditsBalance: %+v err=%v", creditsBalance, err)
	}
}

func TestControlServiceReserveCommitReleaseAndIdempotency(t *testing.T) {
	service := newControlTestService(t)
	_, _ = service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		Units:              10,
	})
	reservation, err := service.Reserve(ReserveInput{
		ResourceType:       "quota",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		ReservationKey:     "reserve-1",
		Units:              3,
		ReferenceID:        "job-1",
	})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	duplicate, err := service.Reserve(ReserveInput{
		ResourceType:       "quota",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		ReservationKey:     "reserve-1",
		Units:              3,
	})
	if err != nil || duplicate.ID != reservation.ID {
		t.Fatalf("Reserve duplicate: %+v err=%v", duplicate, err)
	}
	committed, err := service.CommitReservation(reservation.ID)
	if err != nil || committed.Status != "committed" || committed.CommittedAt == nil {
		t.Fatalf("CommitReservation: %+v err=%v", committed, err)
	}
	if _, err := service.CommitReservation(reservation.ID); err == nil {
		t.Fatalf("expected commit invalid status error")
	}

	_, _ = service.GrantCredits(GrantCreditsInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		Amount:             5,
	})
	released, err := service.Reserve(ReserveInput{
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		Units:              2,
	})
	if err != nil {
		t.Fatalf("Reserve credits: %v", err)
	}
	released, err = service.ReleaseReservation(released.ID)
	if err != nil || released.Status != "released" || released.ReleasedAt == nil {
		t.Fatalf("ReleaseReservation: %+v err=%v", released, err)
	}
}

func TestControlServiceErrorBranchesAndOptionalString(t *testing.T) {
	service := newControlTestService(t)
	if _, err := service.Reserve(ReserveInput{
		ResourceType:       "quota",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-no-quota",
		BillableItemCode:   "IMAGE_GENERATION",
		Units:              1,
	}); err != ErrInsufficientQuota {
		t.Fatalf("expected ErrInsufficientQuota, got %v", err)
	}
	if _, err := service.Reserve(ReserveInput{
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-no-credits",
		Units:              1,
	}); err != ErrInsufficientCredits {
		t.Fatalf("expected ErrInsufficientCredits, got %v", err)
	}
	if _, err := service.Reserve(ReserveInput{
		ResourceType:       "unknown",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		Units:              1,
	}); err != ErrReservationInvalid {
		t.Fatalf("expected ErrReservationInvalid, got %v", err)
	}
	if optionalString(" ") != nil {
		t.Fatalf("expected nil optionalString")
	}
	value := optionalString("abc")
	if value == nil || *value != "abc" {
		t.Fatalf("unexpected optionalString result: %v", value)
	}
	if _, err := service.ReleaseReservation("missing"); err == nil {
		t.Fatalf("expected release missing error")
	}
}

func newControlTestServiceWithPolicies(t *testing.T) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("control-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.CommercialPackage{}, &models.QuotaLedger{}, &models.ResourceReservation{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.AutoMigrate(&models.AssetDefinition{}, &models.WalletAccount{}, &models.WalletBucket{}, &models.WalletLedger{}); err != nil {
		t.Fatalf("auto migrate wallet: %v", err)
	}
	if err := db.AutoMigrate(&models.QuotaGrantPolicy{}, &models.PackageCapabilityPolicy{}, &models.CapabilityGrant{}); err != nil {
		t.Fatalf("auto migrate policies: %v", err)
	}
	walletService := walletmodule.NewService(repository.NewFinanceRepository(db))
	return NewService(repository.NewControlRepository(db), walletService)
}

func seedControlPackage(t *testing.T, service *Service, productCode, packageCode, status string) {
	t.Helper()
	now := timeNowUTC()
	product := models.Product{ID: "prod_" + productCode, Code: productCode, Name: productCode, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := service.repo.DB().Where("code = ?", productCode).FirstOrCreate(&product).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	pkg := models.CommercialPackage{ID: "pkg_" + packageCode, ProductID: product.ID, Code: packageCode, Name: packageCode, PackageType: "trial", Status: status, CreatedAt: now, UpdatedAt: now}
	if err := service.repo.DB().Create(&pkg).Error; err != nil {
		t.Fatalf("seed package: %v", err)
	}
}

func timeNowUTC() time.Time { return time.Now().UTC() }

func TestActivatePackageAppliesPoliciesAndIsIdempotent(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)
	seedControlPackage(t, service, "menu", "menu.pkg.trial.signup", "active")
	if _, err := service.CreateQuotaGrantPolicy(CreateQuotaGrantPolicyInput{ProductCode: "menu", PackageCode: "menu.pkg.trial.signup", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 5}); err != nil {
		t.Fatalf("CreateQuotaGrantPolicy: %v", err)
	}
	if _, err := service.CreatePackageCapabilityPolicy(CreatePackageCapabilityPolicyInput{ProductCode: "menu", PackageCode: "menu.pkg.trial.signup", CapabilityCode: "template_scope", GrantValue: "free_templates"}); err != nil {
		t.Fatalf("CreatePackageCapabilityPolicy: %v", err)
	}

	longReferenceID := "menu:signup_package:user-" + strings.Repeat("x", 72) + ":org-signup"
	input := ActivatePackageInput{
		ProductCode: "menu", PackageCode: "menu.pkg.trial.signup",
		BillingSubjectType: "organization", BillingSubjectID: "org-signup",
		ActivationReason: "signup_trial", ReferenceID: longReferenceID,
		Metadata: []byte(`{"user_id":"user-1","source":"menu_signup"}`),
	}
	result, err := service.ActivatePackage(input)
	if err != nil {
		t.Fatalf("ActivatePackage: %v", err)
	}
	if result.PackageCode != input.PackageCode || result.GrantedQuotaUnits != 5 || len(result.QuotaGrants) != 1 || len(result.CapabilityGrants) != 1 {
		t.Fatalf("unexpected activation result: %+v", result)
	}
	if result.CapabilityGrants[0].SourceID != longReferenceID || len(result.CapabilityGrants[0].SourceID) <= 64 {
		t.Fatalf("capability activation should preserve long reference source id: %+v", result.CapabilityGrants[0])
	}
	balance, err := service.QuotaBalance("organization", "org-signup", "menu.render.call")
	if err != nil || balance.Available != 5 {
		t.Fatalf("QuotaBalance after activation: %+v err=%v", balance, err)
	}
	capability, err := service.ResolveCapability("menu", "organization", "org-signup", "template_scope")
	if err != nil || capability.GrantValue != "free_templates" || capability.Grant.Metadata != `{"user_id":"user-1","source":"menu_signup"}` {
		t.Fatalf("ResolveCapability after activation: %+v err=%v", capability, err)
	}

	duplicate, err := service.ActivatePackage(input)
	if err != nil {
		t.Fatalf("ActivatePackage duplicate: %v", err)
	}
	if !duplicate.Idempotent || duplicate.GrantedQuotaUnits != 5 || duplicate.QuotaGrants[0].ID != result.QuotaGrants[0].ID || duplicate.CapabilityGrants[0].ID != result.CapabilityGrants[0].ID {
		t.Fatalf("duplicate activation was not idempotent: first=%+v duplicate=%+v", result, duplicate)
	}
	balance, _ = service.QuotaBalance("organization", "org-signup", "menu.render.call")
	if balance.Available != 5 {
		t.Fatalf("duplicate activation changed quota balance: %+v", balance)
	}
}

func TestActivatePackageFailsClosedWithoutActivePolicies(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)
	seedControlPackage(t, service, "menu", "menu.pkg.no.policy", "active")
	if _, err := service.ActivatePackage(ActivatePackageInput{ProductCode: "menu", PackageCode: "menu.pkg.no.policy", BillingSubjectType: "organization", BillingSubjectID: "org-1", ReferenceID: "ref-1"}); err == nil {
		t.Fatalf("expected activation to fail without active policies")
	}
	seedControlPackage(t, service, "menu", "menu.pkg.disabled.policy", "active")
	if _, err := service.CreateQuotaGrantPolicy(CreateQuotaGrantPolicyInput{ProductCode: "menu", PackageCode: "menu.pkg.disabled.policy", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 5, Status: "disabled"}); err != nil {
		t.Fatalf("Create disabled quota policy: %v", err)
	}
	if _, err := service.ActivatePackage(ActivatePackageInput{ProductCode: "menu", PackageCode: "menu.pkg.disabled.policy", BillingSubjectType: "organization", BillingSubjectID: "org-1", ReferenceID: "ref-2"}); err == nil {
		t.Fatalf("expected activation to fail when all policies are disabled")
	}
}

func TestQuotaGrantPolicyCRUD(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)

	// Create: success
	created, err := service.CreateQuotaGrantPolicy(CreateQuotaGrantPolicyInput{
		ProductCode:      "product-a",
		PackageCode:      "pkg-1",
		BillableItemCode: "IMAGE_GENERATION",
		GrantMode:        "on_activation",
		Units:            100,
		ResetCycle:       "monthly",
	})
	if err != nil {
		t.Fatalf("CreateQuotaGrantPolicy: %v", err)
	}
	if created.ID == "" || created.ProductCode != "product-a" || created.Units != 100 || created.Status != "active" {
		t.Fatalf("unexpected created policy: %+v", created)
	}

	// Create duplicate: error "already exists"
	_, err = service.CreateQuotaGrantPolicy(CreateQuotaGrantPolicyInput{
		ProductCode:      "product-a",
		PackageCode:      "pkg-1",
		BillableItemCode: "IMAGE_GENERATION",
		GrantMode:        "on_activation",
		Units:            200,
	})
	if err == nil || err.Error() != "quota grant policy already exists" {
		t.Fatalf("expected 'already exists' error, got: %v", err)
	}

	// Create second policy for different package
	_, err = service.CreateQuotaGrantPolicy(CreateQuotaGrantPolicyInput{
		ProductCode:      "product-a",
		PackageCode:      "pkg-2",
		BillableItemCode: "IMAGE_GENERATION",
		GrantMode:        "on_activation",
		Units:            50,
	})
	if err != nil {
		t.Fatalf("CreateQuotaGrantPolicy second: %v", err)
	}

	// List: filter by productCode/packageCode
	list, err := service.ListQuotaGrantPolicies("product-a", "pkg-1")
	if err != nil {
		t.Fatalf("ListQuotaGrantPolicies: %v", err)
	}
	if len(list) != 1 || list[0].PackageCode != "pkg-1" {
		t.Fatalf("expected 1 policy for pkg-1, got %d", len(list))
	}

	listAll, err := service.ListQuotaGrantPolicies("product-a", "")
	if err != nil {
		t.Fatalf("ListQuotaGrantPolicies all: %v", err)
	}
	if len(listAll) != 2 {
		t.Fatalf("expected 2 policies for product-a, got %d", len(listAll))
	}

	// Update: partial fields
	updated, err := service.UpdateQuotaGrantPolicy(created.ID, UpdateQuotaGrantPolicyInput{
		Units:     200,
		GrantMode: "on_renewal",
	})
	if err != nil {
		t.Fatalf("UpdateQuotaGrantPolicy: %v", err)
	}
	if updated.Units != 200 || updated.GrantMode != "on_renewal" || updated.ProductCode != "product-a" {
		t.Fatalf("unexpected updated policy: %+v", updated)
	}

	// Update: not found
	_, err = service.UpdateQuotaGrantPolicy("non-existent-id", UpdateQuotaGrantPolicyInput{
		Units: 300,
	})
	if err == nil {
		t.Fatalf("expected error updating non-existent policy")
	}

	// Delete: success
	err = service.DeleteQuotaGrantPolicy(created.ID)
	if err != nil {
		t.Fatalf("DeleteQuotaGrantPolicy: %v", err)
	}
	listAfterDelete, err := service.ListQuotaGrantPolicies("product-a", "pkg-1")
	if err != nil {
		t.Fatalf("ListQuotaGrantPolicies after delete: %v", err)
	}
	if len(listAfterDelete) != 0 {
		t.Fatalf("expected 0 policies after delete, got %d", len(listAfterDelete))
	}
}

func TestPackageCapabilityPolicyCRUD(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)

	// Create: success
	created, err := service.CreatePackageCapabilityPolicy(CreatePackageCapabilityPolicyInput{
		ProductCode:    "product-b",
		PackageCode:    "pkg-1",
		CapabilityCode: "WATERMARK_REMOVAL",
		GrantValue:     "enabled",
	})
	if err != nil {
		t.Fatalf("CreatePackageCapabilityPolicy: %v", err)
	}
	if created.ID == "" || created.CapabilityCode != "WATERMARK_REMOVAL" || created.Status != "active" {
		t.Fatalf("unexpected created capability policy: %+v", created)
	}

	// Create duplicate: error "already exists"
	_, err = service.CreatePackageCapabilityPolicy(CreatePackageCapabilityPolicyInput{
		ProductCode:    "product-b",
		PackageCode:    "pkg-1",
		CapabilityCode: "WATERMARK_REMOVAL",
		GrantValue:     "disabled",
	})
	if err == nil || err.Error() != "package capability policy already exists" {
		t.Fatalf("expected 'already exists' error, got: %v", err)
	}

	// Create second policy
	_, err = service.CreatePackageCapabilityPolicy(CreatePackageCapabilityPolicyInput{
		ProductCode:    "product-b",
		PackageCode:    "pkg-2",
		CapabilityCode: "WATERMARK_REMOVAL",
		GrantValue:     "enabled",
	})
	if err != nil {
		t.Fatalf("CreatePackageCapabilityPolicy second: %v", err)
	}

	// List
	list, err := service.ListPackageCapabilityPolicies("product-b", "pkg-1")
	if err != nil {
		t.Fatalf("ListPackageCapabilityPolicies: %v", err)
	}
	if len(list) != 1 || list[0].PackageCode != "pkg-1" {
		t.Fatalf("expected 1 capability policy for pkg-1, got %d", len(list))
	}

	listAll, err := service.ListPackageCapabilityPolicies("product-b", "")
	if err != nil {
		t.Fatalf("ListPackageCapabilityPolicies all: %v", err)
	}
	if len(listAll) != 2 {
		t.Fatalf("expected 2 capability policies for product-b, got %d", len(listAll))
	}

	// Update: partial fields
	updated, err := service.UpdatePackageCapabilityPolicy(created.ID, UpdatePackageCapabilityPolicyInput{
		GrantValue: "premium",
		Status:     "disabled",
	})
	if err != nil {
		t.Fatalf("UpdatePackageCapabilityPolicy: %v", err)
	}
	if updated.GrantValue != "premium" || updated.Status != "disabled" {
		t.Fatalf("unexpected updated capability policy: %+v", updated)
	}

	// Update: not found
	_, err = service.UpdatePackageCapabilityPolicy("non-existent-id", UpdatePackageCapabilityPolicyInput{
		GrantValue: "x",
	})
	if err == nil {
		t.Fatalf("expected error updating non-existent capability policy")
	}

	// Delete
	err = service.DeletePackageCapabilityPolicy(created.ID)
	if err != nil {
		t.Fatalf("DeletePackageCapabilityPolicy: %v", err)
	}
	listAfterDelete, err := service.ListPackageCapabilityPolicies("product-b", "pkg-1")
	if err != nil {
		t.Fatalf("ListPackageCapabilityPolicies after delete: %v", err)
	}
	if len(listAfterDelete) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(listAfterDelete))
	}
}

func TestGrantAndResolveCapability(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)

	// GrantCapability: normal create
	grant, err := service.GrantCapability(GrantCapabilityInput{
		ProductCode:        "product-c",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		CapabilityCode:     "WATERMARK_REMOVAL",
		GrantValue:         "enabled",
		SourceType:         "package_activation",
		SourceID:           "activation-1",
	})
	if err != nil {
		t.Fatalf("GrantCapability: %v", err)
	}
	if grant.ID == "" || grant.GrantValue != "enabled" || grant.Status != "active" {
		t.Fatalf("unexpected grant: %+v", grant)
	}

	// GrantCapability: idempotent via sourceType+sourceID
	dup, err := service.GrantCapability(GrantCapabilityInput{
		ProductCode:        "product-c",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		CapabilityCode:     "WATERMARK_REMOVAL",
		GrantValue:         "different-value",
		SourceType:         "package_activation",
		SourceID:           "activation-1",
	})
	if err != nil {
		t.Fatalf("GrantCapability idempotent: %v", err)
	}
	if dup.ID != grant.ID {
		t.Fatalf("expected idempotent return same grant, got different ID: %s vs %s", dup.ID, grant.ID)
	}

	// GrantCapability: no sourceType skips idempotency check, creates new
	grantNoSource, err := service.GrantCapability(GrantCapabilityInput{
		ProductCode:        "product-c",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		CapabilityCode:     "WATERMARK_REMOVAL",
		GrantValue:         "enabled",
	})
	if err != nil {
		t.Fatalf("GrantCapability no source: %v", err)
	}
	if grantNoSource.ID == grant.ID {
		t.Fatalf("expected new grant when no sourceType, got same ID")
	}

	// ResolveCapability: has grants, returns first
	result, err := service.ResolveCapability("product-c", "organization", "org-1", "WATERMARK_REMOVAL")
	if err != nil {
		t.Fatalf("ResolveCapability: %v", err)
	}
	if result.GrantValue == "" || result.Grant == nil {
		t.Fatalf("expected resolved capability with grant, got: %+v", result)
	}

	// ResolveCapability: no grants, returns empty
	resultEmpty, err := service.ResolveCapability("product-c", "organization", "org-999", "WATERMARK_REMOVAL")
	if err != nil {
		t.Fatalf("ResolveCapability empty: %v", err)
	}
	if resultEmpty.GrantValue != "" || resultEmpty.Grant != nil {
		t.Fatalf("expected empty resolve, got: %+v", resultEmpty)
	}
}

func TestGrantQuotaIdempotency(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)

	// Grant with referenceID
	first, err := service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-idem",
		BillableItemCode:   "IMAGE_GENERATION",
		Units:              10,
		ReferenceID:        "ref-unique-1",
	})
	if err != nil {
		t.Fatalf("GrantQuota first: %v", err)
	}

	// Second call with same referenceID returns same ledger
	second, err := service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-idem",
		BillableItemCode:   "IMAGE_GENERATION",
		Units:              10,
		ReferenceID:        "ref-unique-1",
	})
	if err != nil {
		t.Fatalf("GrantQuota idempotent: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected same ledger ID on idempotent grant: %s vs %s", second.ID, first.ID)
	}

	// Verify balance only counted once
	balance, err := service.QuotaBalance("organization", "org-idem", "IMAGE_GENERATION")
	if err != nil {
		t.Fatalf("QuotaBalance: %v", err)
	}
	if balance.Available != 10 {
		t.Fatalf("expected available=10 (idempotent), got %d", balance.Available)
	}
}

func TestCommitReservationCreditsPath(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)

	// Grant credits
	_, err := service.GrantCredits(GrantCreditsInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-credits-commit",
		Amount:             50,
	})
	if err != nil {
		t.Fatalf("GrantCredits: %v", err)
	}

	// Reserve credits
	reservation, err := service.Reserve(ReserveInput{
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-credits-commit",
		Units:              10,
		ReferenceID:        "job-credits-1",
	})
	if err != nil {
		t.Fatalf("Reserve credits: %v", err)
	}

	// Commit reservation (credits type)
	committed, err := service.CommitReservation(reservation.ID)
	if err != nil {
		t.Fatalf("CommitReservation credits: %v", err)
	}
	if committed.Status != "committed" || committed.CommittedAt == nil {
		t.Fatalf("unexpected committed state: %+v", committed)
	}

	// Verify wallet was debited: credits balance should drop
	balance, err := service.CreditsBalance("organization", "org-credits-commit")
	if err != nil {
		t.Fatalf("CreditsBalance after commit: %v", err)
	}
	// Granted 50, committed(debited) 10, no more reservations => available = 40
	if balance.Available != 40 {
		t.Fatalf("expected available=40 after credits commit, got %d", balance.Available)
	}
}

func TestCreditsBalanceNegativeClamp(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)

	// Grant small credits
	_, err := service.GrantCredits(GrantCreditsInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-clamp",
		Amount:             5,
	})
	if err != nil {
		t.Fatalf("GrantCredits: %v", err)
	}

	// Reserve more than the credits balance (reserve the full 5 first, then reserve again)
	_, err = service.Reserve(ReserveInput{
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-clamp",
		ReservationKey:     "res-clamp-1",
		Units:              3,
	})
	if err != nil {
		t.Fatalf("Reserve 3: %v", err)
	}

	_, err = service.Reserve(ReserveInput{
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-clamp",
		ReservationKey:     "res-clamp-2",
		Units:              2,
	})
	if err != nil {
		t.Fatalf("Reserve 2: %v", err)
	}

	// Now balance = 5 total, 5 reserved => available = 0
	balance, err := service.CreditsBalance("organization", "org-clamp")
	if err != nil {
		t.Fatalf("CreditsBalance: %v", err)
	}
	if balance.Available != 0 {
		t.Fatalf("expected available=0, got %d", balance.Available)
	}
}
