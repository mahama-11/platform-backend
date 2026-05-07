package templateops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTemplateOpsService_ListCatalogAndDetail(t *testing.T) {
	menuServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/template-center/catalog":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"items": []map[string]any{
						{
							"template_id":     "menu-tpl-1",
							"slug":            "winter-dish",
							"name":            "Winter Dish",
							"description":     "menu summary",
							"platforms":       []string{"xiaohongshu"},
							"tags":            []string{"winter", "dish"},
							"recommend_score": 90,
							"cover_asset_id":  "asset-1",
							"cuisine":         "sichuan",
							"dish_type":       "main",
							"moods":           []string{"warm"},
							"plan_required":   "pro",
						},
					},
				},
			})
		case "/api/v1/template-center/catalog/menu-tpl-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"template_id":     "menu-tpl-1",
					"slug":            "winter-dish",
					"name":            "Winter Dish",
					"description":     "menu detail",
					"platforms":       []string{"xiaohongshu"},
					"tags":            []string{"winter"},
					"recommend_score": 91,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer menuServer.Close()

	ecomServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/ecommerce/template-center/catalog":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": []map[string]any{
					{
						"id":             "ecom-tpl-1",
						"slug":           "hero-banner",
						"name":           "Hero Banner",
						"summary":        "ecom summary",
						"modality":       "image",
						"executorType":   "tool",
						"series":         "banner",
						"capabilityType": "design",
						"coverAssetUrl":  "https://cdn.example/banner.png",
						"platformTags":   []string{"amazon"},
						"industryTags":   []string{"retail"},
						"scenarioTags":   []string{"hero"},
						"recommendScore": 88,
					},
				},
			})
		case "/api/v1/ecommerce/template-center/catalog/ecom-tpl-1":
			// The live Ecommerce seed can list templates before every detail row has a
			// complete version/schema. Platform sync should still converge the visible
			// projection from the list payload instead of failing the whole endpoint.
			response := map[string]any{
				"code":       1004,
				"message":    "Resource not found",
				"error":      "template not found",
				"error_code": "TEMPLATE_NOT_FOUND",
			}
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(response)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ecomServer.Close()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	service := NewService(config.Config{
		Bootstrap: config.BootstrapConfig{
			Runtime: config.RuntimeBootstrapConfig{
				ProductEndpoints: []config.BootstrapRuntimeProductEndpoint{
					{ProductCode: "menu", BaseURL: menuServer.URL},
					{ProductCode: "ecommerce", BaseURL: ecomServer.URL},
				},
			},
		},
	}, db)

	if _, err := service.SyncFromUpstream(context.Background(), "", "zh"); err != nil {
		t.Fatalf("SyncFromUpstream err=%v", err)
	}

	result, err := service.ListCatalog(context.Background(), ListCatalogInput{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("ListCatalog err=%v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected total 2, got %d", result.Total)
	}
	if result.Items[0].TemplateRef == "" {
		t.Fatalf("expected template ref")
	}

	detail, err := service.GetDetail(context.Background(), "menu:menu-tpl-1", "zh")
	if err != nil {
		t.Fatalf("GetDetail(menu) err=%v", err)
	}
	if detail.Item.ProductCode != "menu" {
		t.Fatalf("expected menu product code, got %s", detail.Item.ProductCode)
	}

	ecomDetail, err := service.GetDetail(context.Background(), "ecommerce:ecom-tpl-1", "zh")
	if err != nil {
		t.Fatalf("GetDetail(ecommerce) err=%v", err)
	}
	if ecomDetail.Item.ProductCode != "ecommerce" {
		t.Fatalf("expected ecommerce product code, got %s", ecomDetail.Item.ProductCode)
	}

	exported, err := service.ExportCSV(context.Background(), ListCatalogInput{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("ExportCSV err=%v", err)
	}
	if exported == "" {
		t.Fatalf("expected exported csv")
	}

	imported, err := service.ImportCSV(context.Background(), CSVImportInput{
		Content: "product_code,template_id,slug,name,summary,status,scope,managed_source,cover_asset_url,cover_asset_id,recommend_score,platforms_json,tags_json,series,capability_type,modality,raw_json,detail_json\nmenu,TPL-OPS-NEW,ops-new,Ops New,from csv,active,official,ops_manual,,,77,\"[\"\"xiaohongshu\"\"]\",\"[\"\"ops\"\"]\",,,,\"{\"\"cuisine\"\":\"\"fusion\"\"}\",\"{\"\"prompt_templates\"\":{\"\"hero\"\":\"\"hello\"\"}}\"\n",
		Publish: true,
	})
	if err != nil {
		t.Fatalf("ImportCSV err=%v", err)
	}
	if imported.ImportedCount != 1 {
		t.Fatalf("expected imported_count 1, got %d", imported.ImportedCount)
	}
	if imported.PublishedCount != 1 {
		t.Fatalf("expected published_count 1, got %d", imported.PublishedCount)
	}
}

func TestTemplateOpsService_PreviewImportCSV(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.StorageAsset{
		ID:          "asset_1",
		ProductCode: "ecommerce",
		Category:    "template-examples",
		SourceType:  "template_example",
		SourceRef:   "templates/changing-model/M1-T01/example-1",
		StorageKey:  "ecommerce/template-examples/m1-t01.png",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed storage asset: %v", err)
	}
	service := NewService(config.Config{}, db)
	csvContent := "product_code,template_id,slug,name,summary,status,scope,managed_source,cover_asset_url,cover_asset_id,recommend_score,platforms_json,tags_json,series,capability_type,modality,raw_json,detail_json\n" +
		"ecommerce,tpl_m1_t01,changing-model-m1-t01-template,欧美白人女模特,summary,active,official,seed_import,,,10,\"[\"\"amazon\"\"]\",\"[\"\"fashion\"\"]\",model_image,model_swap,image,\"{}\",\"{\"\"examples\"\": [{\"\"sourceRef\"\":\"\"templates/changing-model/M1-T01/example-1\"\"}], \"\"toolBinding\"\": {\"\"toolSlug\"\":\"\"changing-model\"\"}}\"\n" +
		"ecommerce,tpl_m1_t02,changing-model-m1-t02-template,亚洲女模特,summary,active,official,seed_import,,,10,\"[\"\"amazon\"\"]\",\"[\"\"fashion\"\"]\",model_image,model_swap,image,\"{}\",\"{\"\"examples\"\": [{\"\"sourceRef\"\":\"\"templates/changing-model/M1-T02/example-1\"\"}], \"\"toolBinding\"\": {\"\"toolSlug\"\":\"\"changing-model\"\"}}\"\n"

	preview, err := service.PreviewImportCSV(context.Background(), CSVImportPreviewInput{Content: csvContent})
	if err != nil {
		t.Fatalf("PreviewImportCSV err=%v", err)
	}
	if preview.Summary.TotalRows != 2 {
		t.Fatalf("expected total_rows 2, got %d", preview.Summary.TotalRows)
	}
	if preview.Summary.ReadyToImportCount != 1 {
		t.Fatalf("expected ready_to_import_count 1, got %d", preview.Summary.ReadyToImportCount)
	}
	if preview.Summary.MissingAssetCount != 1 {
		t.Fatalf("expected missing_asset_count 1, got %d", preview.Summary.MissingAssetCount)
	}
	if !preview.Rows[0].ReadyToImport {
		t.Fatalf("expected first row ready to import")
	}
	if preview.Rows[1].ReadyToImport {
		t.Fatalf("expected second row not ready to import")
	}
}

func TestTemplateOpsService_LoadPreparedRealImportBundle(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tempDir := t.TempDir()
	testdataDir := filepath.Join(tempDir, "testdata", "templateops", "real-import")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir tempDir: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	csvPath := filepath.Join(testdataDir, "template_ops_real_import.csv")
	if err := os.WriteFile(csvPath, []byte("product_code,template_id,name\nmenu,tpl-1,Test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile csv: %v", err)
	}
	summaryPath := filepath.Join(testdataDir, "template_ops_real_import_summary.json")
	summaryBody := []byte(`{"templateCount":1,"menuTemplateCount":1,"ecommerceTemplateCount":0,"assetManifestItemCount":0,"missingAssetCount":0}`)
	if err := os.WriteFile(summaryPath, summaryBody, 0o644); err != nil {
		t.Fatalf("WriteFile summary: %v", err)
	}
	manifestPath := filepath.Join(testdataDir, "template_ops_real_asset_manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"version":1,"items":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}

	service := NewService(config.Config{}, nil)
	bundle, err := service.LoadPreparedRealImportBundle()
	if err != nil {
		t.Fatalf("LoadPreparedRealImportBundle err=%v", err)
	}
	if bundle.TemplateCount != 1 {
		t.Fatalf("expected template count 1, got %d", bundle.TemplateCount)
	}
	if bundle.Content == "" {
		t.Fatalf("expected bundle content")
	}
}

func TestTemplateOpsService_UpsertAndUnbindTemplateAsset(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}, &models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	storageRoot := t.TempDir()
	if err := db.Create(&models.StorageBinding{
		ID:           "binding_prepared_import",
		ProductCode:  "ecommerce",
		Category:     "template-examples",
		ProviderCode: "local",
		LocalBaseDir: storageRoot,
		Priority:     1,
		Enabled:      true,
	}).Error; err != nil {
		t.Fatalf("seed storage binding: %v", err)
	}
	service := NewService(config.Config{}, db)
	_, err = service.CreateTemplate(context.Background(), UpsertTemplateInput{
		ProductCode:    "ecommerce",
		TemplateID:     "tpl_m1_t01",
		Slug:           "changing-model-tpl-m1-t01",
		Name:           "Changing Model",
		Summary:        "asset binding test",
		Status:         "active",
		Scope:          "official",
		ManagedSource:  "ops_manual",
		RecommendScore: 10,
		Platforms:      []string{"amazon"},
		Tags:           []string{"fashion"},
		Series:         "model_image",
		CapabilityType: "model_swap",
		Modality:       "image",
		Raw:            map[string]any{},
		DetailRaw: map[string]any{
			"id":           "tpl_m1_t01",
			"externalCode": "M1-T01",
			"toolBinding": map[string]any{
				"toolSlug": "changing-model",
			},
			"examples": []any{},
		},
	})
	if err != nil {
		t.Fatalf("CreateTemplate err=%v", err)
	}

	result, err := service.UpsertTemplateAsset(context.Background(), "ecommerce:tpl_m1_t01", UpsertTemplateAssetInput{
		AssetRole: "example_1",
		Title:     "Example One",
		FileName:  "example-1.png",
		MimeType:  "image/png",
		Payload:   "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4//8/AwAI/AL+X2HFNwAAAABJRU5ErkJggg==",
	})
	if err != nil {
		t.Fatalf("UpsertTemplateAsset err=%v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 asset binding, got %d", len(result.Items))
	}
	if result.Items[0].SourceRef != "templates/changing-model/M1-T01/example-1" {
		t.Fatalf("unexpected source_ref %s", result.Items[0].SourceRef)
	}
	if result.Items[0].Status != "ready" {
		t.Fatalf("expected asset status ready, got %s", result.Items[0].Status)
	}

	detail, err := service.GetDetail(context.Background(), "ecommerce:tpl_m1_t01", "zh")
	if err != nil {
		t.Fatalf("GetDetail err=%v", err)
	}
	examples, _ := detail.DetailRaw["examples"].([]any)
	if len(examples) != 1 {
		t.Fatalf("expected 1 example after upsert, got %d", len(examples))
	}

	unbound, err := service.UnbindTemplateAsset(context.Background(), "ecommerce:tpl_m1_t01", "example_1")
	if err != nil {
		t.Fatalf("UnbindTemplateAsset err=%v", err)
	}
	if len(unbound.Items) != 0 {
		t.Fatalf("expected no asset bindings after unbind, got %d", len(unbound.Items))
	}
}

func TestTemplateOpsService_ImportPreparedRealAssets(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tempDir := t.TempDir()
	storageRoot := filepath.Join(tempDir, "storage")
	if err := os.MkdirAll(storageRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll storage: %v", err)
	}
	testdataDir := filepath.Join(tempDir, "testdata", "templateops", "real-import")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll testdata: %v", err)
	}
	sourcePath := filepath.Join(tempDir, "example.png")
	payload, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4//8/AwAI/AL+X2HFNwAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("DecodeString payload: %v", err)
	}
	if err := os.WriteFile(sourcePath, payload, 0o644); err != nil {
		t.Fatalf("WriteFile source asset: %v", err)
	}
	manifestBody := []byte(`{"items":[{"productCode":"ecommerce","category":"template-examples","sourceType":"template_example","sourceRef":"templates/changing-model/M1-T01/example-1","assetRef":"infra/examples/example.png","storageFileName":"changing-model/m1-t01-example-1.png","title":"Example One","description":"prepared import","sourcePath":"` + sourcePath + `","tags":["template-example"],"metadata":{"templateID":"tpl_m1_t01"}}]}`)
	if err := os.WriteFile(filepath.Join(testdataDir, "template_ops_real_asset_manifest.json"), manifestBody, 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir tempDir: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TemplateProjection{}, &models.StorageAsset{}, &models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&models.StorageBinding{
		ID:           "binding_1",
		ProductCode:  "ecommerce",
		Category:     "template-examples",
		ProviderCode: "local",
		LocalBaseDir: storageRoot,
		Priority:     1,
		Enabled:      true,
	}).Error; err != nil {
		t.Fatalf("seed storage binding: %v", err)
	}
	service := NewService(config.Config{}, db)
	result, err := service.ImportPreparedRealAssets(context.Background(), PreparedAssetImportInput{OnlyMissing: true})
	if err != nil {
		t.Fatalf("ImportPreparedRealAssets err=%v", err)
	}
	if result.ImportedCount+result.SkippedCount != 1 {
		t.Fatalf("expected exactly one prepared asset to be handled, got imported=%d skipped=%d", result.ImportedCount, result.SkippedCount)
	}
	if result.FailedCount != 0 {
		t.Fatalf("expected failed_count 0, got %d", result.FailedCount)
	}

	second, err := service.ImportPreparedRealAssets(context.Background(), PreparedAssetImportInput{OnlyMissing: true})
	if err != nil {
		t.Fatalf("ImportPreparedRealAssets second err=%v", err)
	}
	if second.SkippedCount != 1 {
		t.Fatalf("expected skipped_count 1 on second import, got %d", second.SkippedCount)
	}
}

func TestTemplateOpsService_BatchUploadAssets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.StorageAsset{}, &models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	storageRoot := t.TempDir()
	if err := db.Create(&models.StorageBinding{
		ID:           "binding_batch_upload",
		ProductCode:  "ecommerce",
		Category:     "template-examples",
		ProviderCode: "local",
		LocalBaseDir: storageRoot,
		Priority:     1,
		Enabled:      true,
	}).Error; err != nil {
		t.Fatalf("seed storage binding: %v", err)
	}
	service := NewService(config.Config{}, db)
	result, err := service.BatchUploadAssets(context.Background(), BatchUploadAssetsInput{
		Items: []BatchUploadAssetItemInput{
			{
				ProductCode: "ecommerce",
				Category:    "template-examples",
				SourceType:  "template_example",
				SourceRef:   "templates/changing-model/M1-T09/example-1",
				FileName:    "m1-t09-example-1.png",
				MimeType:    "image/png",
				Payload:     "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4//8/AwAI/AL+X2HFNwAAAABJRU5ErkJggg==",
				Title:       "Example Nine",
			},
		},
	})
	if err != nil {
		t.Fatalf("BatchUploadAssets err=%v", err)
	}
	if result.ImportedCount != 1 {
		t.Fatalf("expected imported_count 1, got %d", result.ImportedCount)
	}
	if result.FailedCount != 0 {
		t.Fatalf("expected failed_count 0, got %d", result.FailedCount)
	}
	if result.Items[0].StorageKey == "" {
		t.Fatalf("expected storage key after batch upload")
	}
}
