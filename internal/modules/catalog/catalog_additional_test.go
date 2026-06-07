package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"platform-service/internal/models"
	"platform-service/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCatalogHandlerSemanticErrorsAndExistingUpdateBindErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newCatalogTestService(t)
	handler := NewHandler(service, repository.NewFinanceRepository(repo.DB()), nil)

	product, err := service.CreateProduct(CreateProductInput{Code: "handler-boundary-prod", Name: "Handler Boundary Product"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	sku, err := service.CreateSKU(CreateSKUInput{ProductID: product.ID, Code: "handler-boundary-sku", Name: "Handler Boundary SKU", SKUType: "package", BillingMode: "prepaid"})
	if err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}
	item, err := service.CreateBillableItem(CreateBillableItemInput{ProductID: product.ID, Code: "handler-boundary-item", Name: "Handler Boundary Item", MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat"})
	if err != nil {
		t.Fatalf("CreateBillableItem: %v", err)
	}
	pkg, err := service.CreatePackage(CreatePackageInput{ProductID: product.ID, Code: "handler-boundary-pkg", Name: "Handler Boundary Package", PackageType: "subscription"})
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	card, err := service.CreateRateCard(CreateRateCardInput{ProductID: product.ID, Code: "handler-boundary-card", TargetType: "product", TargetID: product.ID, PriceModel: "flat"})
	if err != nil {
		t.Fatalf("CreateRateCard: %v", err)
	}

	for _, tc := range []struct {
		name     string
		fn       func(*gin.Context)
		path     string
		params   gin.Params
		expected string
	}{
		{"update_product_missing", handler.UpdateProduct, "/products/missing", gin.Params{{Key: "productID", Value: "missing"}}, "CATALOG_PRODUCT_NOT_FOUND"},
		{"update_sku_missing", handler.UpdateSKU, "/skus/missing", gin.Params{{Key: "skuID", Value: "missing"}}, "CATALOG_SKU_NOT_FOUND"},
		{"update_billable_item_missing", handler.UpdateBillableItem, "/items/missing", gin.Params{{Key: "billableItemID", Value: "missing"}}, "CATALOG_BILLABLE_ITEM_NOT_FOUND"},
		{"update_package_missing", handler.UpdatePackage, "/packages/missing", gin.Params{{Key: "packageID", Value: "missing"}}, "CATALOG_PACKAGE_NOT_FOUND"},
		{"update_rate_card_missing", handler.UpdateRateCard, "/rate-cards/missing", gin.Params{{Key: "rateCardID", Value: "missing"}}, "CATALOG_RATE_CARD_NOT_FOUND"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performCatalogRawAllowAny(t, tc.fn, http.MethodPut, tc.path, []byte(`{"name":"x","status":"inactive"}`), tc.params)
			assertCatalogResponse(t, resp, http.StatusNotFound, tc.expected)
		})
	}

	for _, tc := range []struct {
		name   string
		fn     func(*gin.Context)
		path   string
		params gin.Params
	}{
		{"update_product_bind", handler.UpdateProduct, "/products/" + product.ID, gin.Params{{Key: "productID", Value: product.ID}}},
		{"update_sku_bind", handler.UpdateSKU, "/skus/" + sku.ID, gin.Params{{Key: "skuID", Value: sku.ID}}},
		{"update_billable_item_bind", handler.UpdateBillableItem, "/items/" + item.ID, gin.Params{{Key: "billableItemID", Value: item.ID}}},
		{"update_package_bind", handler.UpdatePackage, "/packages/" + pkg.ID, gin.Params{{Key: "packageID", Value: pkg.ID}}},
		{"update_rate_card_bind", handler.UpdateRateCard, "/rate-cards/" + card.ID, gin.Params{{Key: "rateCardID", Value: card.ID}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performCatalogRawAllowAny(t, tc.fn, http.MethodPut, tc.path, []byte("{bad"), tc.params)
			assertCatalogResponse(t, resp, http.StatusBadRequest, "invalid update")
		})
	}

	for _, tc := range []struct {
		name    string
		fn      func(*gin.Context)
		path    string
		key     string
		message string
	}{
		{"delete_product_missing", handler.DeleteProduct, "/products/missing", "productID", "product not found"},
		{"delete_sku_missing", handler.DeleteSKU, "/skus/missing", "skuID", "sku not found"},
		{"delete_billable_item_missing", handler.DeleteBillableItem, "/items/missing", "billableItemID", "billable item not found"},
		{"delete_package_missing", handler.DeletePackage, "/packages/missing", "packageID", "package not found"},
		{"delete_rate_card_missing", handler.DeleteRateCard, "/rate-cards/missing", "rateCardID", "rate card not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performCatalogRawAllowAny(t, tc.fn, http.MethodDelete, tc.path, nil, gin.Params{{Key: tc.key, Value: "missing"}})
			assertCatalogResponse(t, resp, http.StatusNotFound, tc.message)
		})
	}

	resp := performCatalogRawAllowAny(t, handler.Offerings, http.MethodGet, "/offerings", nil, nil)
	assertCatalogResponse(t, resp, http.StatusBadRequest, "missing product_code")
	resp = performCatalogRawAllowAny(t, handler.Offerings, http.MethodGet, "/offerings?product_code=missing", nil, nil)
	assertCatalogResponse(t, resp, http.StatusNotFound, "CATALOG_PRODUCT_NOT_FOUND")
}

func TestCatalogHandlerInternalErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newCatalogTestService(t)
	handler := NewHandler(service, repository.NewFinanceRepository(repo.DB()), nil)

	product, err := service.CreateProduct(CreateProductInput{Code: "handler-internal-prod", Name: "Handler Internal Product"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	otherProduct, err := service.CreateProduct(CreateProductInput{Code: "handler-internal-other", Name: "Handler Internal Other"})
	if err != nil {
		t.Fatalf("CreateProduct other: %v", err)
	}
	sku, err := service.CreateSKU(CreateSKUInput{ProductID: product.ID, Code: "handler-internal-sku", Name: "Handler Internal SKU", SKUType: "package", BillingMode: "prepaid"})
	if err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}
	otherSKU, err := service.CreateSKU(CreateSKUInput{ProductID: product.ID, Code: "handler-internal-sku-other", Name: "Handler Internal SKU Other", SKUType: "package", BillingMode: "prepaid"})
	if err != nil {
		t.Fatalf("CreateSKU other: %v", err)
	}
	item, err := service.CreateBillableItem(CreateBillableItemInput{ProductID: product.ID, Code: "handler-internal-item", Name: "Handler Internal Item", MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat"})
	if err != nil {
		t.Fatalf("CreateBillableItem: %v", err)
	}
	otherItem, err := service.CreateBillableItem(CreateBillableItemInput{ProductID: product.ID, Code: "handler-internal-item-other", Name: "Handler Internal Item Other", MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat"})
	if err != nil {
		t.Fatalf("CreateBillableItem other: %v", err)
	}
	pkg, err := service.CreatePackage(CreatePackageInput{ProductID: product.ID, Code: "handler-internal-pkg", Name: "Handler Internal Package", PackageType: "subscription"})
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	card, err := service.CreateRateCard(CreateRateCardInput{ProductID: product.ID, Code: "handler-internal-card", TargetType: "product", TargetID: product.ID, PriceModel: "flat"})
	if err != nil {
		t.Fatalf("CreateRateCard: %v", err)
	}
	otherCard, err := service.CreateRateCard(CreateRateCardInput{ProductID: product.ID, Code: "handler-internal-card-other", TargetType: "product", TargetID: product.ID, PriceModel: "flat"})
	if err != nil {
		t.Fatalf("CreateRateCard other: %v", err)
	}

	createCases := []struct {
		name     string
		fn       func(*gin.Context)
		path     string
		body     any
		expected string
	}{
		{"create_product_duplicate", handler.CreateProduct, "/products", CreateProductInput{Code: product.Code, Name: "Duplicate"}, "CATALOG_PRODUCT_CREATE_FAILED"},
		{"create_sku_duplicate", handler.CreateSKU, "/skus", CreateSKUInput{ProductID: product.ID, Code: sku.Code, Name: "Duplicate", SKUType: "package", BillingMode: "prepaid"}, "CATALOG_SKU_CREATE_FAILED"},
		{"create_billable_item_duplicate", handler.CreateBillableItem, "/items", CreateBillableItemInput{ProductID: product.ID, Code: item.Code, Name: "Duplicate", MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat"}, "CATALOG_BILLABLE_ITEM_CREATE_FAILED"},
		{"create_package_duplicate", handler.CreatePackage, "/packages", CreatePackageInput{ProductID: product.ID, Code: pkg.Code, Name: "Duplicate", PackageType: "subscription"}, "CATALOG_PACKAGE_CREATE_FAILED"},
		{"create_rate_card_duplicate", handler.CreateRateCard, "/rate-cards", CreateRateCardInput{ProductID: product.ID, Code: card.Code, TargetType: "product", TargetID: product.ID, PriceModel: "flat"}, "CATALOG_RATE_CARD_CREATE_FAILED"},
	}
	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performCatalogJSONAllowAny(t, tc.fn, http.MethodPost, tc.path, tc.body, nil)
			assertCatalogResponse(t, resp, http.StatusInternalServerError, tc.expected)
		})
	}

	updateCases := []struct {
		name     string
		fn       func(*gin.Context)
		path     string
		body     any
		params   gin.Params
		expected string
	}{
		{"update_product_duplicate", handler.UpdateProduct, "/products/" + otherProduct.ID, UpdateProductInput{Code: product.Code}, gin.Params{{Key: "productID", Value: otherProduct.ID}}, "CATALOG_PRODUCT_UPDATE_FAILED"},
		{"update_sku_duplicate", handler.UpdateSKU, "/skus/" + otherSKU.ID, UpdateSKUInput{Code: sku.Code}, gin.Params{{Key: "skuID", Value: otherSKU.ID}}, "CATALOG_SKU_UPDATE_FAILED"},
		{"update_billable_item_duplicate", handler.UpdateBillableItem, "/items/" + otherItem.ID, UpdateBillableItemInput{Code: item.Code}, gin.Params{{Key: "billableItemID", Value: otherItem.ID}}, "CATALOG_BILLABLE_ITEM_UPDATE_FAILED"},
		{"update_rate_card_parse_error", handler.UpdateRateCard, "/rate-cards/" + otherCard.ID, UpdateRateCardInput{EffectiveFrom: "not-rfc3339"}, gin.Params{{Key: "rateCardID", Value: otherCard.ID}}, "CATALOG_RATE_CARD_UPDATE_FAILED"},
	}
	for _, tc := range updateCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performCatalogJSONAllowAny(t, tc.fn, http.MethodPut, tc.path, tc.body, tc.params)
			assertCatalogResponse(t, resp, http.StatusInternalServerError, tc.expected)
		})
	}

	callbackName := "catalog_test_force_update_error"
	if err := repo.DB().Callback().Update().Before("gorm:update").Register(callbackName, func(db *gorm.DB) {
		db.AddError(errors.New("forced update failure"))
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
	resp := performCatalogJSONAllowAny(t, handler.UpdatePackage, http.MethodPut, "/packages/"+pkg.ID, UpdatePackageInput{Name: "will fail"}, gin.Params{{Key: "packageID", Value: pkg.ID}})
	assertCatalogResponse(t, resp, http.StatusInternalServerError, "CATALOG_PACKAGE_UPDATE_FAILED")
	resp = performCatalogJSONAllowAny(t, handler.UpdateRateCard, http.MethodPut, "/rate-cards/"+card.ID, UpdateRateCardInput{Status: "inactive"}, gin.Params{{Key: "rateCardID", Value: card.ID}})
	assertCatalogResponse(t, resp, http.StatusInternalServerError, "CATALOG_RATE_CARD_UPDATE_FAILED")

	for _, tc := range []struct {
		name     string
		model    any
		invoke   func(*Handler) func(*gin.Context)
		path     string
		expected string
	}{
		{"list_products_error", &models.Product{}, func(h *Handler) func(*gin.Context) { return h.ListProducts }, "/products", "CATALOG_PRODUCT_LIST_FAILED"},
		{"list_skus_error", &models.SKU{}, func(h *Handler) func(*gin.Context) { return h.ListSKUs }, "/skus", "CATALOG_SKU_LIST_FAILED"},
		{"list_billable_items_error", &models.BillableItem{}, func(h *Handler) func(*gin.Context) { return h.ListBillableItems }, "/items", "CATALOG_BILLABLE_ITEM_LIST_FAILED"},
		{"list_packages_error", &models.CommercialPackage{}, func(h *Handler) func(*gin.Context) { return h.ListPackages }, "/packages", "CATALOG_PACKAGE_LIST_FAILED"},
		{"list_rate_cards_error", &models.RateCard{}, func(h *Handler) func(*gin.Context) { return h.ListRateCards }, "/rate-cards", "CATALOG_RATE_CARD_LIST_FAILED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listService, listRepo := newCatalogServiceWithMigrations(t, &models.Product{}, &models.SKU{}, &models.BillableItem{}, &models.CommercialPackage{}, &models.RateCard{})
			listHandler := NewHandler(listService, repository.NewFinanceRepository(listRepo.DB()), nil)
			if err := listRepo.DB().Migrator().DropTable(tc.model); err != nil {
				t.Fatalf("drop table: %v", err)
			}
			resp := performCatalogRawAllowAny(t, tc.invoke(listHandler), http.MethodGet, tc.path, nil, nil)
			assertCatalogResponse(t, resp, http.StatusInternalServerError, tc.expected)
		})
	}

	offeringService, offeringRepo := newCatalogTestService(t)
	offeringHandler := NewHandler(offeringService, repository.NewFinanceRepository(offeringRepo.DB()), nil)
	if _, err := offeringService.CreateProduct(CreateProductInput{Code: "offer-finance-missing", Name: "Offer Finance Missing"}); err != nil {
		t.Fatalf("CreateProduct offering: %v", err)
	}
	resp = performCatalogRawAllowAny(t, offeringHandler.Offerings, http.MethodGet, "/offerings?product_code=offer-finance-missing", nil, nil)
	assertCatalogResponse(t, resp, http.StatusInternalServerError, "CATALOG_OFFERINGS_GET_FAILED")
}

func TestCatalogServiceBoundaryUpdatesAndDependencyErrors(t *testing.T) {
	service, repo := newCatalogTestService(t)
	product, err := service.CreateProduct(CreateProductInput{Code: "service-boundary-prod", Name: "Service Boundary Product", Metadata: "{}"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	otherProduct, err := service.CreateProduct(CreateProductInput{Code: "service-boundary-other", Name: "Service Boundary Other"})
	if err != nil {
		t.Fatalf("CreateProduct other: %v", err)
	}

	updatedProduct, err := service.UpdateProduct(product.ID, UpdateProductInput{Code: "service-boundary-prod-v2", Name: "Service Boundary Product V2", Status: "inactive", OwnerTeam: "billing", Metadata: `{"version":2}`})
	if err != nil {
		t.Fatalf("UpdateProduct full: %v", err)
	}
	if updatedProduct.Code != "service-boundary-prod-v2" || updatedProduct.Status != "inactive" || updatedProduct.OwnerTeam != "billing" || updatedProduct.Metadata == "" {
		t.Fatalf("unexpected updated product: %+v", updatedProduct)
	}

	sku, err := service.CreateSKU(CreateSKUInput{ProductID: product.ID, Code: "service-boundary-sku", Name: "Service Boundary SKU", SKUType: "package", BillingMode: "prepaid", Currency: "USD", ListPrice: 10, Status: "draft", Metadata: "{}"})
	if err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}
	price := int64(2500)
	updatedSKU, err := service.UpdateSKU(sku.ID, UpdateSKUInput{ProductID: otherProduct.ID, Code: "service-boundary-sku-v2", Name: "Service Boundary SKU V2", SKUType: "seat", BillingMode: "postpaid", Currency: "EUR", ListPrice: &price, Status: "inactive", Metadata: `{"sku":2}`})
	if err != nil {
		t.Fatalf("UpdateSKU full: %v", err)
	}
	if updatedSKU.ProductID != otherProduct.ID || updatedSKU.Code != "service-boundary-sku-v2" || updatedSKU.ListPrice != price || updatedSKU.Currency != "EUR" {
		t.Fatalf("unexpected updated sku: %+v", updatedSKU)
	}

	item, err := service.CreateBillableItem(CreateBillableItemInput{ProductID: product.ID, Code: "service-boundary-item", Name: "Service Boundary Item", MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat"})
	if err != nil {
		t.Fatalf("CreateBillableItem: %v", err)
	}
	updatedItem, err := service.UpdateBillableItem(item.ID, UpdateBillableItemInput{ProductID: otherProduct.ID, Code: "service-boundary-item-v2", Name: "Service Boundary Item V2", MeterUnit: "tokens", BillingScope: "user", SettlementMode: "usage_billing", PricingBehavior: "tiered", Status: "inactive", Metadata: `{"item":2}`})
	if err != nil {
		t.Fatalf("UpdateBillableItem full: %v", err)
	}
	if updatedItem.ProductID != otherProduct.ID || updatedItem.Code != "service-boundary-item-v2" || updatedItem.MeterUnit != "tokens" || updatedItem.PricingBehavior != "tiered" {
		t.Fatalf("unexpected updated billable item: %+v", updatedItem)
	}

	pkg, err := service.CreatePackage(CreatePackageInput{ProductID: product.ID, Code: "service-boundary-pkg", Name: "Service Boundary Package", PackageType: "subscription"})
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	updatedPackage, err := service.UpdatePackage(pkg.ID, UpdatePackageInput{Name: "Service Boundary Package V2", PackageType: "one_time", Status: "inactive", Metadata: `{"pkg":2}`})
	if err != nil {
		t.Fatalf("UpdatePackage full: %v", err)
	}
	if updatedPackage.PackageType != "one_time" || updatedPackage.Status != "inactive" || updatedPackage.Metadata == "" {
		t.Fatalf("unexpected updated package: %+v", updatedPackage)
	}

	rateCard, err := service.CreateRateCard(CreateRateCardInput{ProductID: product.ID, Code: "service-boundary-card", TargetType: "product", TargetID: product.ID, PriceModel: "flat", Currency: "USD", PriceConfig: `{"amount":1}`, EffectiveFrom: "2025-01-01T00:00:00Z", EffectiveTo: "2025-12-31T23:59:59Z", Version: 3, Status: "draft", Metadata: `{"card":1}`})
	if err != nil {
		t.Fatalf("CreateRateCard explicit: %v", err)
	}
	if rateCard.Version != 3 || rateCard.Currency != "USD" || rateCard.EffectiveFrom == nil || rateCard.EffectiveTo == nil {
		t.Fatalf("unexpected created rate card: %+v", rateCard)
	}
	updatedRateCard, err := service.UpdateRateCard(rateCard.ID, UpdateRateCardInput{PriceModel: "tiered", Currency: "EUR", PriceConfig: `{"tiers":[]}`, EffectiveFrom: "2026-01-01T00:00:00Z", EffectiveTo: "2026-12-31T23:59:59Z", Version: 4, Status: "active", Metadata: `{"card":2}`})
	if err != nil {
		t.Fatalf("UpdateRateCard full: %v", err)
	}
	if updatedRateCard.PriceModel != "tiered" || updatedRateCard.Currency != "EUR" || updatedRateCard.Version != 4 || updatedRateCard.EffectiveFrom == nil || updatedRateCard.EffectiveTo == nil {
		t.Fatalf("unexpected updated rate card: %+v", updatedRateCard)
	}
	if _, err := service.CreateRateCard(CreateRateCardInput{ProductID: product.ID, Code: "service-boundary-card-bad", TargetType: "product", TargetID: product.ID, PriceModel: "flat", EffectiveFrom: "not-rfc3339"}); err == nil {
		t.Fatalf("expected CreateRateCard invalid effective window error")
	}

	deleteCallbackName := "catalog_test_force_delete_error"
	if err := repo.DB().Callback().Delete().Before("gorm:delete").Register(deleteCallbackName, func(db *gorm.DB) {
		db.AddError(errors.New("forced delete failure"))
	}); err != nil {
		t.Fatalf("register delete callback: %v", err)
	}
	if _, err := service.DeleteProduct(otherProduct.ID); err == nil {
		t.Fatalf("expected DeleteProduct forced error")
	}
	if _, err := service.DeleteSKU(sku.ID); err == nil {
		t.Fatalf("expected DeleteSKU forced error")
	}
	if _, err := service.DeleteBillableItem(item.ID); err == nil {
		t.Fatalf("expected DeleteBillableItem forced error")
	}
	if _, err := service.DeletePackage(pkg.ID); err == nil {
		t.Fatalf("expected DeletePackage forced error")
	}
	if _, err := service.DeleteRateCard(rateCard.ID); err == nil {
		t.Fatalf("expected DeleteRateCard forced error")
	}
}

func TestCatalogServiceOfferingsDependencyErrors(t *testing.T) {
	for _, tc := range []struct {
		name          string
		migrations    []any
		seedSKU       bool
		seedPackage   bool
		seedItem      bool
		seedRateCard  bool
		expectedError string
	}{
		{"missing_sku_table", []any{&models.Product{}}, false, false, false, false, "skus"},
		{"missing_package_table", []any{&models.Product{}, &models.SKU{}}, true, false, false, false, "commercial_packages"},
		{"missing_billable_item_table", []any{&models.Product{}, &models.SKU{}, &models.CommercialPackage{}}, true, true, false, false, "billable_items"},
		{"missing_rate_card_table", []any{&models.Product{}, &models.SKU{}, &models.CommercialPackage{}, &models.BillableItem{}}, true, true, true, false, "rate_cards"},
		{"missing_asset_definition_table", []any{&models.Product{}, &models.SKU{}, &models.CommercialPackage{}, &models.BillableItem{}, &models.RateCard{}}, true, true, true, true, "asset_definitions"},
		{"missing_allowance_policy_table", []any{&models.Product{}, &models.SKU{}, &models.CommercialPackage{}, &models.BillableItem{}, &models.RateCard{}, &models.AssetDefinition{}}, true, true, true, true, "allowance_policies"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, repo := newCatalogServiceWithMigrations(t, tc.migrations...)
			product, err := service.CreateProduct(CreateProductInput{Code: "offer-dependency-" + tc.name, Name: "Offer Dependency"})
			if err != nil {
				t.Fatalf("CreateProduct: %v", err)
			}
			if tc.seedSKU {
				if _, err := service.CreateSKU(CreateSKUInput{ProductID: product.ID, Code: "sku-" + tc.name, Name: "SKU", SKUType: "package", BillingMode: "prepaid"}); err != nil {
					t.Fatalf("CreateSKU: %v", err)
				}
			}
			if tc.seedPackage {
				if _, err := service.CreatePackage(CreatePackageInput{ProductID: product.ID, Code: "pkg-" + tc.name, Name: "Package", PackageType: "subscription"}); err != nil {
					t.Fatalf("CreatePackage: %v", err)
				}
			}
			if tc.seedItem {
				if _, err := service.CreateBillableItem(CreateBillableItemInput{ProductID: product.ID, Code: "item-" + tc.name, Name: "Item", MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat"}); err != nil {
					t.Fatalf("CreateBillableItem: %v", err)
				}
			}
			if tc.seedRateCard {
				if _, err := service.CreateRateCard(CreateRateCardInput{ProductID: product.ID, Code: "card-" + tc.name, TargetType: "product", TargetID: product.ID, PriceModel: "flat"}); err != nil {
					t.Fatalf("CreateRateCard: %v", err)
				}
			}
			_, err = service.Offerings(product.Code, repository.NewFinanceRepository(repo.DB()))
			if err == nil {
				t.Fatalf("expected Offerings dependency error")
			}
			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Fatalf("expected error mentioning %q, got %v", tc.expectedError, err)
			}
		})
	}
}

func performCatalogJSONAllowAny(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performCatalogRawAllowAny(t, fn, method, path, payload, params)
}

func performCatalogRawAllowAny(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	c.Params = params
	fn(c)
	return w
}

func assertCatalogResponse(t *testing.T, resp *httptest.ResponseRecorder, expectedStatus int, expectedBody string) {
	t.Helper()
	if resp.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d body=%s", expectedStatus, resp.Code, resp.Body.String())
	}
	if expectedBody != "" && !strings.Contains(resp.Body.String(), expectedBody) {
		t.Fatalf("expected body to contain %q, got %s", expectedBody, resp.Body.String())
	}
}

func newCatalogServiceWithMigrations(t *testing.T, migrations ...any) (*Service, *repository.CommercialRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "catalog.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if len(migrations) > 0 {
		if err := db.AutoMigrate(migrations...); err != nil {
			t.Fatalf("auto migrate: %v", err)
		}
	}
	repo := repository.NewCommercialRepository(db)
	return NewService(repo), repo
}
