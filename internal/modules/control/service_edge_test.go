package control

import (
	"errors"
	"strings"
	"testing"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
)

func TestQuotaBalanceAccountsRefundsAndOpenReservationsOnly(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)

	if _, err := service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-quota-ledger",
		BillableItemCode:   "menu.render.call",
		Units:              10,
		ReferenceID:        "grant-1",
	}); err != nil {
		t.Fatalf("GrantQuota: %v", err)
	}
	reservation, err := service.Reserve(ReserveInput{
		ResourceType:       platformconst.ResourceTypeQuota,
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-quota-ledger",
		BillableItemCode:   "menu.render.call",
		ReservationKey:     "quota-balance-reservation",
		Units:              3,
		ReferenceID:        "job-1",
	})
	if err != nil {
		t.Fatalf("Reserve quota: %v", err)
	}
	if err := service.repo.CreateQuotaLedger(&models.QuotaLedger{
		ID:                 "consume-ledger-1",
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-quota-ledger",
		BillableItemCode:   "menu.render.call",
		Direction:          platformconst.LedgerDirectionConsume,
		Units:              4,
		Reason:             "runtime_success",
		ReferenceID:        "job-2",
	}); err != nil {
		t.Fatalf("seed consume ledger: %v", err)
	}
	if err := service.repo.CreateQuotaLedger(&models.QuotaLedger{
		ID:                 "refund-ledger-1",
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-quota-ledger",
		BillableItemCode:   "menu.render.call",
		Direction:          platformconst.LedgerDirectionRefund,
		Units:              2,
		Reason:             "runtime_refund",
		ReferenceID:        "job-2-refund",
	}); err != nil {
		t.Fatalf("seed refund ledger: %v", err)
	}

	balance, err := service.QuotaBalance(platformconst.SubjectTypeOrganization, "org-quota-ledger", "menu.render.call")
	if err != nil {
		t.Fatalf("QuotaBalance: %v", err)
	}
	if balance.Granted != 10 || balance.Consumed != 4 || balance.Refunded != 2 || balance.Reserved != 3 || balance.Available != 5 {
		t.Fatalf("unexpected quota balance while reservation is open: %+v", balance)
	}

	if _, err := service.ReleaseReservation(reservation.ID); err != nil {
		t.Fatalf("ReleaseReservation: %v", err)
	}
	balance, err = service.QuotaBalance(platformconst.SubjectTypeOrganization, "org-quota-ledger", "menu.render.call")
	if err != nil {
		t.Fatalf("QuotaBalance after release: %v", err)
	}
	if balance.Reserved != 0 || balance.Available != 8 {
		t.Fatalf("released reservation should no longer reduce availability: %+v", balance)
	}
}

func TestReserveQuotaDefaultsNonPositiveUnitsToOneAndPersistsMetadata(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)
	if _, err := service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-default-units",
		BillableItemCode:   "image.render",
		Units:              2,
	}); err != nil {
		t.Fatalf("GrantQuota: %v", err)
	}

	reservation, err := service.Reserve(ReserveInput{
		ResourceType:       platformconst.ResourceTypeQuota,
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-default-units",
		BillableItemCode:   "image.render",
		Units:              0,
		ReferenceID:        "job-default-units",
		Metadata:           `{"source":"unit-default"}`,
	})
	if err != nil {
		t.Fatalf("Reserve default units: %v", err)
	}
	if reservation.Units != 1 || reservation.ReservationKey != nil || reservation.Metadata != `{"source":"unit-default"}` {
		t.Fatalf("unexpected reservation defaults: %+v", reservation)
	}

	balance, err := service.QuotaBalance(platformconst.SubjectTypeOrganization, "org-default-units", "image.render")
	if err != nil {
		t.Fatalf("QuotaBalance: %v", err)
	}
	if balance.Reserved != 1 || balance.Available != 1 {
		t.Fatalf("defaulted one-unit reservation should consume one unit: %+v", balance)
	}
}

func TestUpdateQuotaGrantPolicyTrimsFieldsPreservesUnitsOnZeroAndClearsOptionalFields(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)
	created, err := service.CreateQuotaGrantPolicy(CreateQuotaGrantPolicyInput{
		ProductCode:      " product-trim ",
		PackageCode:      " pkg-trim ",
		BillableItemCode: " image.render ",
		GrantMode:        " one_time ",
		Units:            25,
		ResetCycle:       " monthly ",
		Status:           " active ",
		Metadata:         ` {"tier":"trial"} `,
	})
	if err != nil {
		t.Fatalf("CreateQuotaGrantPolicy: %v", err)
	}
	if created.ProductCode != "product-trim" || created.PackageCode != "pkg-trim" || created.BillableItemCode != "image.render" || created.GrantMode != "one_time" || created.ResetCycle != "monthly" || created.Status != platformconst.StatusActive || created.Metadata != `{"tier":"trial"}` {
		t.Fatalf("create should trim persisted policy fields: %+v", created)
	}

	updated, err := service.UpdateQuotaGrantPolicy(created.ID, UpdateQuotaGrantPolicyInput{
		ProductCode:      " product-new ",
		PackageCode:      " pkg-new ",
		BillableItemCode: " image.hd.render ",
		GrantMode:        " renewal ",
		Units:            0,
		ResetCycle:       "   ",
		Status:           " disabled ",
		Metadata:         "   ",
	})
	if err != nil {
		t.Fatalf("UpdateQuotaGrantPolicy: %v", err)
	}
	if updated.ProductCode != "product-new" || updated.PackageCode != "pkg-new" || updated.BillableItemCode != "image.hd.render" || updated.GrantMode != "renewal" || updated.Status != "disabled" {
		t.Fatalf("update should trim replacement fields: %+v", updated)
	}
	if updated.Units != 25 {
		t.Fatalf("zero-unit update must not erase grant amount, got %d", updated.Units)
	}
	if updated.ResetCycle != "" || updated.Metadata != "" {
		t.Fatalf("optional reset cycle and metadata should be clearable: %+v", updated)
	}
}

func TestActivatePackageFiltersInactiveAndMalformedPoliciesAndUsesPolicyMetadataFallback(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)
	seedControlPackage(t, service, "menu", "menu.pkg.policy.filter", platformconst.StatusActive)

	policies := []CreateQuotaGrantPolicyInput{
		{ProductCode: "menu", PackageCode: "menu.pkg.policy.filter", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 10, Metadata: `{"valid":true}`},
		{ProductCode: "menu", PackageCode: "menu.pkg.policy.filter", BillableItemCode: "menu.disabled.call", GrantMode: "one_time", Units: 99, Status: "disabled"},
		{ProductCode: "menu", PackageCode: "menu.pkg.policy.filter", BillableItemCode: "menu.zero.call", GrantMode: "one_time", Units: 0},
		{ProductCode: "menu", PackageCode: "menu.pkg.policy.filter", BillableItemCode: "   ", GrantMode: "one_time", Units: 50},
	}
	for _, input := range policies {
		if _, err := service.CreateQuotaGrantPolicy(input); err != nil {
			t.Fatalf("CreateQuotaGrantPolicy(%q): %v", input.BillableItemCode, err)
		}
	}
	capabilityPolicies := []CreatePackageCapabilityPolicyInput{
		{ProductCode: "menu", PackageCode: "menu.pkg.policy.filter", CapabilityCode: "template_scope", GrantValue: "pro_templates", Metadata: `{"policy":"capability"}`},
		{ProductCode: "menu", PackageCode: "menu.pkg.policy.filter", CapabilityCode: "watermark", GrantValue: "enabled", Status: "disabled"},
		{ProductCode: "menu", PackageCode: "menu.pkg.policy.filter", CapabilityCode: "blank_value", GrantValue: "   "},
		{ProductCode: "menu", PackageCode: "menu.pkg.policy.filter", CapabilityCode: "   ", GrantValue: "ignored"},
	}
	for _, input := range capabilityPolicies {
		if _, err := service.CreatePackageCapabilityPolicy(input); err != nil {
			t.Fatalf("CreatePackageCapabilityPolicy(%q): %v", input.CapabilityCode, err)
		}
	}

	activation, err := service.ActivatePackage(ActivatePackageInput{
		ProductCode:        " menu ",
		PackageCode:        " menu.pkg.policy.filter ",
		BillingSubjectType: " organization ",
		BillingSubjectID:   " org-policy-filter ",
		ReferenceID:        " activation-policy-filter ",
	})
	if err != nil {
		t.Fatalf("ActivatePackage: %v", err)
	}
	if activation.Idempotent || activation.ActivationReason != "package_activation" {
		t.Fatalf("first activation should not be idempotent and should use default reason: %+v", activation)
	}
	if activation.GrantedQuotaUnits != 10 || len(activation.QuotaGrants) != 1 || activation.QuotaGrants[0].BillableItemCode != "menu.render.call" {
		t.Fatalf("activation should grant only active positive quota policies: %+v", activation)
	}
	if len(activation.CapabilityGrants) != 1 || activation.CapabilityGrants[0].CapabilityCode != "template_scope" || activation.CapabilityGrants[0].GrantValue != "pro_templates" {
		t.Fatalf("activation should grant only active complete capability policies: %+v", activation)
	}
	if activation.CapabilityGrants[0].Metadata != `{"policy":"capability"}` {
		t.Fatalf("capability grant should fall back to policy metadata when activation metadata is empty: %+v", activation.CapabilityGrants[0])
	}
}

func TestActivatePackageRejectsInactiveCommercialPackageWithoutPartialQuotaGrant(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)
	seedControlPackage(t, service, "menu", "menu.pkg.inactive", "disabled")
	if _, err := service.CreateQuotaGrantPolicy(CreateQuotaGrantPolicyInput{
		ProductCode:      "menu",
		PackageCode:      "menu.pkg.inactive",
		BillableItemCode: "menu.render.call",
		GrantMode:        "one_time",
		Units:            5,
	}); err != nil {
		t.Fatalf("CreateQuotaGrantPolicy: %v", err)
	}

	_, err := service.ActivatePackage(ActivatePackageInput{
		ProductCode:        "menu",
		PackageCode:        "menu.pkg.inactive",
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-inactive-package",
		ReferenceID:        "activation-inactive-package",
	})
	if err == nil || !strings.Contains(err.Error(), "active package not found") {
		t.Fatalf("expected inactive package failure, got %v", err)
	}
	balance, balanceErr := service.QuotaBalance(platformconst.SubjectTypeOrganization, "org-inactive-package", "menu.render.call")
	if balanceErr != nil {
		t.Fatalf("QuotaBalance: %v", balanceErr)
	}
	if balance.Available != 0 || balance.Granted != 0 {
		t.Fatalf("failed activation must not leave quota grants: %+v", balance)
	}
}

func TestCreditsPathsRequireWalletServiceAndKeepReservationReservedOnCommitFailure(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)
	if _, err := service.GrantCredits(GrantCreditsInput{
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-wallet-required",
		Amount:             5,
	}); err != nil {
		t.Fatalf("GrantCredits: %v", err)
	}
	reservation, err := service.Reserve(ReserveInput{
		ResourceType:       platformconst.ResourceTypeCredits,
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-wallet-required",
		Units:              2,
		ReservationKey:     "wallet-required-reservation",
	})
	if err != nil {
		t.Fatalf("Reserve credits: %v", err)
	}

	service.wallet = nil
	if _, err := service.GrantCredits(GrantCreditsInput{BillingSubjectType: platformconst.SubjectTypeOrganization, BillingSubjectID: "org-wallet-required", Amount: 1}); err == nil || !strings.Contains(err.Error(), "wallet service is required") {
		t.Fatalf("expected GrantCredits wallet-required error, got %v", err)
	}
	if _, err := service.CreditsBalance(platformconst.SubjectTypeOrganization, "org-wallet-required"); err == nil || !strings.Contains(err.Error(), "wallet service is required") {
		t.Fatalf("expected CreditsBalance wallet-required error, got %v", err)
	}
	if _, err := service.CommitReservation(reservation.ID); err == nil || !strings.Contains(err.Error(), "wallet service is required") {
		t.Fatalf("expected CommitReservation wallet-required error, got %v", err)
	}

	stored, err := service.repo.FindReservationByID(reservation.ID)
	if err != nil {
		t.Fatalf("FindReservationByID: %v", err)
	}
	if stored.Status != platformconst.ReservationStatusReserved || stored.CommittedAt != nil {
		t.Fatalf("failed credits commit must keep reservation reserved: %+v", stored)
	}
}

func TestReserveQuotaRejectsOverReservationAfterExistingHold(t *testing.T) {
	service := newControlTestServiceWithPolicies(t)
	if _, err := service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-over-reserve",
		BillableItemCode:   "image.render",
		Units:              3,
	}); err != nil {
		t.Fatalf("GrantQuota: %v", err)
	}
	if _, err := service.Reserve(ReserveInput{
		ResourceType:       platformconst.ResourceTypeQuota,
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-over-reserve",
		BillableItemCode:   "image.render",
		ReservationKey:     "hold-2-units",
		Units:              2,
	}); err != nil {
		t.Fatalf("Reserve first hold: %v", err)
	}
	if _, err := service.Reserve(ReserveInput{
		ResourceType:       platformconst.ResourceTypeQuota,
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-over-reserve",
		BillableItemCode:   "image.render",
		ReservationKey:     "hold-too-many",
		Units:              2,
	}); !errors.Is(err, ErrInsufficientQuota) {
		t.Fatalf("expected insufficient quota after existing hold, got %v", err)
	}
}
