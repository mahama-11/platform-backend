package templateops

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTemplateOpsHandlerCatalogCSVAndAssetPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:templateops-handler?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	service := NewService(config.Config{}, db)
	handler := NewHandler(service)

	createResp := performTemplateOpsJSON(t, handler.CreateCatalog, http.MethodPost, "/templateops/catalog", UpsertTemplateInput{ProductCode: "menu", TemplateID: "tpl-handler-1", Slug: "handler-template", Name: "Handler Template", Summary: "summary", Status: "draft", Scope: "official", ManagedSource: "handler_test", RecommendScore: 42, Tags: []string{"ops"}, Platforms: []string{"xiaohongshu"}, Raw: map[string]any{"k": "v"}, DetailRaw: map[string]any{"examples": []any{map[string]any{"sourceRef": "handler/source-1"}}}}, nil)
	if createResp.Code != http.StatusCreated || !bytes.Contains(createResp.Body.Bytes(), []byte("menu:tpl-handler-1")) {
		t.Fatalf("expected create catalog success, got %d: %s", createResp.Code, createResp.Body.String())
	}
	performTemplateOpsJSON(t, handler.UpdateCatalog, http.MethodPut, "/templateops/catalog/menu:tpl-handler-1", UpsertTemplateInput{Name: "Handler Template Updated", Status: "draft", RecommendScore: 50, DetailRaw: map[string]any{"examples": []any{map[string]any{"sourceRef": "handler/source-1"}}}}, gin.Params{{Key: "templateRef", Value: "menu:tpl-handler-1"}})
	performTemplateOpsRaw(t, handler.PublishCatalog, http.MethodPost, "/templateops/catalog/menu:tpl-handler-1/publish", nil, gin.Params{{Key: "templateRef", Value: "menu:tpl-handler-1"}})
	listResp := performTemplateOpsRaw(t, handler.ListCatalog, http.MethodGet, "/templateops/catalog?product_code=menu&limit=10&offset=0&published_only=true", nil, nil)
	if listResp.Code != http.StatusOK || !bytes.Contains(listResp.Body.Bytes(), []byte("Handler Template Updated")) {
		t.Fatalf("expected list catalog success, got %d: %s", listResp.Code, listResp.Body.String())
	}
	getResp := performTemplateOpsRaw(t, handler.GetDetail, http.MethodGet, "/templateops/catalog/menu:tpl-handler-1?locale=zh", nil, gin.Params{{Key: "templateRef", Value: "menu:tpl-handler-1"}})
	if getResp.Code != http.StatusOK || !bytes.Contains(getResp.Body.Bytes(), []byte("tpl-handler-1")) {
		t.Fatalf("expected get detail success, got %d: %s", getResp.Code, getResp.Body.String())
	}

	csvContent := "product_code,template_id,slug,name,summary,status,scope,managed_source,cover_asset_url,cover_asset_id,recommend_score,platforms_json,tags_json,series,capability_type,modality,raw_json,detail_json\n" +
		"menu,tpl-handler-csv,handler-csv,CSV Template,from csv,active,official,handler_test,,,77,\"[\"\"xiaohongshu\"\"]\",\"[\"\"ops\"\"]\",,,,\"{}\",\"{}\"\n"
	previewResp := performTemplateOpsJSON(t, handler.PreviewImportCSV, http.MethodPost, "/templateops/csv/preview", CSVImportPreviewInput{Content: csvContent}, nil)
	if previewResp.Code != http.StatusOK || !bytes.Contains(previewResp.Body.Bytes(), []byte(`"total_rows":1`)) {
		t.Fatalf("expected csv preview success, got %d: %s", previewResp.Code, previewResp.Body.String())
	}
	importResp := performTemplateOpsJSON(t, handler.ImportCSV, http.MethodPost, "/templateops/csv/import", CSVImportInput{Content: csvContent, Publish: true}, nil)
	if importResp.Code != http.StatusOK || !bytes.Contains(importResp.Body.Bytes(), []byte(`"imported_count":1`)) {
		t.Fatalf("expected csv import success, got %d: %s", importResp.Code, importResp.Body.String())
	}
	exportResp := performTemplateOpsRaw(t, handler.ExportCSV, http.MethodGet, "/templateops/csv/export?product_code=menu&limit=20&offset=0", nil, nil)
	if exportResp.Code != http.StatusOK || !bytes.Contains(exportResp.Body.Bytes(), []byte("tpl-handler-csv")) {
		t.Fatalf("expected csv export success, got %d: %s", exportResp.Code, exportResp.Body.String())
	}
	templateResp := performTemplateOpsRaw(t, handler.ExportCSVTemplate, http.MethodGet, "/templateops/csv/template", nil, nil)
	if templateResp.Code != http.StatusOK || !bytes.Contains(templateResp.Body.Bytes(), []byte("product_code")) {
		t.Fatalf("expected csv template success, got %d: %s", templateResp.Code, templateResp.Body.String())
	}

	now := time.Now()
	if err := db.Create(&models.StorageAsset{ID: "tpl-handler-asset", ProductCode: "menu", Category: "template-examples", SourceType: "template_example", SourceRef: "handler/source-1", StorageKey: "menu/template-examples/source-1.png", Status: "active", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed storage asset: %v", err)
	}
	assetList := performTemplateOpsRaw(t, handler.ListTemplateAssets, http.MethodGet, "/templateops/catalog/menu:tpl-handler-1/assets", nil, gin.Params{{Key: "templateRef", Value: "menu:tpl-handler-1"}})
	if assetList.Code != http.StatusOK || !bytes.Contains(assetList.Body.Bytes(), []byte("tpl-handler-asset")) {
		t.Fatalf("expected template asset list success, got %d: %s", assetList.Code, assetList.Body.String())
	}
}

func TestTemplateOpsHandlerBindErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:templateops-handler-errors?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	handler := NewHandler(NewService(config.Config{}, db))
	resp := performTemplateOpsRaw(t, handler.CreateCatalog, http.MethodPost, "/templateops/catalog", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated || resp.Code == http.StatusOK {
		t.Fatalf("expected create bind error")
	}
	resp = performTemplateOpsRaw(t, handler.ImportCSV, http.MethodPost, "/templateops/csv/import", []byte("{bad"), nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected csv import bind error")
	}
}

func performTemplateOpsJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performTemplateOpsRaw(t, fn, method, path, payload, params)
}

func performTemplateOpsRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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
		t.Fatalf("unexpected templateops handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}
