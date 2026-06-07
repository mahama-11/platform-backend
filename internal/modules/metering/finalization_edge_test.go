package metering

import (
	"errors"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

func TestFinalize_FillsReservationDefaultsBeforeIngesting(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "final-defaults")
	createTestBillableItem(t, db, productID, "final.defaults", platformconst.SettlementModeUsageBilling)
	reservation := &models.ResourceReservation{
		ID:                 "resv-defaults",
		ResourceType:       "quota",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-final-defaults",
		BillableItemCode:   "final.defaults",
		Units:              6,
		Status:             platformconst.ReservationStatusReserved,
		ReferenceID:        "job-defaults",
		Metadata:           `{"job":"defaults"}`,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(reservation).Error; err != nil {
		t.Fatalf("create reservation: %v", err)
	}

	result, err := service.Finalize(FinalizeInput{
		FinalizationID: "fin-defaults",
		ReservationID:  reservation.ID,
		IngestEventInput: IngestEventInput{
			EventID:     "evt-final-defaults",
			ProductCode: "final-defaults",
			OrgID:       "org-final-defaults",
			// Billing subject, billable item and usage units intentionally omitted.
		},
	})
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if result.Reservation == nil || result.Reservation.Status != platformconst.ReservationStatusFinalized || result.Reservation.FinalizationID == nil || *result.Reservation.FinalizationID != "fin-defaults" || result.Reservation.CommittedAt == nil {
		t.Fatalf("unexpected reservation result: %+v", result.Reservation)
	}
	if result.Event == nil {
		t.Fatalf("Finalize() event = nil")
	}
	if result.Event.BillingSubjectType != reservation.BillingSubjectType || result.Event.BillingSubjectID != reservation.BillingSubjectID {
		t.Fatalf("event billing subject = %s/%s, want reservation subject %s/%s", result.Event.BillingSubjectType, result.Event.BillingSubjectID, reservation.BillingSubjectType, reservation.BillingSubjectID)
	}
	if result.Event.BillableItemCode != reservation.BillableItemCode || result.Event.UsageUnits != reservation.Units {
		t.Fatalf("event item/units = %s/%d, want %s/%d", result.Event.BillableItemCode, result.Event.UsageUnits, reservation.BillableItemCode, reservation.Units)
	}

	settlement := loadSettlementByEvent(t, db, "evt-final-defaults")
	if settlement.SettlementMode != platformconst.SettlementModeUsageBilling || settlement.BillingSubjectID != reservation.BillingSubjectID || settlement.BillableItemCode != reservation.BillableItemCode || settlement.GrossAmount != 0 || settlement.BillingAmount != 0 {
		t.Fatalf("unexpected settlement from finalized reservation: %+v", settlement)
	}
	if result.Settlement == nil || result.Settlement.EventID != "evt-final-defaults" {
		t.Fatalf("FinalizeResult settlement = %+v, want event settlement", result.Settlement)
	}
}

func TestFinalize_RequiresEventIDBeforeTouchingReservation(t *testing.T) {
	service, db := newTestService(t)
	reservation := &models.ResourceReservation{
		ID:                 "resv-missing-event",
		ResourceType:       "quota",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-missing-event",
		BillableItemCode:   "missing.event",
		Units:              1,
		Status:             platformconst.ReservationStatusReserved,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(reservation).Error; err != nil {
		t.Fatalf("create reservation: %v", err)
	}

	_, err := service.Finalize(FinalizeInput{FinalizationID: "fin-missing-event", ReservationID: reservation.ID})
	if err == nil || err.Error() != "event_id is required" {
		t.Fatalf("Finalize() error = %v, want missing event_id", err)
	}

	var stored models.ResourceReservation
	if err := db.Where("id = ?", reservation.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload reservation: %v", err)
	}
	if stored.Status != platformconst.ReservationStatusReserved || stored.FinalizationID != nil || stored.CommittedAt != nil {
		t.Fatalf("reservation changed after invalid finalize: %+v", stored)
	}
}

func TestFinalize_MissingReservationReturnsNotFoundWithoutCreatingEvent(t *testing.T) {
	service, db := newTestService(t)

	_, err := service.Finalize(FinalizeInput{
		FinalizationID: "fin-missing-reservation",
		ReservationID:  "resv-does-not-exist",
		IngestEventInput: IngestEventInput{
			EventID:          "evt-missing-reservation",
			ProductCode:      "missing-reservation",
			OrgID:            "org-missing-reservation",
			BillableItemCode: "missing.item",
		},
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Finalize() error = %v, want gorm.ErrRecordNotFound", err)
	}

	var eventCount int64
	if err := db.Model(&models.MeterEvent{}).Where("event_id = ?", "evt-missing-reservation").Count(&eventCount).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("events created = %d, want 0", eventCount)
	}
}

func TestFinalize_IdempotentRetryDoesNotReplaySettlementSideEffects(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "final-idem")
	billableItem := createTestBillableItem(t, db, productID, "final.idem", platformconst.SettlementModeUsageBilling)
	createTestRateCard(t, db, productID, billableItem.ID, "IDEM_CREDIT", 20)
	createTestWallet(t, db, "organization", "org-final-idem", "IDEM_CREDIT", 100)
	reservation := &models.ResourceReservation{
		ID:                 "resv-idem-side-effects",
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-final-idem",
		BillableItemCode:   "final.idem",
		Units:              3,
		Status:             platformconst.ReservationStatusReserved,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(reservation).Error; err != nil {
		t.Fatalf("create reservation: %v", err)
	}
	input := FinalizeInput{
		FinalizationID: "fin-idem-side-effects",
		ReservationID:  reservation.ID,
		IngestEventInput: IngestEventInput{
			EventID:            "evt-final-idem-side-effects",
			ProductCode:        "final-idem",
			OrgID:              "org-final-idem",
			BillableItemCode:   "final.idem",
			BillingSubjectType: "organization",
			BillingSubjectID:   "org-final-idem",
			UsageUnits:         3,
			CurrencyContext:    "IDEM_CREDIT",
		},
	}

	first, err := service.Finalize(input)
	if err != nil {
		t.Fatalf("Finalize() first error = %v", err)
	}
	second, err := service.Finalize(input)
	if err != nil {
		t.Fatalf("Finalize() second error = %v", err)
	}
	if first.Event == nil || second.Event == nil || first.Event.ID != second.Event.ID {
		t.Fatalf("idempotent retry returned different events: first=%+v second=%+v", first.Event, second.Event)
	}
	if second.Settlement == nil || second.Settlement.EventID != input.EventID {
		t.Fatalf("idempotent retry settlement = %+v", second.Settlement)
	}

	var eventCount int64
	if err := db.Model(&models.MeterEvent{}).Where("event_id = ?", input.EventID).Count(&eventCount).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("events count = %d, want exactly one", eventCount)
	}
	var settlementCount int64
	if err := db.Model(&models.SettlementRecord{}).Where("event_id = ?", input.EventID).Count(&settlementCount).Error; err != nil {
		t.Fatalf("count settlements: %v", err)
	}
	if settlementCount != 1 {
		t.Fatalf("settlements count = %d, want exactly one", settlementCount)
	}
	var walletLedgerCount int64
	if err := db.Model(&models.WalletLedger{}).Where("reference_id = ?", input.EventID).Count(&walletLedgerCount).Error; err != nil {
		t.Fatalf("count wallet ledgers: %v", err)
	}
	if walletLedgerCount != 1 {
		t.Fatalf("wallet ledgers count = %d, want exactly one debit", walletLedgerCount)
	}

	var wallet models.WalletAccount
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-final-idem", "IDEM_CREDIT").First(&wallet).Error; err != nil {
		t.Fatalf("load wallet: %v", err)
	}
	if wallet.Balance != 40 {
		t.Fatalf("wallet balance = %d, want one 60-credit debit only", wallet.Balance)
	}
}
