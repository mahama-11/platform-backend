package productbilling

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"platform-service/internal/models"
	controlmodule "platform-service/internal/modules/control"
	meteringmodule "platform-service/internal/modules/metering"
	runtimemodule "platform-service/internal/modules/runtime"
	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"
)

var (
	ErrActionNotFound       = errors.New("product billing action not found")
	ErrActionInvalidRequest = errors.New("invalid product billing action request")
)

type Service struct {
	commercialRepo *repository.CommercialRepository
	control        *controlmodule.Service
	metering       *meteringmodule.Service
	runtime        *runtimemodule.Service
	wallet         *walletmodule.Service
}

func NewService(commercialRepo *repository.CommercialRepository, control *controlmodule.Service, metering *meteringmodule.Service, runtime *runtimemodule.Service, wallet *walletmodule.Service) *Service {
	return &Service{commercialRepo: commercialRepo, control: control, metering: metering, runtime: runtime, wallet: wallet}
}

type CommercialViewInput struct {
	ProductCode        string
	BillingSubjectType string
	BillingSubjectID   string
}

type CommercialView struct {
	Product            *models.Product                  `json:"product,omitempty"`
	SKUs               []models.SKU                     `json:"skus"`
	Packages           []models.CommercialPackage       `json:"packages"`
	BillableItems      []models.BillableItem            `json:"billable_items"`
	RateCards          []models.RateCard                `json:"rate_cards"`
	QuotaBalances      []controlmodule.BalanceResult    `json:"quota_balances"`
	Wallet             *walletmodule.WalletSummary      `json:"wallet,omitempty"`
	Readiness          CommercialViewReadiness          `json:"readiness"`
	DeprecatedV1Notice ProductBillingDeprecatedV1Notice `json:"deprecated_v1_notice"`
}

type CommercialViewReadiness struct {
	CatalogComplete bool     `json:"catalog_complete"`
	Reasons         []string `json:"reasons,omitempty"`
}

type ProductBillingDeprecatedV1Notice struct {
	Message      string   `json:"message"`
	V2Endpoints  []string `json:"v2_endpoints"`
	SunsetPolicy string   `json:"sunset_policy"`
}

type BeginActionInput struct {
	ProductCode        string `json:"product_code" binding:"required"`
	OrganizationID     string `json:"organization_id" binding:"required"`
	UserID             string `json:"user_id"`
	SourceType         string `json:"source_type" binding:"required"`
	SourceID           string `json:"source_id" binding:"required"`
	SourceAction       string `json:"source_action"`
	TaskType           string `json:"task_type"`
	BillingSubjectType string `json:"billing_subject_type"`
	BillingSubjectID   string `json:"billing_subject_id"`
	BillableItemCode   string `json:"billable_item_code" binding:"required"`
	ResourceType       string `json:"resource_type"`
	EstimatedUnits     int64  `json:"estimated_units"`
	Unit               string `json:"unit"`
	IdempotencyKey     string `json:"idempotency_key" binding:"required"`
	RouteSnapshot      string `json:"route_snapshot"`
	Metadata           string `json:"metadata"`
}

type BindRuntimeInput struct {
	RuntimeJobID string `json:"runtime_job_id" binding:"required"`
}

type CompleteActionInput struct {
	RuntimeJobID   string `json:"runtime_job_id"`
	FinalUnits     int64  `json:"final_units"`
	FinalizationID string `json:"finalization_id"`
	EventID        string `json:"event_id"`
	SettlementID   string `json:"settlement_id"`
	OutputManifest string `json:"output_manifest"`
	StageMessage   string `json:"stage_message"`
	Dimensions     string `json:"dimensions"`
	OccurredAt     string `json:"occurred_at"`
	Metadata       string `json:"metadata"`
}

type ReleaseActionInput struct {
	Reason   string `json:"reason"`
	Metadata string `json:"metadata"`
}

type ReconcileActionInput struct {
	RepairMode string `json:"repair_mode"`
	Reason     string `json:"reason"`
}

type ProductBillingAction struct {
	ActionID           string                      `json:"action_id"`
	ProductCode        string                      `json:"product_code"`
	OrganizationID     string                      `json:"organization_id"`
	UserID             string                      `json:"user_id,omitempty"`
	SourceType         string                      `json:"source_type"`
	SourceID           string                      `json:"source_id"`
	BillingSubjectType string                      `json:"billing_subject_type"`
	BillingSubjectID   string                      `json:"billing_subject_id"`
	BillableItemCode   string                      `json:"billable_item_code"`
	ResourceType       string                      `json:"resource_type"`
	Status             string                      `json:"status"`
	EstimatedUnits     int64                       `json:"estimated_units"`
	FinalUnits         int64                       `json:"final_units,omitempty"`
	ChargeSessionID    string                      `json:"charge_session_id"`
	ReservationID      string                      `json:"reservation_id,omitempty"`
	RuntimeJobID       string                      `json:"runtime_job_id,omitempty"`
	FinalizationID     string                      `json:"finalization_id,omitempty"`
	EventID            string                      `json:"event_id,omitempty"`
	SettlementID       string                      `json:"settlement_id,omitempty"`
	Repair             ProductBillingRepairSummary `json:"repair"`
	Idempotent         bool                        `json:"idempotent"`
}

type ProductBillingRepairSummary struct {
	State    string   `json:"state"`
	Required bool     `json:"required"`
	Reasons  []string `json:"reasons,omitempty"`
}

func (s *Service) CommercialView(input CommercialViewInput) (*CommercialView, error) {
	if s == nil || s.commercialRepo == nil {
		return nil, errors.New("product billing service is not configured")
	}
	productCode := strings.TrimSpace(input.ProductCode)
	if productCode == "" {
		return nil, fmt.Errorf("%w: product_code is required", ErrActionInvalidRequest)
	}
	product, err := s.commercialRepo.FindProductByCode(productCode)
	if err != nil {
		return nil, err
	}
	skus, err := s.commercialRepo.ListSKUs(product.ID)
	if err != nil {
		return nil, err
	}
	packages, err := s.commercialRepo.ListPackages(product.ID)
	if err != nil {
		return nil, err
	}
	items, err := s.commercialRepo.ListBillableItems(product.ID)
	if err != nil {
		return nil, err
	}
	rates, err := s.commercialRepo.ListRateCards(product.ID, "")
	if err != nil {
		return nil, err
	}
	view := &CommercialView{
		Product:       product,
		SKUs:          skus,
		Packages:      packages,
		BillableItems: items,
		RateCards:     rates,
		QuotaBalances: []controlmodule.BalanceResult{},
		Readiness: CommercialViewReadiness{
			CatalogComplete: true,
		},
		DeprecatedV1Notice: deprecatedV1Notice(),
	}
	if len(skus) == 0 {
		view.Readiness.CatalogComplete = false
		view.Readiness.Reasons = append(view.Readiness.Reasons, "sku_missing")
	}
	if len(packages) == 0 {
		view.Readiness.CatalogComplete = false
		view.Readiness.Reasons = append(view.Readiness.Reasons, "package_missing")
	}
	if len(items) == 0 {
		view.Readiness.CatalogComplete = false
		view.Readiness.Reasons = append(view.Readiness.Reasons, "billable_item_missing")
	}
	if len(rates) == 0 {
		view.Readiness.CatalogComplete = false
		view.Readiness.Reasons = append(view.Readiness.Reasons, "rate_card_missing")
	}
	subjectType := defaultString(strings.TrimSpace(input.BillingSubjectType), platformconst.SubjectTypeOrganization)
	subjectID := strings.TrimSpace(input.BillingSubjectID)
	if subjectID != "" && s.control != nil {
		for _, item := range items {
			balance, balanceErr := s.control.QuotaBalance(subjectType, subjectID, item.Code)
			if balanceErr == nil && balance != nil {
				view.QuotaBalances = append(view.QuotaBalances, *balance)
			}
		}
	}
	if subjectID != "" && s.wallet != nil {
		wallet, walletErr := s.wallet.GetWalletSummary(subjectType, subjectID, productCode, time.Now())
		if walletErr == nil {
			view.Wallet = wallet
		}
	}
	return view, nil
}

func (s *Service) ActivatePackage(input controlmodule.ActivatePackageInput) (*controlmodule.PackageActivationResult, error) {
	if s == nil || s.control == nil {
		return nil, errors.New("control service is not configured")
	}
	return s.control.ActivatePackage(input)
}

func (s *Service) BeginAction(input BeginActionInput) (*ProductBillingAction, error) {
	if s == nil || s.runtime == nil || s.control == nil {
		return nil, errors.New("product billing dependencies are not configured")
	}
	if err := validateBeginAction(input); err != nil {
		return nil, err
	}
	input = normalizeBeginAction(input)
	reservationKey := actionReservationKey(input)
	if existing, existingErr := s.runtime.GetChargeSessionByReservationKey(reservationKey); existingErr == nil && existing != nil {
		return s.actionFromCharge(existing, true), nil
	}
	charge, err := s.runtime.CreateChargeSession(runtimemodule.CreateChargeSessionInput{
		SourceType:         input.SourceType,
		SourceID:           input.SourceID,
		ProductCode:        input.ProductCode,
		OrganizationID:     input.OrganizationID,
		UserID:             input.UserID,
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		BillableItemCode:   input.BillableItemCode,
		ResourceType:       input.ResourceType,
		ReservationKey:     reservationKey,
		EstimatedUnits:     input.EstimatedUnits,
		RouteSnapshot:      input.RouteSnapshot,
		Metadata:           mergeMetadata(input.Metadata, map[string]any{"gateway_version": "v2", "task_type": input.TaskType, "source_action": input.SourceAction}),
	})
	if err != nil {
		return nil, err
	}
	idempotent := charge.Status != platformconst.StatusCreated || charge.ReservationID != ""
	if charge.ReservationID != "" || isActionTerminal(charge.Status) {
		return s.actionFromCharge(charge, idempotent), nil
	}
	reservation, err := s.control.Reserve(controlmodule.ReserveInput{
		ResourceType:       input.ResourceType,
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		BillableItemCode:   input.BillableItemCode,
		ReservationKey:     "reserve:" + charge.ID,
		Units:              input.EstimatedUnits,
		ReferenceID:        charge.ID,
		Metadata:           mergeMetadata(input.Metadata, map[string]any{"gateway_version": "v2", "charge_session_id": charge.ID}),
	})
	if err != nil {
		_, _ = s.runtime.UpdateChargeSession(charge.ID, runtimemodule.UpdateChargeSessionInput{Status: platformconst.StatusFailed, Metadata: mergeMetadata(input.Metadata, map[string]any{"gateway_version": "v2", "reserve_error": err.Error()})})
		return nil, err
	}
	charge, err = s.runtime.UpdateChargeSession(charge.ID, runtimemodule.UpdateChargeSessionInput{Status: platformconst.ReservationStatusReserved, ReservationID: reservation.ID})
	if err != nil {
		_, _ = s.control.ReleaseReservation(reservation.ID)
		return nil, err
	}
	return s.actionFromCharge(charge, idempotent), nil
}

func (s *Service) BindRuntime(actionID string, input BindRuntimeInput) (*ProductBillingAction, error) {
	if s == nil || s.runtime == nil {
		return nil, errors.New("runtime service is not configured")
	}
	charge, err := s.runtime.GetChargeSession(strings.TrimSpace(actionID))
	if err != nil {
		return nil, ErrActionNotFound
	}
	job, _, err := s.runtime.BindChargeSessionToRuntimeJob(charge.ID, strings.TrimSpace(input.RuntimeJobID))
	if err != nil {
		return nil, err
	}
	charge, _ = s.runtime.GetChargeSession(charge.ID)
	action := s.actionFromCharge(charge, true)
	if job != nil {
		action.RuntimeJobID = job.ID
	}
	return action, nil
}

func (s *Service) CompleteAction(actionID string, input CompleteActionInput) (*ProductBillingAction, error) {
	if s == nil || s.runtime == nil {
		return nil, errors.New("runtime service is not configured")
	}
	charge, err := s.runtime.GetChargeSession(strings.TrimSpace(actionID))
	if err != nil {
		return nil, ErrActionNotFound
	}
	if charge.Status == platformconst.SettlementStatusSettled {
		return s.actionFromCharge(charge, true), nil
	}
	if strings.TrimSpace(input.RuntimeJobID) != "" {
		var finalUnits *int64
		if input.FinalUnits > 0 {
			value := input.FinalUnits
			finalUnits = &value
		}
		job, charge, bindErr := s.runtime.BindChargeSessionToRuntimeJobWithChargeUpdate(charge.ID, input.RuntimeJobID, runtimemodule.UpdateChargeSessionInput{
			FinalUnits:     finalUnits,
			FinalizationID: input.FinalizationID,
			EventID:        input.EventID,
			SettlementID:   input.SettlementID,
			Metadata:       mergeMetadata(input.Metadata, map[string]any{"gateway_version": "v2", "runtime_job_id": input.RuntimeJobID}),
		})
		if bindErr != nil {
			return nil, bindErr
		}
		if job != nil && isActionTerminal(job.Status) {
			charge, _ = s.runtime.GetChargeSession(charge.ID)
			return s.actionFromCharge(charge, true), nil
		}
		_, updateErr := s.runtime.UpdateRuntimeJob(job.ID, runtimemodule.UpdateRuntimeJobInput{Status: platformconst.StatusCompleted, Stage: platformconst.StatusCompleted, StageMessage: defaultString(input.StageMessage, "Product billing action completed"), OutputManifest: input.OutputManifest, Metadata: input.Metadata})
		if updateErr != nil {
			return nil, updateErr
		}
		charge, _ = s.runtime.GetChargeSession(charge.ID)
		return s.actionFromCharge(charge, true), nil
	}
	if s.metering == nil {
		return nil, errors.New("metering service is not configured")
	}
	if strings.TrimSpace(charge.ReservationID) == "" {
		return nil, fmt.Errorf("%w: reservation_id is required before complete", ErrActionInvalidRequest)
	}
	finalUnits := input.FinalUnits
	if finalUnits <= 0 {
		finalUnits = charge.EstimatedUnits
	}
	if finalUnits <= 0 {
		finalUnits = 1
	}
	finalizationID := defaultString(input.FinalizationID, "pb:v2:finalization:"+charge.ID)
	eventID := defaultString(input.EventID, "pb:v2:event:"+charge.ID)
	billable := true
	finalized, err := s.metering.Finalize(meteringmodule.FinalizeInput{
		FinalizationID: finalizationID,
		ReservationID:  charge.ReservationID,
		IngestEventInput: meteringmodule.IngestEventInput{
			EventID:            eventID,
			SourceType:         charge.SourceType,
			SourceID:           charge.SourceID,
			SourceAction:       "product_billing_action_complete",
			ProductCode:        charge.ProductCode,
			OrgID:              charge.OrganizationID,
			UserID:             charge.UserID,
			BillableItemCode:   charge.BillableItemCode,
			ChargeGroupID:      charge.ID,
			BillingSubjectType: charge.BillingSubjectType,
			BillingSubjectID:   charge.BillingSubjectID,
			UsageUnits:         finalUnits,
			Unit:               defaultString(inputUnitFromCharge(charge), platformconst.MeterUnitRequest),
			Billable:           &billable,
			Dimensions:         input.Dimensions,
			OccurredAt:         input.OccurredAt,
		},
	})
	if err != nil {
		return nil, err
	}
	settlementID := strings.TrimSpace(input.SettlementID)
	if finalized != nil && finalized.Settlement != nil {
		settlementID = finalized.Settlement.ID
	}
	charge, err = s.runtime.UpdateChargeSession(charge.ID, runtimemodule.UpdateChargeSessionInput{Status: platformconst.SettlementStatusSettled, FinalizationID: finalizationID, EventID: eventID, SettlementID: settlementID, FinalUnits: &finalUnits, Metadata: mergeMetadata(input.Metadata, map[string]any{"gateway_version": "v2", "completed_via": "product_billing_gateway"})})
	if err != nil {
		return nil, err
	}
	return s.actionFromCharge(charge, true), nil
}

func (s *Service) ReleaseAction(actionID string, input ReleaseActionInput) (*ProductBillingAction, error) {
	if s == nil || s.runtime == nil {
		return nil, errors.New("runtime service is not configured")
	}
	charge, err := s.runtime.GetChargeSession(strings.TrimSpace(actionID))
	if err != nil {
		return nil, ErrActionNotFound
	}
	if charge.Status == platformconst.SettlementStatusSettled || charge.Status == platformconst.ReservationStatusReleased || charge.Status == platformconst.StatusCanceled || charge.Status == platformconst.StatusFailed {
		return s.actionFromCharge(charge, true), nil
	}
	if strings.TrimSpace(charge.ReservationID) != "" && s.control != nil {
		if _, releaseErr := s.control.ReleaseReservation(charge.ReservationID); releaseErr != nil {
			return nil, releaseErr
		}
	}
	charge, err = s.runtime.UpdateChargeSession(charge.ID, runtimemodule.UpdateChargeSessionInput{Status: platformconst.ReservationStatusReleased, Metadata: mergeMetadata(input.Metadata, map[string]any{"gateway_version": "v2", "release_reason": input.Reason})})
	if err != nil {
		return nil, err
	}
	return s.actionFromCharge(charge, true), nil
}

func (s *Service) ReconcileAction(actionID string, _ ReconcileActionInput) (*ProductBillingAction, error) {
	if s == nil || s.runtime == nil {
		return nil, errors.New("runtime service is not configured")
	}
	charge, err := s.runtime.GetChargeSession(strings.TrimSpace(actionID))
	if err != nil {
		return nil, ErrActionNotFound
	}
	return s.actionFromCharge(charge, true), nil
}

func (s *Service) actionFromCharge(charge *models.ChargeSession, idempotent bool) *ProductBillingAction {
	if charge == nil {
		return nil
	}
	action := &ProductBillingAction{
		ActionID:           charge.ID,
		ProductCode:        charge.ProductCode,
		OrganizationID:     charge.OrganizationID,
		UserID:             charge.UserID,
		SourceType:         charge.SourceType,
		SourceID:           charge.SourceID,
		BillingSubjectType: charge.BillingSubjectType,
		BillingSubjectID:   charge.BillingSubjectID,
		BillableItemCode:   charge.BillableItemCode,
		ResourceType:       charge.ResourceType,
		Status:             charge.Status,
		EstimatedUnits:     charge.EstimatedUnits,
		FinalUnits:         charge.FinalUnits,
		ChargeSessionID:    charge.ID,
		ReservationID:      charge.ReservationID,
		FinalizationID:     charge.FinalizationID,
		EventID:            charge.EventID,
		SettlementID:       charge.SettlementID,
		Idempotent:         idempotent,
	}
	metadata := decodeMap(charge.Metadata)
	if runtimeJobID, _ := metadata["runtime_job_id"].(string); runtimeJobID != "" {
		action.RuntimeJobID = runtimeJobID
	}
	action.Repair = repairSummary(charge)
	return action
}

func repairSummary(charge *models.ChargeSession) ProductBillingRepairSummary {
	if charge == nil {
		return ProductBillingRepairSummary{State: "missing", Required: true, Reasons: []string{"action_missing"}}
	}
	summary := ProductBillingRepairSummary{State: "ok"}
	switch charge.Status {
	case platformconst.StatusCreated:
		summary.State = "reservation_missing"
		summary.Required = true
		summary.Reasons = append(summary.Reasons, "begin_action_did_not_reserve")
	case platformconst.ReservationStatusReserved:
		summary.State = "pending_terminal_action"
		summary.Required = true
		summary.Reasons = append(summary.Reasons, "complete_or_release_required")
	case platformconst.SettlementStatusSettled, platformconst.ReservationStatusReleased, platformconst.StatusCanceled, platformconst.StatusFailed:
		summary.State = "terminal"
	}
	return summary
}

func validateBeginAction(input BeginActionInput) error {
	missing := []string{}
	if strings.TrimSpace(input.ProductCode) == "" {
		missing = append(missing, "product_code")
	}
	if strings.TrimSpace(input.OrganizationID) == "" {
		missing = append(missing, "organization_id")
	}
	if strings.TrimSpace(input.SourceType) == "" {
		missing = append(missing, "source_type")
	}
	if strings.TrimSpace(input.SourceID) == "" {
		missing = append(missing, "source_id")
	}
	if strings.TrimSpace(input.BillableItemCode) == "" {
		missing = append(missing, "billable_item_code")
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		missing = append(missing, "idempotency_key")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %s", ErrActionInvalidRequest, strings.Join(missing, ","))
	}
	return nil
}

func normalizeBeginAction(input BeginActionInput) BeginActionInput {
	input.ProductCode = strings.TrimSpace(input.ProductCode)
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.SourceAction = strings.TrimSpace(input.SourceAction)
	input.TaskType = strings.TrimSpace(input.TaskType)
	input.BillableItemCode = strings.TrimSpace(input.BillableItemCode)
	input.BillingSubjectType = defaultString(strings.TrimSpace(input.BillingSubjectType), platformconst.SubjectTypeOrganization)
	input.BillingSubjectID = defaultString(strings.TrimSpace(input.BillingSubjectID), input.OrganizationID)
	input.ResourceType = defaultString(strings.TrimSpace(input.ResourceType), platformconst.ResourceTypeQuota)
	input.Unit = defaultString(strings.TrimSpace(input.Unit), platformconst.MeterUnitRequest)
	if input.EstimatedUnits <= 0 {
		input.EstimatedUnits = 1
	}
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	return input
}

func actionReservationKey(input BeginActionInput) string {
	parts := []string{"pb", "v2", input.ProductCode, input.OrganizationID, input.SourceType, input.SourceID, input.BillableItemCode, input.IdempotencyKey}
	return strings.Join(parts, ":")
}

func isActionTerminal(status string) bool {
	switch status {
	case platformconst.SettlementStatusSettled, platformconst.ReservationStatusReleased, platformconst.StatusCanceled, platformconst.StatusFailed:
		return true
	default:
		return false
	}
}

func deprecatedV1Notice() ProductBillingDeprecatedV1Notice {
	return ProductBillingDeprecatedV1Notice{
		Message: "Product billing v1 primitive APIs remain available for compatibility, but product integrations should migrate to /internal/v2/product-billing.",
		V2Endpoints: []string{
			"GET /internal/v2/product-billing/commercial-view",
			"POST /internal/v2/product-billing/package-activations",
			"POST /internal/v2/product-billing/actions/begin",
			"POST /internal/v2/product-billing/actions/:actionID/bind-runtime",
			"POST /internal/v2/product-billing/actions/:actionID/complete",
			"POST /internal/v2/product-billing/actions/:actionID/release",
			"POST /internal/v2/product-billing/actions/:actionID/reconcile",
		},
		SunsetPolicy: "v1 primitive billing APIs will be phased out for product backends after each product has a passing v2 consumer smoke.",
	}
}

func inputUnitFromCharge(_ *models.ChargeSession) string { return platformconst.MeterUnitRequest }

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func decodeMap(raw string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func mergeMetadata(raw string, extra map[string]any) string {
	out := decodeMap(raw)
	for k, v := range extra {
		if str, ok := v.(string); ok && strings.TrimSpace(str) == "" {
			continue
		}
		out[k] = v
	}
	body, _ := json.Marshal(out)
	return string(body)
}
