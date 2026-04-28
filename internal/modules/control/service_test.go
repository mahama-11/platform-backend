package control

import (
	"fmt"
	"path/filepath"
	"testing"

	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/internal/models"
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
