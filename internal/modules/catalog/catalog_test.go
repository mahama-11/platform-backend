package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCatalogTestService(t *testing.T) (*Service, *repository.CommercialRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("catalog-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.SKU{}, &models.BillableItem{}, &models.CommercialPackage{}, &models.RateCard{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCommercialRepository(db)
	return NewService(repo), repo
}

func TestCatalogServiceAndHandler(t *testing.T) {
	service, repo := newCatalogTestService(t)
	product, err := service.CreateProduct(CreateProductInput{Code: "ecommerce", Name: "Ecommerce"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if _, err := service.CreateSKU(CreateSKUInput{ProductID: product.ID, Code: "sku-1", Name: "SKU 1", SKUType: "package", BillingMode: "prepaid"}); err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}
	if _, err := service.CreateBillableItem(CreateBillableItemInput{ProductID: product.ID, Code: "item-1", Name: "Item 1", MeterUnit: "times", BillingScope: "org", SettlementMode: "usage_billing", PricingBehavior: "flat"}); err != nil {
		t.Fatalf("CreateBillableItem: %v", err)
	}
	pkg, err := service.CreatePackage(CreatePackageInput{ProductID: product.ID, Code: "pkg-1", Name: "Package 1", PackageType: "subscription"})
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	if gotPkg, err := service.GetPackage(pkg.ID); err != nil || gotPkg.ID != pkg.ID {
		t.Fatalf("GetPackage: %+v err=%v", gotPkg, err)
	}
	if _, err := service.UpdatePackage(pkg.ID, UpdatePackageInput{Name: "Package 1 Updated"}); err != nil {
		t.Fatalf("UpdatePackage: %v", err)
	}
	rateCard, err := service.CreateRateCard(CreateRateCardInput{ProductID: product.ID, Code: "rc-1", TargetType: "product", TargetID: product.ID, PriceModel: "flat"})
	if err != nil {
		t.Fatalf("CreateRateCard: %v", err)
	}
	if cards, err := service.ListRateCards(product.ID, "product"); err != nil || len(cards) != 1 {
		t.Fatalf("ListRateCards: %+v err=%v", cards, err)
	}
	card, err := service.GetRateCard(rateCard.ID)
	if err != nil || card.ID != rateCard.ID {
		t.Fatalf("GetRateCard: %+v err=%v", card, err)
	}
	if _, err := service.UpdateRateCard(rateCard.ID, UpdateRateCardInput{Currency: "CNY", Status: "active"}); err != nil {
		t.Fatalf("UpdateRateCard: %v", err)
	}
	if _, err := service.DeleteRateCard("missing"); err == nil {
		t.Fatalf("expected missing rate card delete error")
	}
	if _, _, err := parseEffectiveWindow("bad-time", ""); err == nil {
		t.Fatalf("expected parseEffectiveWindow error")
	}
	if defaultString("", "fallback") != "fallback" || defaultInt(0, 7) != 7 {
		t.Fatalf("expected default helpers to use fallback values")
	}
	if _, err := service.UpdateRateCard("missing", UpdateRateCardInput{}); err == nil {
		t.Fatalf("expected missing rate card error")
	}
	gin.SetMode(gin.TestMode)
	handler := NewHandler(service, repository.NewFinanceRepository(repo.DB()), nil)
	resp := performCatalogJSON(t, handler.CreateProduct, http.MethodPost, "/products", CreateProductInput{Code: "p2", Name: "P2"}, nil)
	if resp.Code != http.StatusCreated {
		t.Fatalf("CreateProduct handler status=%d body=%s", resp.Code, resp.Body.String())
	}
	performCatalogJSON(t, handler.CreateSKU, http.MethodPost, "/skus", CreateSKUInput{ProductID: product.ID, Code: "sku-2", Name: "SKU 2", SKUType: "package", BillingMode: "postpaid"}, nil)
	performCatalogJSON(t, handler.CreateBillableItem, http.MethodPost, "/items", CreateBillableItemInput{ProductID: product.ID, Code: "item-2", Name: "Item 2", MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat"}, nil)
	createdPkgResp := performCatalogJSON(t, handler.CreatePackage, http.MethodPost, "/packages", CreatePackageInput{ProductID: product.ID, Code: "pkg-2", Name: "Package 2", PackageType: "subscription"}, nil)
	createdPkgID := extractCatalogID(t, createdPkgResp)
	createdRateCardResp := performCatalogJSON(t, handler.CreateRateCard, http.MethodPost, "/rate-cards", CreateRateCardInput{ProductID: product.ID, Code: "rc-2", TargetType: "product", TargetID: product.ID, PriceModel: "flat"}, nil)
	createdRateCardID := extractCatalogID(t, createdRateCardResp)
	performCatalogQuery(t, handler.ListProducts, "/products")
	performCatalogQuery(t, handler.ListSKUs, "/skus?product_id="+product.ID)
	performCatalogQuery(t, handler.ListBillableItems, "/items?product_id="+product.ID)
	performCatalogQuery(t, handler.ListPackages, "/packages?product_id="+product.ID)
	performCatalogJSON(t, handler.UpdatePackage, http.MethodPut, "/packages/"+pkg.ID, UpdatePackageInput{Name: "Pkg Via Handler"}, gin.Params{{Key: "packageID", Value: pkg.ID}})
	performCatalogParam(t, handler.DeletePackage, http.MethodDelete, "/packages/"+pkg.ID, "packageID", pkg.ID, nil)
	performCatalogQuery(t, handler.ListRateCards, "/rate-cards?product_id="+product.ID+"&target_type=product")
	performCatalogJSON(t, handler.UpdateRateCard, http.MethodPut, "/rate-cards/"+rateCard.ID, UpdateRateCardInput{Status: "inactive"}, gin.Params{{Key: "rateCardID", Value: rateCard.ID}})
	performCatalogParam(t, handler.DeleteRateCard, http.MethodDelete, "/rate-cards/"+rateCard.ID, "rateCardID", rateCard.ID, nil)
	performCatalogParam(t, handler.DeletePackage, http.MethodDelete, "/packages/"+createdPkgID, "packageID", createdPkgID, nil)
	performCatalogParam(t, handler.DeleteRateCard, http.MethodDelete, "/rate-cards/"+createdRateCardID, "rateCardID", createdRateCardID, nil)
	resp = performCatalogRaw(t, handler.CreateSKU, http.MethodPost, "/skus", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected CreateSKU bind error")
	}
	resp = performCatalogParam(t, handler.UpdateRateCard, http.MethodPut, "/rate-cards/missing", "rateCardID", "missing", UpdateRateCardInput{Status: "inactive"})
	if resp.Code == http.StatusOK {
		t.Fatalf("expected UpdateRateCard missing error")
	}
	resp = performCatalogParam(t, handler.DeletePackage, http.MethodDelete, "/packages/missing", "packageID", "missing", nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected DeletePackage missing error")
	}
}

func extractCatalogID(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal catalog response: %v body=%s", err, resp.Body.String())
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["id"] == nil {
		t.Fatalf("missing catalog data.id: %s", resp.Body.String())
	}
	return data["id"].(string)
}

func performCatalogJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performCatalogRaw(t, fn, method, path, payload, params)
}

func performCatalogQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performCatalogRaw(t, fn, http.MethodGet, path, nil, nil)
}

func performCatalogParam(t *testing.T, fn func(*gin.Context), method, path, key, value string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	return performCatalogRaw(t, fn, method, path, payload, gin.Params{{Key: key, Value: value}})
}

func performCatalogRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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
	if w.Code >= 500 {
		t.Fatalf("unexpected handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func TestUpdateAndDeleteSKU(t *testing.T) {
	service, _ := newCatalogTestService(t)

	product, err := service.CreateProduct(CreateProductInput{Code: "sku-test-product", Name: "SKU Test Product"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	sku, err := service.CreateSKU(CreateSKUInput{
		ProductID: product.ID, Code: "sku-upd-1", Name: "Original SKU",
		SKUType: "package", BillingMode: "prepaid",
	})
	if err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}

	// UpdateSKU: partial update – name only
	updated, err := service.UpdateSKU(sku.ID, UpdateSKUInput{Name: "Renamed SKU"})
	if err != nil {
		t.Fatalf("UpdateSKU name-only: %v", err)
	}
	if updated.Name != "Renamed SKU" {
		t.Fatalf("expected name 'Renamed SKU', got %q", updated.Name)
	}
	if updated.SKUType != "package" {
		t.Fatalf("expected sku_type unchanged 'package', got %q", updated.SKUType)
	}

	// UpdateSKU: ListPrice pointer update
	price := int64(9900)
	updated, err = service.UpdateSKU(sku.ID, UpdateSKUInput{ListPrice: &price})
	if err != nil {
		t.Fatalf("UpdateSKU list_price: %v", err)
	}
	if updated.ListPrice != 9900 {
		t.Fatalf("expected list_price 9900, got %d", updated.ListPrice)
	}

	// UpdateSKU: not found
	if _, err := service.UpdateSKU("nonexistent-sku", UpdateSKUInput{Name: "X"}); err == nil {
		t.Fatalf("expected UpdateSKU not-found error")
	}

	// DeleteSKU: success
	deleted, err := service.DeleteSKU(sku.ID)
	if err != nil {
		t.Fatalf("DeleteSKU: %v", err)
	}
	if deleted.ID != sku.ID {
		t.Fatalf("expected deleted ID %q, got %q", sku.ID, deleted.ID)
	}

	// DeleteSKU: not found (already deleted)
	if _, err := service.DeleteSKU(sku.ID); err == nil {
		t.Fatalf("expected DeleteSKU not-found error")
	}
}

func TestUpdateAndDeleteBillableItem(t *testing.T) {
	service, _ := newCatalogTestService(t)

	product, err := service.CreateProduct(CreateProductInput{Code: "bi-test-product", Name: "BI Test Product"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	item, err := service.CreateBillableItem(CreateBillableItemInput{
		ProductID: product.ID, Code: "bi-upd-1", Name: "Original Item",
		MeterUnit: "times", BillingScope: "org", SettlementMode: "usage_billing", PricingBehavior: "flat",
	})
	if err != nil {
		t.Fatalf("CreateBillableItem: %v", err)
	}

	// UpdateBillableItem: partial update – name + status
	updated, err := service.UpdateBillableItem(item.ID, UpdateBillableItemInput{Name: "Renamed Item", Status: "inactive"})
	if err != nil {
		t.Fatalf("UpdateBillableItem partial: %v", err)
	}
	if updated.Name != "Renamed Item" {
		t.Fatalf("expected name 'Renamed Item', got %q", updated.Name)
	}
	if updated.Status != "inactive" {
		t.Fatalf("expected status 'inactive', got %q", updated.Status)
	}
	if updated.MeterUnit != "times" {
		t.Fatalf("expected meter_unit unchanged 'times', got %q", updated.MeterUnit)
	}

	// UpdateBillableItem: not found
	if _, err := service.UpdateBillableItem("nonexistent-bi", UpdateBillableItemInput{Name: "X"}); err == nil {
		t.Fatalf("expected UpdateBillableItem not-found error")
	}

	// DeleteBillableItem: success
	deleted, err := service.DeleteBillableItem(item.ID)
	if err != nil {
		t.Fatalf("DeleteBillableItem: %v", err)
	}
	if deleted.ID != item.ID {
		t.Fatalf("expected deleted ID %q, got %q", item.ID, deleted.ID)
	}

	// DeleteBillableItem: not found
	if _, err := service.DeleteBillableItem(item.ID); err == nil {
		t.Fatalf("expected DeleteBillableItem not-found error")
	}
}

func TestOfferings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("catalog-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Product{}, &models.SKU{}, &models.BillableItem{},
		&models.CommercialPackage{}, &models.RateCard{},
		&models.AssetDefinition{}, &models.AllowancePolicy{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCommercialRepository(db)
	financeRepo := repository.NewFinanceRepository(db)
	service := NewService(repo)

	// Offerings: product code not found
	if _, err := service.Offerings("nonexistent-code", financeRepo); err == nil {
		t.Fatalf("expected Offerings not-found error for missing product code")
	}

	// Create product with related data
	product, err := service.CreateProduct(CreateProductInput{Code: "offer-prod", Name: "Offering Product"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if _, err := service.CreateSKU(CreateSKUInput{
		ProductID: product.ID, Code: "offer-sku-1", Name: "Offer SKU",
		SKUType: "package", BillingMode: "prepaid",
	}); err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}
	if _, err := service.CreatePackage(CreatePackageInput{
		ProductID: product.ID, Code: "offer-pkg-1", Name: "Offer Pkg", PackageType: "subscription",
	}); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	if _, err := service.CreateBillableItem(CreateBillableItemInput{
		ProductID: product.ID, Code: "offer-bi-1", Name: "Offer BI",
		MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat",
	}); err != nil {
		t.Fatalf("CreateBillableItem: %v", err)
	}
	if _, err := service.CreateRateCard(CreateRateCardInput{
		ProductID: product.ID, Code: "offer-rc-1", TargetType: "product", TargetID: product.ID, PriceModel: "flat",
	}); err != nil {
		t.Fatalf("CreateRateCard: %v", err)
	}

	// Offerings: success
	view, err := service.Offerings("offer-prod", financeRepo)
	if err != nil {
		t.Fatalf("Offerings: %v", err)
	}
	if view.Product == nil || view.Product.Code != "offer-prod" {
		t.Fatalf("expected product code 'offer-prod', got %+v", view.Product)
	}
	if len(view.SKUs) != 1 {
		t.Fatalf("expected 1 SKU, got %d", len(view.SKUs))
	}
	if len(view.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(view.Packages))
	}
	if len(view.BillableItems) != 1 {
		t.Fatalf("expected 1 billable item, got %d", len(view.BillableItems))
	}
	if len(view.RateCards) != 1 {
		t.Fatalf("expected 1 rate card, got %d", len(view.RateCards))
	}

	// Offerings: product exists but no related data → empty slices
	emptyProduct, err := service.CreateProduct(CreateProductInput{Code: "empty-prod", Name: "Empty Product"})
	if err != nil {
		t.Fatalf("CreateProduct empty: %v", err)
	}
	_ = emptyProduct
	emptyView, err := service.Offerings("empty-prod", financeRepo)
	if err != nil {
		t.Fatalf("Offerings empty: %v", err)
	}
	if emptyView.Product == nil {
		t.Fatalf("expected non-nil product for empty offerings")
	}
	if len(emptyView.SKUs) != 0 || len(emptyView.Packages) != 0 || len(emptyView.BillableItems) != 0 || len(emptyView.RateCards) != 0 {
		t.Fatalf("expected empty slices, got SKUs=%d Packages=%d BillableItems=%d RateCards=%d",
			len(emptyView.SKUs), len(emptyView.Packages), len(emptyView.BillableItems), len(emptyView.RateCards))
	}
}

func TestProductCRUD(t *testing.T) {
	service, _ := newCatalogTestService(t)

	// CreateProduct
	product, err := service.CreateProduct(CreateProductInput{Code: "crud-prod", Name: "CRUD Product", Status: "draft", OwnerTeam: "platform"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if product.Status != "draft" {
		t.Fatalf("expected status 'draft', got %q", product.Status)
	}

	// GetProduct
	got, err := service.GetProduct(product.ID)
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if got.Code != "crud-prod" {
		t.Fatalf("expected code 'crud-prod', got %q", got.Code)
	}

	// GetProduct: not found
	if _, err := service.GetProduct("nonexistent-product"); err == nil {
		t.Fatalf("expected GetProduct not-found error")
	}

	// UpdateProduct: partial update
	updated, err := service.UpdateProduct(product.ID, UpdateProductInput{Name: "CRUD Product Updated", OwnerTeam: "billing"})
	if err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}
	if updated.Name != "CRUD Product Updated" {
		t.Fatalf("expected name 'CRUD Product Updated', got %q", updated.Name)
	}
	if updated.OwnerTeam != "billing" {
		t.Fatalf("expected owner_team 'billing', got %q", updated.OwnerTeam)
	}
	if updated.Status != "draft" {
		t.Fatalf("expected status unchanged 'draft', got %q", updated.Status)
	}

	// UpdateProduct: not found
	if _, err := service.UpdateProduct("nonexistent-product", UpdateProductInput{Name: "X"}); err == nil {
		t.Fatalf("expected UpdateProduct not-found error")
	}

	// ListProducts
	service.CreateProduct(CreateProductInput{Code: "crud-prod-2", Name: "CRUD Product 2"})
	products, err := service.ListProducts()
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(products) < 2 {
		t.Fatalf("expected at least 2 products, got %d", len(products))
	}

	// DeleteProduct: success
	deleted, err := service.DeleteProduct(product.ID)
	if err != nil {
		t.Fatalf("DeleteProduct: %v", err)
	}
	if deleted.ID != product.ID {
		t.Fatalf("expected deleted ID %q, got %q", product.ID, deleted.ID)
	}

	// DeleteProduct: not found
	if _, err := service.DeleteProduct(product.ID); err == nil {
		t.Fatalf("expected DeleteProduct not-found error")
	}
}

func TestParseEffectiveWindowNormalPath(t *testing.T) {
	// Both from and to valid RFC3339
	from, to, err := parseEffectiveWindow("2025-01-01T00:00:00Z", "2025-12-31T23:59:59Z")
	if err != nil {
		t.Fatalf("parseEffectiveWindow both: %v", err)
	}
	if from == nil || to == nil {
		t.Fatalf("expected non-nil from and to")
	}
	expectedFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	expectedTo := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
	if !from.Equal(expectedFrom) {
		t.Fatalf("expected from %v, got %v", expectedFrom, *from)
	}
	if !to.Equal(expectedTo) {
		t.Fatalf("expected to %v, got %v", expectedTo, *to)
	}

	// Only from
	from, to, err = parseEffectiveWindow("2025-06-15T12:00:00Z", "")
	if err != nil {
		t.Fatalf("parseEffectiveWindow from-only: %v", err)
	}
	if from == nil {
		t.Fatalf("expected non-nil from")
	}
	if to != nil {
		t.Fatalf("expected nil to, got %v", *to)
	}

	// Only to
	from, to, err = parseEffectiveWindow("", "2025-06-15T12:00:00Z")
	if err != nil {
		t.Fatalf("parseEffectiveWindow to-only: %v", err)
	}
	if from != nil {
		t.Fatalf("expected nil from, got %v", *from)
	}
	if to == nil {
		t.Fatalf("expected non-nil to")
	}

	// Both empty
	from, to, err = parseEffectiveWindow("", "")
	if err != nil {
		t.Fatalf("parseEffectiveWindow both-empty: %v", err)
	}
	if from != nil || to != nil {
		t.Fatalf("expected nil from and to for empty strings")
	}

	// Invalid 'to' value
	if _, _, err := parseEffectiveWindow("2025-01-01T00:00:00Z", "bad-to"); err == nil {
		t.Fatalf("expected parseEffectiveWindow error for bad 'to'")
	}
}

func TestHandlerCRUDExtended(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("catalog-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Product{}, &models.SKU{}, &models.BillableItem{},
		&models.CommercialPackage{}, &models.RateCard{},
		&models.AssetDefinition{}, &models.AllowancePolicy{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCommercialRepository(db)
	financeRepo := repository.NewFinanceRepository(db)
	service := NewService(repo)
	gin.SetMode(gin.TestMode)
	handler := NewHandler(service, financeRepo, nil)

	// Create base product
	createProductResp := performCatalogJSON(t, handler.CreateProduct, http.MethodPost, "/products", CreateProductInput{Code: "handler-prod", Name: "Handler Product"}, nil)
	productID := extractCatalogID(t, createProductResp)

	// --- Product CRUD handlers ---
	// UpdateProduct handler: success
	performCatalogJSON(t, handler.UpdateProduct, http.MethodPut, "/products/"+productID,
		UpdateProductInput{Name: "Handler Product Updated"},
		gin.Params{{Key: "productID", Value: productID}})

	// UpdateProduct handler: not found
	resp := performCatalogParam(t, handler.UpdateProduct, http.MethodPut, "/products/missing", "productID", "missing",
		UpdateProductInput{Name: "X"})
	if resp.Code == http.StatusOK {
		t.Fatalf("expected UpdateProduct handler not-found error")
	}

	// DeleteProduct handler: success
	// Create a temporary product to delete
	tmpProdResp := performCatalogJSON(t, handler.CreateProduct, http.MethodPost, "/products", CreateProductInput{Code: "handler-prod-del", Name: "To Delete"}, nil)
	tmpProdID := extractCatalogID(t, tmpProdResp)
	performCatalogParam(t, handler.DeleteProduct, http.MethodDelete, "/products/"+tmpProdID, "productID", tmpProdID, nil)

	// DeleteProduct handler: not found
	resp = performCatalogParam(t, handler.DeleteProduct, http.MethodDelete, "/products/missing", "productID", "missing", nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected DeleteProduct handler not-found error")
	}

	// --- SKU CRUD handlers ---
	createSKUResp := performCatalogJSON(t, handler.CreateSKU, http.MethodPost, "/skus",
		CreateSKUInput{ProductID: productID, Code: "handler-sku-1", Name: "Handler SKU", SKUType: "package", BillingMode: "prepaid"}, nil)
	skuID := extractCatalogID(t, createSKUResp)

	// UpdateSKU handler: success
	performCatalogJSON(t, handler.UpdateSKU, http.MethodPut, "/skus/"+skuID,
		UpdateSKUInput{Name: "Handler SKU Updated"},
		gin.Params{{Key: "skuID", Value: skuID}})

	// UpdateSKU handler: not found
	resp = performCatalogParam(t, handler.UpdateSKU, http.MethodPut, "/skus/missing", "skuID", "missing",
		UpdateSKUInput{Name: "X"})
	if resp.Code == http.StatusOK {
		t.Fatalf("expected UpdateSKU handler not-found error")
	}

	// DeleteSKU handler: success
	performCatalogParam(t, handler.DeleteSKU, http.MethodDelete, "/skus/"+skuID, "skuID", skuID, nil)

	// DeleteSKU handler: not found
	resp = performCatalogParam(t, handler.DeleteSKU, http.MethodDelete, "/skus/missing", "skuID", "missing", nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected DeleteSKU handler not-found error")
	}

	// --- BillableItem CRUD handlers ---
	createBIResp := performCatalogJSON(t, handler.CreateBillableItem, http.MethodPost, "/items",
		CreateBillableItemInput{ProductID: productID, Code: "handler-bi-1", Name: "Handler BI",
			MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat"}, nil)
	biID := extractCatalogID(t, createBIResp)

	// UpdateBillableItem handler: success
	performCatalogJSON(t, handler.UpdateBillableItem, http.MethodPut, "/items/"+biID,
		UpdateBillableItemInput{Name: "Handler BI Updated"},
		gin.Params{{Key: "billableItemID", Value: biID}})

	// UpdateBillableItem handler: not found
	resp = performCatalogParam(t, handler.UpdateBillableItem, http.MethodPut, "/items/missing", "billableItemID", "missing",
		UpdateBillableItemInput{Name: "X"})
	if resp.Code == http.StatusOK {
		t.Fatalf("expected UpdateBillableItem handler not-found error")
	}

	// DeleteBillableItem handler: success
	performCatalogParam(t, handler.DeleteBillableItem, http.MethodDelete, "/items/"+biID, "billableItemID", biID, nil)

	// DeleteBillableItem handler: not found
	resp = performCatalogParam(t, handler.DeleteBillableItem, http.MethodDelete, "/items/missing", "billableItemID", "missing", nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected DeleteBillableItem handler not-found error")
	}

	// --- Offerings handler ---
	// Create related data for offerings
	performCatalogJSON(t, handler.CreateSKU, http.MethodPost, "/skus",
		CreateSKUInput{ProductID: productID, Code: "handler-sku-off", Name: "Off SKU", SKUType: "package", BillingMode: "prepaid"}, nil)
	performCatalogJSON(t, handler.CreatePackage, http.MethodPost, "/packages",
		CreatePackageInput{ProductID: productID, Code: "handler-pkg-off", Name: "Off Pkg", PackageType: "subscription"}, nil)
	performCatalogJSON(t, handler.CreateRateCard, http.MethodPost, "/rate-cards",
		CreateRateCardInput{ProductID: productID, Code: "handler-rc-off", TargetType: "product", TargetID: productID, PriceModel: "flat"}, nil)

	// Offerings handler: success
	resp = performCatalogQuery(t, handler.Offerings, "/offerings?product_code=handler-prod")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected Offerings handler 200, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Offerings handler: missing product_code query param
	resp = performCatalogQuery(t, handler.Offerings, "/offerings")
	if resp.Code == http.StatusOK {
		t.Fatalf("expected Offerings handler error for missing product_code")
	}

	// Offerings handler: product code not found
	resp = performCatalogQuery(t, handler.Offerings, "/offerings?product_code=nonexistent")
	if resp.Code == http.StatusOK {
		t.Fatalf("expected Offerings handler error for nonexistent product code")
	}
}

func TestGetSKUAndGetBillableItem(t *testing.T) {
	service, _ := newCatalogTestService(t)

	product, err := service.CreateProduct(CreateProductInput{Code: "get-test-prod", Name: "Get Test Product"})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// GetSKU
	sku, err := service.CreateSKU(CreateSKUInput{
		ProductID: product.ID, Code: "get-sku-1", Name: "Get SKU",
		SKUType: "package", BillingMode: "prepaid",
	})
	if err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}
	got, err := service.GetSKU(sku.ID)
	if err != nil {
		t.Fatalf("GetSKU: %v", err)
	}
	if got.ID != sku.ID {
		t.Fatalf("expected SKU ID %q, got %q", sku.ID, got.ID)
	}
	if _, err := service.GetSKU("nonexistent"); err == nil {
		t.Fatalf("expected GetSKU not-found error")
	}

	// GetBillableItem
	bi, err := service.CreateBillableItem(CreateBillableItemInput{
		ProductID: product.ID, Code: "get-bi-1", Name: "Get BI",
		MeterUnit: "times", BillingScope: "org", SettlementMode: "quota", PricingBehavior: "flat",
	})
	if err != nil {
		t.Fatalf("CreateBillableItem: %v", err)
	}
	gotBI, err := service.GetBillableItem(bi.ID)
	if err != nil {
		t.Fatalf("GetBillableItem: %v", err)
	}
	if gotBI.ID != bi.ID {
		t.Fatalf("expected BI ID %q, got %q", bi.ID, gotBI.ID)
	}
	if _, err := service.GetBillableItem("nonexistent"); err == nil {
		t.Fatalf("expected GetBillableItem not-found error")
	}
}
