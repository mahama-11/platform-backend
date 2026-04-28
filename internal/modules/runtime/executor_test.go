package runtime

import (
	"testing"

	"platform-service/internal/models"
	"platform-service/internal/repository"
)

func TestOutputStorageCategoryUsesEndpointMetadata(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	repo := repository.NewRuntimeRepository(db)
	service := NewService(repo, runtimeConfigForTest(), runtimeSecurityForTest(), runtimeComfyForTest())

	if err := repo.CreateProductEndpoint(&models.RuntimeProductEndpoint{
		ID:           "ep-menu",
		ProductCode:  "menu",
		CallbackKind: "menu_internal",
		BaseURL:      "http://menu",
		Secret:       "secret",
		Status:       "active",
		Metadata:     `{"output_storage_category":"studio-assets"}`,
	}); err != nil {
		t.Fatalf("CreateProductEndpoint(menu) error = %v", err)
	}
	if err := repo.CreateProductEndpoint(&models.RuntimeProductEndpoint{
		ID:           "ep-ecommerce",
		ProductCode:  "ecommerce",
		CallbackKind: "ecommerce_internal",
		BaseURL:      "http://ecommerce",
		Secret:       "secret",
		Status:       "active",
		Metadata:     `{"output_storage_category":"ecommerce-assets"}`,
	}); err != nil {
		t.Fatalf("CreateProductEndpoint(ecommerce) error = %v", err)
	}

	menuCategory := service.outputStorageCategory(&models.RuntimeJob{ProductCode: "menu"})
	if menuCategory != "studio-assets" {
		t.Fatalf("menu output category = %s, want studio-assets", menuCategory)
	}

	ecommerceCategory := service.outputStorageCategory(&models.RuntimeJob{ProductCode: "ecommerce"})
	if ecommerceCategory != "ecommerce-assets" {
		t.Fatalf("ecommerce output category = %s, want ecommerce-assets", ecommerceCategory)
	}
}

func TestOutputStorageCategoryFallsBackToStorageBinding(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	repo := repository.NewRuntimeRepository(db)
	service := NewService(repo, runtimeConfigForTest(), runtimeSecurityForTest(), runtimeComfyForTest())

	if err := repo.CreateProductEndpoint(&models.RuntimeProductEndpoint{
		ID:           "ep-ecommerce",
		ProductCode:  "ecommerce",
		CallbackKind: "ecommerce_internal",
		BaseURL:      "http://ecommerce",
		Secret:       "secret",
		Status:       "active",
		Metadata:     `{}`,
	}); err != nil {
		t.Fatalf("CreateProductEndpoint(ecommerce) error = %v", err)
	}
	if err := repo.CreateStorageBinding(&models.StorageBinding{
		ID:           "sb-ecommerce",
		ProductCode:  "ecommerce",
		Category:     "ecommerce-assets",
		ProviderCode: "local",
		Priority:     100,
		Enabled:      true,
	}); err != nil {
		t.Fatalf("CreateStorageBinding() error = %v", err)
	}

	category := service.outputStorageCategory(&models.RuntimeJob{ProductCode: "ecommerce"})
	if category != "ecommerce-assets" {
		t.Fatalf("fallback output category = %s, want ecommerce-assets", category)
	}
}
