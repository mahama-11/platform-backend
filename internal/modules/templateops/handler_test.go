package templateops

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	bindCases := []struct {
		name   string
		fn     func(*gin.Context)
		path   string
		body   []byte
		params gin.Params
	}{
		{"update", handler.UpdateCatalog, "/templateops/catalog/menu:tpl-missing", []byte("{bad"), gin.Params{{Key: "templateRef", Value: "menu:tpl-missing"}}},
		{"import_csv", handler.ImportCSV, "/templateops/csv/import", []byte("{bad"), nil},
		{"preview_csv", handler.PreviewImportCSV, "/templateops/csv/preview", []byte("{bad"), nil},
		{"prepared_assets", handler.ImportPreparedRealAssets, "/templateops/assets/prepared/import", []byte("{bad"), nil},
		{"batch_assets", handler.BatchUploadAssets, "/templateops/assets/batch", []byte("{bad"), nil},
		{"upsert_asset", handler.UpsertTemplateAsset, "/templateops/catalog/menu:tpl-missing/assets", []byte("{bad"), gin.Params{{Key: "templateRef", Value: "menu:tpl-missing"}}},
	}
	for _, tc := range bindCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performTemplateOpsRaw(t, tc.fn, http.MethodPost, tc.path, tc.body, tc.params)
			if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
				t.Fatalf("expected bind error for %s, got %d: %s", tc.name, resp.Code, resp.Body.String())
			}
		})
	}
}

func TestTemplateOpsHandlerManagementSemanticErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:templateops-handler-semantic-errors?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	handler := NewHandler(NewService(config.Config{}, db))

	cases := []struct {
		name      string
		fn        func(*gin.Context)
		method    string
		path      string
		body      []byte
		params    gin.Params
		wantHTTP  int
		wantError string
	}{
		{
			name:      "publish_missing_template",
			fn:        handler.PublishCatalog,
			method:    http.MethodPost,
			path:      "/templateops/catalog/menu:missing/publish",
			params:    gin.Params{{Key: "templateRef", Value: "menu:missing"}},
			wantHTTP:  http.StatusInternalServerError,
			wantError: "TEMPLATE_OPS_PUBLISH_FAILED",
		},
		{
			name:      "get_missing_template",
			fn:        handler.GetDetail,
			method:    http.MethodGet,
			path:      "/templateops/catalog/menu:missing",
			params:    gin.Params{{Key: "templateRef", Value: "menu:missing"}},
			wantHTTP:  http.StatusNotFound,
			wantError: "TEMPLATE_OPS_DETAIL_FAILED",
		},
		{
			name:      "list_assets_missing_template",
			fn:        handler.ListTemplateAssets,
			method:    http.MethodGet,
			path:      "/templateops/catalog/menu:missing/assets",
			params:    gin.Params{{Key: "templateRef", Value: "menu:missing"}},
			wantHTTP:  http.StatusNotFound,
			wantError: "TEMPLATE_OPS_ASSET_LIST_FAILED",
		},
		{
			name:      "unbind_missing_template",
			fn:        handler.UnbindTemplateAsset,
			method:    http.MethodDelete,
			path:      "/templateops/catalog/menu:missing/assets/example_1",
			params:    gin.Params{{Key: "templateRef", Value: "menu:missing"}, {Key: "assetRole", Value: "example_1"}},
			wantHTTP:  http.StatusInternalServerError,
			wantError: "TEMPLATE_OPS_ASSET_UNBIND_FAILED",
		},
		{
			name:      "upsert_missing_template",
			fn:        handler.UpsertTemplateAsset,
			method:    http.MethodPut,
			path:      "/templateops/catalog/menu:missing/assets/example_1",
			body:      mustTemplateOpsJSON(t, UpsertTemplateAssetInput{SourceRef: "templates/missing/example-1", Payload: "dGVzdA=="}),
			params:    gin.Params{{Key: "templateRef", Value: "menu:missing"}, {Key: "assetRole", Value: "example_1"}},
			wantHTTP:  http.StatusInternalServerError,
			wantError: "TEMPLATE_OPS_ASSET_UPSERT_FAILED",
		},
		{
			name:      "csv_import_empty_content",
			fn:        handler.ImportCSV,
			method:    http.MethodPost,
			path:      "/templateops/csv/import",
			body:      mustTemplateOpsJSON(t, CSVImportInput{}),
			wantHTTP:  http.StatusInternalServerError,
			wantError: "TEMPLATE_OPS_CSV_IMPORT_FAILED",
		},
		{
			name:      "csv_preview_missing_required_header",
			fn:        handler.PreviewImportCSV,
			method:    http.MethodPost,
			path:      "/templateops/csv/preview",
			body:      mustTemplateOpsJSON(t, CSVImportPreviewInput{Content: "template_id,name\ntpl,Name\n"}),
			wantHTTP:  http.StatusInternalServerError,
			wantError: "TEMPLATE_OPS_CSV_PREVIEW_FAILED",
		},
		{
			name:      "prepared_bundle_missing_testdata",
			fn:        handler.ExportPreparedRealImportCSV,
			method:    http.MethodGet,
			path:      "/templateops/csv/real-import",
			wantHTTP:  http.StatusInternalServerError,
			wantError: "TEMPLATE_OPS_REAL_IMPORT_LOAD_FAILED",
		},
		{
			name:      "prepared_asset_import_empty_body_defaults_only_missing",
			fn:        handler.ImportPreparedRealAssets,
			method:    http.MethodPost,
			path:      "/templateops/assets/prepared/import",
			body:      nil,
			wantHTTP:  http.StatusInternalServerError,
			wantError: "TEMPLATE_OPS_PREPARED_ASSET_IMPORT_FAILED",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performTemplateOpsRawAny(t, tc.fn, tc.method, tc.path, tc.body, tc.params)
			assertTemplateOpsSemanticError(t, resp, tc.wantHTTP, tc.wantError)
		})
	}
}

func TestTemplateOpsHandlerSyncUpstreamNoSourceAndSemanticFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbNoSource, err := gorm.Open(sqlite.Open("file:templateops-handler-sync-no-source?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite no source: %v", err)
	}
	if err := dbNoSource.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}); err != nil {
		t.Fatalf("auto migrate no source: %v", err)
	}
	noSourceHandler := NewHandler(NewService(config.Config{}, dbNoSource))
	noSourceResp := performTemplateOpsRawAny(t, noSourceHandler.SyncCatalog, http.MethodPost, "/templateops/catalog/sync?product_code=menu", nil, nil)
	if noSourceResp.Code != http.StatusOK || !bytes.Contains(noSourceResp.Body.Bytes(), []byte(`"total":0`)) {
		t.Fatalf("expected sync without a configured source to succeed with empty result, got %d: %s", noSourceResp.Code, noSourceResp.Body.String())
	}

	list404Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer list404Server.Close()
	list404Handler := NewHandler(NewService(templateOpsConfigForEndpoint(list404Server.URL), dbNoSource))
	list404Resp := performTemplateOpsRawAny(t, list404Handler.SyncCatalog, http.MethodPost, "/templateops/catalog/sync?product_code=menu", nil, nil)
	assertTemplateOpsSemanticError(t, list404Resp, http.StatusInternalServerError, "TEMPLATE_OPS_SYNC_FAILED")

	dbFallback, err := gorm.Open(sqlite.Open("file:templateops-handler-sync-fallback?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite fallback: %v", err)
	}
	if err := dbFallback.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}); err != nil {
		t.Fatalf("auto migrate fallback: %v", err)
	}
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/menu/template-center/catalog/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/api/v1/menu/template-center/catalog" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"template_id":"tpl-sync-fallback","slug":"sync-fallback","name":"Sync Fallback","description":"from upstream list","platforms":["xiaohongshu"],"tags":["sync"],"recommend_score":88,"business_goal":"engage","input_slots":[{"name":"image"}],"target_outputs":[{"name":"copy"}],"strategy_policy":{"mode":"safe"}}]}}`))
			return
		}
		t.Fatalf("unexpected upstream path: %s", r.URL.String())
	}))
	defer fallbackServer.Close()
	fallbackHandler := NewHandler(NewService(templateOpsConfigForEndpoint(fallbackServer.URL), dbFallback))
	fallbackResp := performTemplateOpsRawAny(t, fallbackHandler.SyncCatalog, http.MethodPost, "/templateops/catalog/sync?product_code=menu&locale=zh", nil, nil)
	if fallbackResp.Code != http.StatusOK || !bytes.Contains(fallbackResp.Body.Bytes(), []byte("tpl-sync-fallback")) {
		t.Fatalf("expected sync fallback success, got %d: %s", fallbackResp.Code, fallbackResp.Body.String())
	}
	var synced models.TemplateProjection
	if err := dbFallback.Where("template_ref = ?", "menu:tpl-sync-fallback").First(&synced).Error; err != nil {
		t.Fatalf("load synced template: %v", err)
	}
	if !strings.Contains(synced.DetailJSON, "catalog_list_fallback") || !strings.Contains(synced.DetailJSON, "detail_sync_status") {
		t.Fatalf("expected detail fallback marker in synced detail json, got %s", synced.DetailJSON)
	}
	if err := dbFallback.Model(&models.TemplateProjection{}).Where("template_ref = ?", "menu:tpl-sync-fallback").Updates(map[string]any{"publish_status": "", "published_at": nil}).Error; err != nil {
		t.Fatalf("clear synced publish status: %v", err)
	}
	reSyncResp := performTemplateOpsRawAny(t, fallbackHandler.SyncCatalog, http.MethodPost, "/templateops/catalog/sync?product_code=menu&locale=zh", nil, nil)
	if reSyncResp.Code != http.StatusOK || !bytes.Contains(reSyncResp.Body.Bytes(), []byte("tpl-sync-fallback")) {
		t.Fatalf("expected sync update fallback success, got %d: %s", reSyncResp.Code, reSyncResp.Body.String())
	}

	ecommerceDB, err := gorm.Open(sqlite.Open("file:templateops-handler-sync-ecommerce?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite ecommerce: %v", err)
	}
	if err := ecommerceDB.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}); err != nil {
		t.Fatalf("auto migrate ecommerce: %v", err)
	}
	ecommerceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/ecommerce/template-center/catalog":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":"ecom-sync","slug":"ecom-sync","name":"Ecom Sync","summary":"from ecommerce list","modality":"image","executorType":"comfy","series":"series-a","capabilityType":"product-photo","coverAssetUrl":"http://cover/list.png","platformTags":["shop"],"industryTags":["retail"],"scenarioTags":["hero"],"recommendScore":91}]}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/ecommerce/template-center/catalog/"):
			_, _ = w.Write([]byte(`{"code":0,"data":{"catalog":{"name":"Ecom Detail","slug":"ecom-sync","summary":"from ecommerce detail","coverAssetUrl":"http://cover/detail.png","recommendScore":92,"platformTags":["shop"],"series":"series-a","capabilityType":"product-photo","modality":"image"}}}`))
		default:
			t.Fatalf("unexpected ecommerce upstream path: %s", r.URL.String())
		}
	}))
	defer ecommerceServer.Close()
	ecommerceCfg := config.Config{Bootstrap: config.BootstrapConfig{Runtime: config.RuntimeBootstrapConfig{ProductEndpoints: []config.BootstrapRuntimeProductEndpoint{{ProductCode: "ecommerce", BaseURL: ecommerceServer.URL}}}}}
	ecommerceHandler := NewHandler(NewService(ecommerceCfg, ecommerceDB))
	ecommerceResp := performTemplateOpsRawAny(t, ecommerceHandler.SyncCatalog, http.MethodPost, "/templateops/catalog/sync?product_code=ecommerce&locale=en", nil, nil)
	if ecommerceResp.Code != http.StatusOK || !bytes.Contains(ecommerceResp.Body.Bytes(), []byte("ecom-sync")) {
		t.Fatalf("expected ecommerce sync success, got %d: %s", ecommerceResp.Code, ecommerceResp.Body.String())
	}
	var ecommerceSynced models.TemplateProjection
	if err := ecommerceDB.Where("template_ref = ?", "ecommerce:ecom-sync").First(&ecommerceSynced).Error; err != nil {
		t.Fatalf("load ecommerce synced template: %v", err)
	}
	if !strings.Contains(ecommerceSynced.DetailJSON, "Ecom Detail") {
		t.Fatalf("expected ecommerce detail payload to be stored, got %s", ecommerceSynced.DetailJSON)
	}
}

func TestTemplateOpsHandlerCreateUpdateListExportAndBatchSemanticErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:templateops-handler-more-semantic-errors?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	handler := NewHandler(NewService(config.Config{}, db))

	createResp := performTemplateOpsRawAny(t, handler.CreateCatalog, http.MethodPost, "/templateops/catalog", mustTemplateOpsJSON(t, UpsertTemplateInput{}), nil)
	assertTemplateOpsSemanticError(t, createResp, http.StatusInternalServerError, "TEMPLATE_OPS_CREATE_FAILED")

	updateResp := performTemplateOpsRawAny(t, handler.UpdateCatalog, http.MethodPut, "/templateops/catalog/menu:missing", mustTemplateOpsJSON(t, UpsertTemplateInput{Name: "Missing"}), gin.Params{{Key: "templateRef", Value: "menu:missing"}})
	assertTemplateOpsSemanticError(t, updateResp, http.StatusInternalServerError, "TEMPLATE_OPS_UPDATE_FAILED")

	noStorageHandler := NewHandler(NewService(config.Config{}, nil))
	batchResp := performTemplateOpsRawAny(t, noStorageHandler.BatchUploadAssets, http.MethodPost, "/templateops/assets/batch", mustTemplateOpsJSON(t, BatchUploadAssetsInput{Items: []BatchUploadAssetItemInput{{ProductCode: "menu", SourceRef: "source", Payload: "dGVzdA=="}}}), nil)
	assertTemplateOpsSemanticError(t, batchResp, http.StatusInternalServerError, "TEMPLATE_OPS_ASSET_BATCH_UPLOAD_FAILED")

	closedDB, err := gorm.Open(sqlite.Open("file:templateops-handler-closed-db?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open closed sqlite: %v", err)
	}
	if err := closedDB.AutoMigrate(&models.TemplateProjection{}); err != nil {
		t.Fatalf("auto migrate closed db: %v", err)
	}
	closedHandler := NewHandler(NewService(config.Config{}, closedDB))
	sqlDB, err := closedDB.DB()
	if err != nil {
		t.Fatalf("extract sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}
	listResp := performTemplateOpsRawAny(t, closedHandler.ListCatalog, http.MethodGet, "/templateops/catalog?product_code=menu", nil, nil)
	assertTemplateOpsSemanticError(t, listResp, http.StatusInternalServerError, "TEMPLATE_OPS_LIST_FAILED")
	exportResp := performTemplateOpsRawAny(t, closedHandler.ExportCSV, http.MethodGet, "/templateops/csv/export?product_code=menu", nil, nil)
	assertTemplateOpsSemanticError(t, exportResp, http.StatusInternalServerError, "TEMPLATE_OPS_CSV_EXPORT_FAILED")
}

func TestTemplateOpsHandlerAssetUploadUpsertUnbindAndBatchSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:templateops-handler-asset-success?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}, &models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.StorageBinding{ID: "tpl-handler-binding", ProductCode: "menu", Category: "template-examples", ProviderCode: "local", LocalBaseDir: t.TempDir(), Priority: 1, Enabled: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed storage binding: %v", err)
	}
	service := NewService(config.Config{}, db)
	handler := NewHandler(service)
	if _, err := service.CreateTemplate(context.Background(), UpsertTemplateInput{
		ProductCode: "menu",
		TemplateID:  "tpl-asset-success",
		Slug:        "asset-success",
		Name:        "Asset Success",
		DetailRaw: map[string]any{
			"externalCode": "ASSET-SUCCESS",
			"toolBinding":  map[string]any{"toolSlug": "asset-tool"},
			"examples": []any{
				map[string]any{"sourceRef": "templates/asset-success/example-1", "title": "Before"},
			},
		},
	}); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	upsertResp := performTemplateOpsRawAny(t, handler.UpsertTemplateAsset, http.MethodPut, "/templateops/catalog/menu:tpl-asset-success/assets/example_1", mustTemplateOpsJSON(t, UpsertTemplateAssetInput{Payload: "aW1hZ2UtYnl0ZXM=", FileName: "example", MimeType: "image/png", Title: "Uploaded"}), gin.Params{{Key: "templateRef", Value: "menu:tpl-asset-success"}, {Key: "assetRole", Value: "example_1"}})
	if upsertResp.Code != http.StatusOK || !bytes.Contains(upsertResp.Body.Bytes(), []byte(`"status":"ready"`)) {
		t.Fatalf("expected upsert asset success, got %d: %s", upsertResp.Code, upsertResp.Body.String())
	}

	listResp := performTemplateOpsRawAny(t, handler.ListCatalog, http.MethodGet, "/templateops/catalog?query=Asset&tool_slug=asset-tool&limit=5&offset=-10", nil, nil)
	if listResp.Code != http.StatusOK || !bytes.Contains(listResp.Body.Bytes(), []byte("tpl-asset-success")) {
		t.Fatalf("expected filtered list success, got %d: %s", listResp.Code, listResp.Body.String())
	}

	coverPath := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(coverPath, []byte("cover-bytes"), 0o644); err != nil {
		t.Fatalf("write cover source: %v", err)
	}
	coverResp := performTemplateOpsRawAny(t, handler.UpsertTemplateAsset, http.MethodPut, "/templateops/catalog/menu:tpl-asset-success/assets/cover", mustTemplateOpsJSON(t, UpsertTemplateAssetInput{SourcePath: coverPath, StorageFileName: "cover", MimeType: "image/png", Title: "Cover"}), gin.Params{{Key: "templateRef", Value: "menu:tpl-asset-success"}, {Key: "assetRole", Value: "cover"}})
	if coverResp.Code != http.StatusOK {
		t.Fatalf("expected cover source_path upsert success, got %d: %s", coverResp.Code, coverResp.Body.String())
	}

	batchResp := performTemplateOpsRawAny(t, handler.BatchUploadAssets, http.MethodPost, "/templateops/assets/batch", mustTemplateOpsJSON(t, BatchUploadAssetsInput{Items: []BatchUploadAssetItemInput{{ProductCode: "menu", Category: "template-examples", SourceRef: "templates/asset-success/example-2", Payload: "YmF0Y2gtYnl0ZXM=", FileName: "batch.png", MimeType: "image/png", Title: "Batch"}}}), nil)
	if batchResp.Code != http.StatusOK || !bytes.Contains(batchResp.Body.Bytes(), []byte(`"imported_count":1`)) {
		t.Fatalf("expected batch upload success, got %d: %s", batchResp.Code, batchResp.Body.String())
	}

	unbindResp := performTemplateOpsRawAny(t, handler.UnbindTemplateAsset, http.MethodDelete, "/templateops/catalog/menu:tpl-asset-success/assets/example_1", nil, gin.Params{{Key: "templateRef", Value: "menu:tpl-asset-success"}, {Key: "assetRole", Value: "example_1"}})
	if unbindResp.Code != http.StatusOK || !bytes.Contains(unbindResp.Body.Bytes(), []byte(`"items"`)) {
		t.Fatalf("expected unbind asset success, got %d: %s", unbindResp.Code, unbindResp.Body.String())
	}
}

func TestTemplateOpsHandlerCSVAndBatchDegradedSuccessResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:templateops-handler-degraded-success?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}, &models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	handler := NewHandler(NewService(config.Config{}, db))
	badCSV := "product_code,template_id,name,recommend_score,platforms_json,tags_json,raw_json,detail_json\n" +
		"menu,tpl-bad,Bad,not-a-number,not-json,[],{},{}\n"
	importResp := performTemplateOpsRawAny(t, handler.ImportCSV, http.MethodPost, "/templateops/csv/import", mustTemplateOpsJSON(t, CSVImportInput{Content: badCSV}), nil)
	if importResp.Code != http.StatusOK || !bytes.Contains(importResp.Body.Bytes(), []byte(`"action":"invalid"`)) || !bytes.Contains(importResp.Body.Bytes(), []byte("invalid recommend_score")) {
		t.Fatalf("expected csv import invalid row result, got %d: %s", importResp.Code, importResp.Body.String())
	}
	previewResp := performTemplateOpsRawAny(t, handler.PreviewImportCSV, http.MethodPost, "/templateops/csv/preview", mustTemplateOpsJSON(t, CSVImportPreviewInput{Content: badCSV}), nil)
	if previewResp.Code != http.StatusOK || !bytes.Contains(previewResp.Body.Bytes(), []byte(`"invalid_rows":1`)) || !bytes.Contains(previewResp.Body.Bytes(), []byte("invalid recommend_score")) {
		t.Fatalf("expected csv preview invalid row result, got %d: %s", previewResp.Code, previewResp.Body.String())
	}

	validCSV := "product_code,template_id,name,summary\nmenu,tpl-update,Update Me,first\n"
	createCSVResp := performTemplateOpsRawAny(t, handler.ImportCSV, http.MethodPost, "/templateops/csv/import", mustTemplateOpsJSON(t, CSVImportInput{Content: validCSV}), nil)
	if createCSVResp.Code != http.StatusOK || !bytes.Contains(createCSVResp.Body.Bytes(), []byte(`"action":"created"`)) {
		t.Fatalf("expected csv create row result, got %d: %s", createCSVResp.Code, createCSVResp.Body.String())
	}
	updateCSVResp := performTemplateOpsRawAny(t, handler.ImportCSV, http.MethodPost, "/templateops/csv/import", mustTemplateOpsJSON(t, CSVImportInput{Content: strings.Replace(validCSV, "first", "second", 1)}), nil)
	if updateCSVResp.Code != http.StatusOK || !bytes.Contains(updateCSVResp.Body.Bytes(), []byte(`"action":"updated"`)) {
		t.Fatalf("expected csv update row result, got %d: %s", updateCSVResp.Code, updateCSVResp.Body.String())
	}
	previewUpdateResp := performTemplateOpsRawAny(t, handler.PreviewImportCSV, http.MethodPost, "/templateops/csv/preview", mustTemplateOpsJSON(t, CSVImportPreviewInput{Content: validCSV}), nil)
	if previewUpdateResp.Code != http.StatusOK || !bytes.Contains(previewUpdateResp.Body.Bytes(), []byte(`"action":"update"`)) {
		t.Fatalf("expected csv preview update action, got %d: %s", previewUpdateResp.Code, previewUpdateResp.Body.String())
	}

	batchInvalidResp := performTemplateOpsRawAny(t, handler.BatchUploadAssets, http.MethodPost, "/templateops/assets/batch", mustTemplateOpsJSON(t, BatchUploadAssetsInput{Items: []BatchUploadAssetItemInput{{ProductCode: "", SourceRef: "", Payload: ""}}}), nil)
	if batchInvalidResp.Code != http.StatusOK || !bytes.Contains(batchInvalidResp.Body.Bytes(), []byte(`"failed_count":1`)) || !bytes.Contains(batchInvalidResp.Body.Bytes(), []byte("product_code, source_ref, and payload are required")) {
		t.Fatalf("expected batch validation failure item, got %d: %s", batchInvalidResp.Code, batchInvalidResp.Body.String())
	}

	batchUploadErrResp := performTemplateOpsRawAny(t, handler.BatchUploadAssets, http.MethodPost, "/templateops/assets/batch", mustTemplateOpsJSON(t, BatchUploadAssetsInput{Items: []BatchUploadAssetItemInput{{ProductCode: "menu", Category: "template-examples", SourceRef: "templates/no-binding/example", Payload: "bm8tYmluZGluZw==", FileName: "no-binding.png", MimeType: "image/png"}}}), nil)
	if batchUploadErrResp.Code != http.StatusOK || !bytes.Contains(batchUploadErrResp.Body.Bytes(), []byte(`"failed_count":1`)) || !bytes.Contains(batchUploadErrResp.Body.Bytes(), []byte("storage binding")) {
		t.Fatalf("expected batch upload failure item, got %d: %s", batchUploadErrResp.Code, batchUploadErrResp.Body.String())
	}
}

func TestTemplateOpsHandlerPreparedRealBundleSuccessAndImportResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bundleDir := filepath.Join("testdata", "templateops", "real-import")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("create prepared bundle dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join("testdata", "templateops")) })

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "prepared.png")
	if err := os.WriteFile(sourcePath, []byte("prepared-image-bytes"), 0o644); err != nil {
		t.Fatalf("write prepared source: %v", err)
	}
	missingPath := filepath.Join(sourceDir, "missing.png")
	csvPath := filepath.Join(bundleDir, "template_ops_real_import.csv")
	manifestPath := filepath.Join(bundleDir, "template_ops_real_asset_manifest.json")
	summaryPath := filepath.Join(bundleDir, "template_ops_real_import_summary.json")
	if err := os.WriteFile(csvPath, []byte("product_code,template_id,name\nmenu,prepared,Prepared\n"), 0o644); err != nil {
		t.Fatalf("write prepared csv: %v", err)
	}
	manifest := map[string]any{"items": []map[string]any{
		{"productCode": "menu", "category": "template-examples", "sourceType": "template_example", "sourceRef": "prepared/source-1", "storageFileName": "prepared.png", "sourcePath": sourcePath, "title": "Prepared"},
		{"productCode": "menu", "category": "template-examples", "sourceType": "template_example", "sourceRef": "prepared/missing", "storageFileName": "missing.png", "sourcePath": missingPath, "title": "Missing"},
	}}
	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestBody, 0o644); err != nil {
		t.Fatalf("write prepared manifest: %v", err)
	}
	if err := os.WriteFile(summaryPath, []byte(`{"templateCount":1,"menuTemplateCount":1,"ecommerceTemplateCount":0,"assetManifestItemCount":2,"missingAssetCount":1}`), 0o644); err != nil {
		t.Fatalf("write prepared summary: %v", err)
	}

	db, err := gorm.Open(sqlite.Open("file:templateops-handler-prepared-success?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}, &models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.StorageBinding{ID: "tpl-prepared-binding", ProductCode: "menu", Category: "template-examples", ProviderCode: "local", LocalBaseDir: t.TempDir(), Priority: 1, Enabled: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed storage binding: %v", err)
	}
	handler := NewHandler(NewService(config.Config{}, db))

	bundleResp := performTemplateOpsRawAny(t, handler.ExportPreparedRealImportCSV, http.MethodGet, "/templateops/csv/real-import", nil, nil)
	if bundleResp.Code != http.StatusOK || !bytes.Contains(bundleResp.Body.Bytes(), []byte(`"asset_manifest_item_count":2`)) {
		t.Fatalf("expected prepared bundle success, got %d: %s", bundleResp.Code, bundleResp.Body.String())
	}

	importResp := performTemplateOpsRawAny(t, handler.ImportPreparedRealAssets, http.MethodPost, "/templateops/assets/prepared/import", mustTemplateOpsJSON(t, PreparedAssetImportInput{OnlyMissing: false}), nil)
	if importResp.Code != http.StatusOK || !bytes.Contains(importResp.Body.Bytes(), []byte(`"imported_count":1`)) || !bytes.Contains(importResp.Body.Bytes(), []byte(`"failed_count":1`)) {
		t.Fatalf("expected prepared import mixed result, got %d: %s", importResp.Code, importResp.Body.String())
	}

	eofResp := performTemplateOpsRawAny(t, handler.ImportPreparedRealAssets, http.MethodPost, "/templateops/assets/prepared/import", nil, nil)
	if eofResp.Code != http.StatusOK || !bytes.Contains(eofResp.Body.Bytes(), []byte(`"skipped_count":1`)) || !bytes.Contains(eofResp.Body.Bytes(), []byte(`"failed_count":1`)) {
		t.Fatalf("expected prepared import EOF path to default to only_missing and skip existing asset, got %d: %s", eofResp.Code, eofResp.Body.String())
	}
}

func performTemplateOpsJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performTemplateOpsRaw(t, fn, method, path, payload, params)
}

func mustTemplateOpsJSON(t *testing.T, body any) []byte {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal templateops test body: %v", err)
	}
	return payload
}

func performTemplateOpsRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	w := performTemplateOpsRawAny(t, fn, method, path, body, params)
	if w.Code >= 500 {
		t.Fatalf("unexpected templateops handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func performTemplateOpsRawAny(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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

func assertTemplateOpsSemanticError(t *testing.T, resp *httptest.ResponseRecorder, wantHTTP int, wantErrorCode string) {
	t.Helper()
	if resp.Code != wantHTTP {
		t.Fatalf("expected HTTP %d, got %d: %s", wantHTTP, resp.Code, resp.Body.String())
	}
	var envelope struct {
		Error     string `json:"error"`
		ErrorCode string `json:"error_code"`
		ErrorHint string `json:"error_hint"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode semantic error response: %v; body=%s", err, resp.Body.String())
	}
	if envelope.Error == "" || envelope.ErrorCode != wantErrorCode || envelope.ErrorHint == "" {
		t.Fatalf("expected semantic error_code=%s with error and hint, got %+v body=%s", wantErrorCode, envelope, resp.Body.String())
	}
}

func templateOpsConfigForEndpoint(baseURL string) config.Config {
	return config.Config{Bootstrap: config.BootstrapConfig{Runtime: config.RuntimeBootstrapConfig{ProductEndpoints: []config.BootstrapRuntimeProductEndpoint{{ProductCode: "menu", BaseURL: baseURL}}}}}
}
