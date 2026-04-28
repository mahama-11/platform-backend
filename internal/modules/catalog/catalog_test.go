package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
