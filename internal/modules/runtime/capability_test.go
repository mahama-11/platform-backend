package runtime

import (
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
)

func seedRuntimeCapabilityBase(t *testing.T, service *Service) {
	t.Helper()
	now := time.Now()
	if err := service.repo.DB().Create(&models.Product{
		ID:        "prod_ecommerce",
		Code:      "ecommerce",
		Name:      "Ecommerce",
		Status:    platformconst.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if err := service.repo.CreateProductEndpoint(&models.RuntimeProductEndpoint{
		ID:           "endpoint_ecommerce",
		ProductCode:  "ecommerce",
		CallbackKind: "ecommerce_internal",
		BaseURL:      "http://ecommerce-backend.internal",
		Secret:       "secret",
		Status:       platformconst.StatusActive,
		Metadata:     `{"output_storage_category":"ecommerce-assets"}`,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("seed endpoint: %v", err)
	}
	if err := service.repo.CreateStorageBinding(&models.StorageBinding{
		ID:           "storage_ecommerce_assets",
		ProductCode:  "ecommerce",
		Category:     "ecommerce-assets",
		ProviderCode: "local",
		Priority:     10,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("seed storage binding: %v", err)
	}
}

func seedRuntimeCapabilityBillableItem(t *testing.T, service *Service, code string) {
	t.Helper()
	now := time.Now()
	if err := service.repo.DB().Create(&models.BillableItem{
		ID:              "billable_" + code,
		ProductID:       "prod_ecommerce",
		Code:            code,
		Name:            code,
		MeterUnit:       "action",
		BillingScope:    "organization",
		SettlementMode:  "credits",
		PricingBehavior: "standard",
		Status:          platformconst.StatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed billable item: %v", err)
	}
}

func TestListRuntimeCapabilitiesImageGenerationAvailable(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(&fakeProvider{name: "mock"})
	service.UseRuntime(&fakeQueue{}, registry)
	seedRuntimeCapabilityBase(t, service)
	seedRuntimeCapabilityBillableItem(t, service, "ecommerce_runtime_image_generation")
	now := time.Now()
	if err := repo.CreateProviderDefinition(&models.RuntimeProviderDefinition{
		ID:           "provider_mock",
		Code:         "mock",
		Name:         "Mock",
		ProviderType: "image",
		Mode:         "sync",
		Status:       platformconst.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("seed provider definition: %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{
		ID:           "binding_mock_generation",
		ProductCode:  "ecommerce",
		TaskType:     RuntimeTaskImageGeneration,
		ProviderCode: "mock",
		Priority:     10,
		Enabled:      true,
		Metadata:     `{"objective_scores":{"quality":90}}`,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("seed provider binding: %v", err)
	}

	result, err := service.ListRuntimeCapabilities("ecommerce", RuntimeTaskImageGeneration)
	if err != nil {
		t.Fatalf("ListRuntimeCapabilities: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len=%d", len(result.Items))
	}
	item := result.Items[0]
	if !item.Available || item.Status != RuntimeCapabilityStatusAvailable || item.UnavailableReason != "" {
		t.Fatalf("expected available item, got %+v", item)
	}
	if !item.Callback.Configured || item.Callback.CallbackKind != "ecommerce_internal" {
		t.Fatalf("unexpected callback capability: %+v", item.Callback)
	}
	if !item.Storage.BindingConfigured || item.Storage.OutputCategory != "ecommerce-assets" {
		t.Fatalf("unexpected storage capability: %+v", item.Storage)
	}
	if !item.Billing.Configured || item.Billing.BillableItemCode != "ecommerce_runtime_image_generation" || item.Billing.MeterUnit != "action" || item.Billing.SettlementMode != "credits" {
		t.Fatalf("unexpected billing capability: %+v", item.Billing)
	}
	if len(item.ProviderBindings) != 1 || !item.ProviderBindings[0].Registered || item.ProviderBindings[0].Status != platformconst.StatusActive {
		t.Fatalf("unexpected provider capability: %+v", item.ProviderBindings)
	}
	if len(item.Reasons) != 0 {
		t.Fatalf("expected no unavailable reasons, got %+v", item.Reasons)
	}
}

func TestListRuntimeCapabilitiesDraftTaskContractNeededFirst(t *testing.T) {
	service, _, _ := newRuntimeServiceForTest(t)
	seedRuntimeCapabilityBase(t, service)

	result, err := service.ListRuntimeCapabilities("ecommerce", RuntimeTaskImageInpainting)
	if err != nil {
		t.Fatalf("ListRuntimeCapabilities: %v", err)
	}
	item := result.Items[0]
	if item.Available || item.Status != RuntimeCapabilityStatusUnavailable {
		t.Fatalf("expected unavailable item, got %+v", item)
	}
	if item.UnavailableReason != RuntimeCapabilityReasonContractNeeded {
		t.Fatalf("expected contract-needed first reason, got %q reasons=%+v", item.UnavailableReason, item.Reasons)
	}
	if !item.hasReason(RuntimeCapabilityReasonProviderBindingMissing) || !item.hasReason(RuntimeCapabilityReasonBillableItemMissing) {
		t.Fatalf("expected missing provider and billable reasons, got %+v", item.Reasons)
	}
}

func TestListRuntimeCapabilitiesUnknownTaskType(t *testing.T) {
	service, _, _ := newRuntimeServiceForTest(t)
	_, err := service.ListRuntimeCapabilities("ecommerce", "sku_deconstruction")
	if err == nil {
		t.Fatal("expected unknown task error")
	}
}
