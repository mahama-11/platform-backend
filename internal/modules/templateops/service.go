package templateops

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	assetstorage "platform-service/internal/modules/assetstorage"
	"platform-service/internal/repository"

	"gorm.io/gorm"
)

type Service struct {
	client       *http.Client
	db           *gorm.DB
	sources      map[string]templateSource
	assetStorage *assetstorage.Service
}

type storageAssetFinder interface {
	FindStorageAssetBySource(productCode, category, sourceType, sourceRef string) (*models.StorageAsset, error)
}

type templateSource struct {
	ProductCode string
	BaseURL     string
}

type ListCatalogInput struct {
	ProductCode   string
	Query         string
	Locale        string
	ToolSlug      string
	Limit         int
	Offset        int
	PublishedOnly bool
}

type TemplateCatalogItem struct {
	TemplateRef    string         `json:"template_ref"`
	ProductCode    string         `json:"product_code"`
	TemplateID     string         `json:"template_id"`
	Slug           string         `json:"slug"`
	Name           string         `json:"name"`
	Summary        string         `json:"summary"`
	Status         string         `json:"status"`
	CoverAssetURL  string         `json:"cover_asset_url,omitempty"`
	CoverAssetID   string         `json:"cover_asset_id,omitempty"`
	RecommendScore int            `json:"recommend_score"`
	Tags           []string       `json:"tags,omitempty"`
	Platforms      []string       `json:"platforms,omitempty"`
	Series         string         `json:"series,omitempty"`
	CapabilityType string         `json:"capability_type,omitempty"`
	Modality       string         `json:"modality,omitempty"`
	Scope          string         `json:"scope,omitempty"`
	ManagedSource  string         `json:"managed_source,omitempty"`
	Raw            map[string]any `json:"raw,omitempty"`
}

type TemplateCatalogListResult struct {
	Items  []TemplateCatalogItem `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

type TemplateCatalogDetail struct {
	Item      TemplateCatalogItem `json:"item"`
	Product   string              `json:"product"`
	DetailRaw map[string]any      `json:"detail_raw"`
}

type TemplateAssetBinding struct {
	AssetRole    string `json:"asset_role"`
	ProductCode  string `json:"product_code"`
	Category     string `json:"category"`
	SourceType   string `json:"source_type"`
	SourceRef    string `json:"source_ref"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	AssetRef     string `json:"asset_ref,omitempty"`
	StorageKey   string `json:"storage_key,omitempty"`
	AssetID      string `json:"asset_id,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	Checksum     string `json:"checksum,omitempty"`
	PreviewURL   string `json:"preview_url,omitempty"`
	ExampleIndex int    `json:"example_index,omitempty"`
	Status       string `json:"status"`
}

type TemplateAssetBindingsResult struct {
	TemplateRef string                 `json:"template_ref"`
	Items       []TemplateAssetBinding `json:"items"`
}

type UpsertTemplateAssetInput struct {
	AssetRole       string         `json:"asset_role"`
	SourceRef       string         `json:"source_ref"`
	SourceType      string         `json:"source_type"`
	Category        string         `json:"category"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	AssetRef        string         `json:"asset_ref"`
	StorageFileName string         `json:"storage_file_name"`
	SourcePath      string         `json:"source_path"`
	Payload         string         `json:"payload"`
	FileName        string         `json:"file_name"`
	MimeType        string         `json:"mime_type"`
	Tags            []string       `json:"tags"`
	Metadata        map[string]any `json:"metadata"`
	Status          string         `json:"status"`
}

type UpsertTemplateInput struct {
	ProductCode    string         `json:"product_code"`
	TemplateID     string         `json:"template_id"`
	Slug           string         `json:"slug"`
	Name           string         `json:"name"`
	Summary        string         `json:"summary"`
	Status         string         `json:"status"`
	Scope          string         `json:"scope"`
	ManagedSource  string         `json:"managed_source"`
	CoverAssetURL  string         `json:"cover_asset_url"`
	CoverAssetID   string         `json:"cover_asset_id"`
	RecommendScore int            `json:"recommend_score"`
	Tags           []string       `json:"tags"`
	Platforms      []string       `json:"platforms"`
	Series         string         `json:"series"`
	CapabilityType string         `json:"capability_type"`
	Modality       string         `json:"modality"`
	Raw            map[string]any `json:"raw"`
	DetailRaw      map[string]any `json:"detail_raw"`
}

type CSVImportInput struct {
	Content string `json:"content"`
	Publish bool   `json:"publish"`
}

type CSVImportPreviewInput struct {
	Content string `json:"content"`
}

type CSVImportRowResult struct {
	Row         int    `json:"row"`
	TemplateRef string `json:"template_ref"`
	Action      string `json:"action"`
	Error       string `json:"error,omitempty"`
}

type CSVImportPreviewAssetCheck struct {
	ProductCode string `json:"product_code"`
	Category    string `json:"category"`
	SourceType  string `json:"source_type"`
	SourceRef   string `json:"source_ref"`
	Status      string `json:"status"`
	StorageKey  string `json:"storage_key,omitempty"`
}

type CSVImportPreviewRow struct {
	Row           int                          `json:"row"`
	TemplateRef   string                       `json:"template_ref"`
	Action        string                       `json:"action"`
	Valid         bool                         `json:"valid"`
	ReadyToImport bool                         `json:"ready_to_import"`
	Error         string                       `json:"error,omitempty"`
	AssetChecks   []CSVImportPreviewAssetCheck `json:"asset_checks,omitempty"`
}

type CSVImportPreviewSummary struct {
	TotalRows          int `json:"total_rows"`
	ValidRows          int `json:"valid_rows"`
	InvalidRows        int `json:"invalid_rows"`
	CreateCount        int `json:"create_count"`
	UpdateCount        int `json:"update_count"`
	ReadyToImportCount int `json:"ready_to_import_count"`
	MissingAssetRows   int `json:"missing_asset_rows"`
	MissingAssetCount  int `json:"missing_asset_count"`
}

type CSVImportPreviewResult struct {
	Summary CSVImportPreviewSummary `json:"summary"`
	Rows    []CSVImportPreviewRow   `json:"rows"`
}

type CSVImportResult struct {
	ImportedCount  int                  `json:"imported_count"`
	PublishedCount int                  `json:"published_count"`
	Rows           []CSVImportRowResult `json:"rows"`
}

type PreparedRealImportBundle struct {
	Content                string `json:"content"`
	CSVPath                string `json:"csv_path"`
	AssetManifestPath      string `json:"asset_manifest_path"`
	SummaryPath            string `json:"summary_path"`
	TemplateCount          int    `json:"template_count"`
	MenuTemplateCount      int    `json:"menu_template_count"`
	EcommerceTemplateCount int    `json:"ecommerce_template_count"`
	AssetManifestItemCount int    `json:"asset_manifest_item_count"`
	MissingAssetCount      int    `json:"missing_asset_count"`
}

type PreparedAssetManifestItem struct {
	ProductCode     string         `json:"productCode"`
	Category        string         `json:"category"`
	SourceType      string         `json:"sourceType"`
	SourceRef       string         `json:"sourceRef"`
	AssetRef        string         `json:"assetRef"`
	StorageFileName string         `json:"storageFileName"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	SourcePath      string         `json:"sourcePath"`
	Tags            []string       `json:"tags"`
	Metadata        map[string]any `json:"metadata"`
}

type PreparedAssetImportItemResult struct {
	SourceRef   string `json:"source_ref"`
	ProductCode string `json:"product_code"`
	Status      string `json:"status"`
	StorageKey  string `json:"storage_key,omitempty"`
	Error       string `json:"error,omitempty"`
}

type PreparedAssetImportResult struct {
	ManifestPath  string                          `json:"manifest_path"`
	ImportedCount int                             `json:"imported_count"`
	SkippedCount  int                             `json:"skipped_count"`
	FailedCount   int                             `json:"failed_count"`
	Items         []PreparedAssetImportItemResult `json:"items"`
}

type PreparedAssetImportInput struct {
	OnlyMissing bool `json:"only_missing"`
}

type BatchUploadAssetItemInput struct {
	ProductCode string         `json:"product_code"`
	Category    string         `json:"category"`
	SourceType  string         `json:"source_type"`
	SourceRef   string         `json:"source_ref"`
	FileName    string         `json:"file_name"`
	MimeType    string         `json:"mime_type"`
	Payload     string         `json:"payload"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
}

type BatchUploadAssetsInput struct {
	Items []BatchUploadAssetItemInput `json:"items"`
}

type BatchUploadAssetItemResult struct {
	SourceRef   string `json:"source_ref"`
	ProductCode string `json:"product_code"`
	Status      string `json:"status"`
	StorageKey  string `json:"storage_key,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	Error       string `json:"error,omitempty"`
}

type BatchUploadAssetsResult struct {
	ImportedCount int                          `json:"imported_count"`
	FailedCount   int                          `json:"failed_count"`
	Items         []BatchUploadAssetItemResult `json:"items"`
}

type upstreamEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

var csvColumns = []string{
	"product_code",
	"template_id",
	"slug",
	"name",
	"summary",
	"status",
	"scope",
	"managed_source",
	"cover_asset_url",
	"cover_asset_id",
	"recommend_score",
	"platforms_json",
	"tags_json",
	"series",
	"capability_type",
	"modality",
	"raw_json",
	"detail_json",
}

func NewService(cfg config.Config, db *gorm.DB) *Service {
	sources := map[string]templateSource{}
	for _, endpoint := range cfg.Bootstrap.Runtime.ProductEndpoints {
		switch endpoint.ProductCode {
		case "menu", "ecommerce":
			baseURL := strings.TrimRight(strings.TrimSpace(endpoint.BaseURL), "/")
			if baseURL == "" {
				continue
			}
			sources[endpoint.ProductCode] = templateSource{
				ProductCode: endpoint.ProductCode,
				BaseURL:     baseURL,
			}
		}
	}
	var storageService *assetstorage.Service
	if db != nil {
		storageService = assetstorage.NewService(repository.NewRuntimeRepository(db))
	}
	return &Service{
		client:       &http.Client{Timeout: 15 * time.Second},
		db:           db,
		sources:      sources,
		assetStorage: storageService,
	}
}

func (s *Service) ListCatalog(ctx context.Context, input ListCatalogInput) (*TemplateCatalogListResult, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	query := s.db.WithContext(ctx).Model(&models.TemplateProjection{})
	if productCode := strings.TrimSpace(input.ProductCode); productCode != "" {
		query = query.Where("product_code = ?", productCode)
	}
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("template_ref LIKE ? OR template_id LIKE ? OR slug LIKE ? OR name LIKE ? OR summary LIKE ?", like, like, like, like, like)
	}
	if toolSlug := strings.TrimSpace(input.ToolSlug); toolSlug != "" {
		routeLike := "%/" + toolSlug + "\"%"
		camelLike := "%\"toolSlug\":\"" + toolSlug + "\"%"
		snakeLike := "%\"tool_slug\":\"" + toolSlug + "\"%"
		query = query.Where("(slug = ? OR raw_json LIKE ? OR detail_json LIKE ? OR raw_json LIKE ? OR detail_json LIKE ? OR raw_json LIKE ? OR detail_json LIKE ?)",
			toolSlug, routeLike, routeLike, camelLike, camelLike, snakeLike, snakeLike)
	}
	if input.PublishedOnly {
		query = query.Where("publish_status = ?", "published")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var records []models.TemplateProjection
	if err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]TemplateCatalogItem, 0, len(records))
	for _, record := range records {
		items = append(items, projectionToItem(record))
	}
	return &TemplateCatalogListResult{
		Items:  items,
		Total:  int(total),
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *Service) GetDetail(ctx context.Context, templateRef string, locale string) (*TemplateCatalogDetail, error) {
	var record models.TemplateProjection
	if err := s.db.WithContext(ctx).Where("template_ref = ?", templateRef).First(&record).Error; err != nil {
		return nil, err
	}
	detailRaw := map[string]any{}
	if strings.TrimSpace(record.DetailJSON) != "" {
		_ = json.Unmarshal([]byte(record.DetailJSON), &detailRaw)
	}
	return &TemplateCatalogDetail{
		Item:      projectionToItem(record),
		Product:   record.ProductCode,
		DetailRaw: detailRaw,
	}, nil
}

func (s *Service) listMenuCatalog(ctx context.Context, input ListCatalogInput) ([]TemplateCatalogItem, error) {
	source, ok := s.sources["menu"]
	if !ok {
		return nil, nil
	}
	values := url.Values{}
	if q := strings.TrimSpace(input.Query); q != "" {
		values.Set("query", q)
	}
	body, err := s.fetchJSON(ctx, source.BaseURL+"/api/v1/template-center/catalog?"+values.Encode())
	if err != nil {
		return nil, err
	}
	var payload struct {
		Items []struct {
			TemplateID     string   `json:"template_id"`
			Slug           string   `json:"slug"`
			Name           string   `json:"name"`
			Description    string   `json:"description"`
			Platforms      []string `json:"platforms"`
			Tags           []string `json:"tags"`
			RecommendScore int      `json:"recommend_score"`
			CoverAssetID   string   `json:"cover_asset_id"`
			Locked         bool     `json:"locked"`
			Cuisine        string   `json:"cuisine"`
			DishType       string   `json:"dish_type"`
			Moods          []string `json:"moods"`
			PlanRequired   string   `json:"plan_required"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]TemplateCatalogItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		raw := map[string]any{
			"cuisine":       item.Cuisine,
			"dish_type":     item.DishType,
			"moods":         item.Moods,
			"plan_required": item.PlanRequired,
			"locked":        item.Locked,
		}
		items = append(items, TemplateCatalogItem{
			TemplateRef:    buildTemplateRef("menu", item.TemplateID),
			ProductCode:    "menu",
			TemplateID:     item.TemplateID,
			Slug:           item.Slug,
			Name:           item.Name,
			Summary:        item.Description,
			Status:         "published",
			CoverAssetID:   item.CoverAssetID,
			RecommendScore: item.RecommendScore,
			Tags:           item.Tags,
			Platforms:      item.Platforms,
			Raw:            raw,
		})
	}
	return items, nil
}

func (s *Service) listEcommerceCatalog(ctx context.Context, input ListCatalogInput) ([]TemplateCatalogItem, error) {
	source, ok := s.sources["ecommerce"]
	if !ok {
		return nil, nil
	}
	values := url.Values{}
	values.Set("locale", defaultLocale(input.Locale))
	values.Set("sortBy", "recommended")
	if q := strings.TrimSpace(input.Query); q != "" {
		values.Set("keyword", q)
	}
	body, err := s.fetchJSON(ctx, source.BaseURL+"/api/v1/ecommerce/template-center/catalog?"+values.Encode())
	if err != nil {
		return nil, err
	}
	var payload []struct {
		ID             string   `json:"id"`
		Slug           string   `json:"slug"`
		Name           string   `json:"name"`
		Summary        string   `json:"summary"`
		Modality       string   `json:"modality"`
		ExecutorType   string   `json:"executorType"`
		Series         string   `json:"series"`
		CapabilityType string   `json:"capabilityType"`
		CoverAssetURL  string   `json:"coverAssetUrl"`
		PlatformTags   []string `json:"platformTags"`
		IndustryTags   []string `json:"industryTags"`
		ScenarioTags   []string `json:"scenarioTags"`
		RecommendScore int      `json:"recommendScore"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]TemplateCatalogItem, 0, len(payload))
	for _, item := range payload {
		raw := map[string]any{
			"executor_type": item.ExecutorType,
			"industry_tags": item.IndustryTags,
			"scenario_tags": item.ScenarioTags,
		}
		items = append(items, TemplateCatalogItem{
			TemplateRef:    buildTemplateRef("ecommerce", item.ID),
			ProductCode:    "ecommerce",
			TemplateID:     item.ID,
			Slug:           item.Slug,
			Name:           item.Name,
			Summary:        item.Summary,
			Status:         "published",
			CoverAssetURL:  item.CoverAssetURL,
			RecommendScore: item.RecommendScore,
			Platforms:      item.PlatformTags,
			Series:         item.Series,
			CapabilityType: item.CapabilityType,
			Modality:       item.Modality,
			Raw:            raw,
		})
	}
	return items, nil
}

func (s *Service) getMenuDetail(ctx context.Context, productCode, templateID string) (*TemplateCatalogDetail, error) {
	source := s.sources[productCode]
	body, err := s.fetchJSON(ctx, fmt.Sprintf("%s/api/v1/template-center/catalog/%s", source.BaseURL, url.PathEscape(templateID)))
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	item := TemplateCatalogItem{
		TemplateRef:  buildTemplateRef(productCode, templateID),
		ProductCode:  productCode,
		TemplateID:   templateID,
		Name:         stringValue(payload["name"]),
		Slug:         stringValue(payload["slug"]),
		Summary:      stringValue(payload["description"]),
		Status:       "published",
		CoverAssetID: stringValue(payload["cover_asset_id"]),
	}
	item.Platforms = toStringSlice(payload["platforms"])
	item.Tags = toStringSlice(payload["tags"])
	item.RecommendScore = intValue(payload["recommend_score"])
	item.Raw = payload
	return &TemplateCatalogDetail{
		Item:      item,
		Product:   productCode,
		DetailRaw: payload,
	}, nil
}

func (s *Service) getEcommerceDetail(ctx context.Context, productCode, templateID, locale string) (*TemplateCatalogDetail, error) {
	source := s.sources[productCode]
	values := url.Values{}
	values.Set("locale", defaultLocale(locale))
	body, err := s.fetchJSON(ctx, fmt.Sprintf("%s/api/v1/ecommerce/template-center/catalog/%s?%s", source.BaseURL, url.PathEscape(templateID), values.Encode()))
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	catalog, _ := payload["catalog"].(map[string]any)
	item := TemplateCatalogItem{
		TemplateRef:    buildTemplateRef(productCode, templateID),
		ProductCode:    productCode,
		TemplateID:     templateID,
		Name:           stringValue(catalog["name"]),
		Slug:           stringValue(catalog["slug"]),
		Summary:        stringValue(catalog["summary"]),
		Status:         "published",
		CoverAssetURL:  stringValue(catalog["coverAssetUrl"]),
		RecommendScore: intValue(catalog["recommendScore"]),
		Platforms:      toStringSlice(catalog["platformTags"]),
		Series:         stringValue(catalog["series"]),
		CapabilityType: stringValue(catalog["capabilityType"]),
		Modality:       stringValue(catalog["modality"]),
		Raw:            payload,
	}
	return &TemplateCatalogDetail{
		Item:      item,
		Product:   productCode,
		DetailRaw: payload,
	}, nil
}

func (s *Service) SyncFromUpstream(ctx context.Context, productCode string, locale string) (*TemplateCatalogListResult, error) {
	input := ListCatalogInput{ProductCode: productCode, Locale: locale, Limit: 500, Offset: 0}
	products := []string{"menu", "ecommerce"}
	if productCode = strings.TrimSpace(productCode); productCode != "" {
		products = []string{productCode}
	}
	upserted := make([]TemplateCatalogItem, 0)
	for _, code := range products {
		var items []TemplateCatalogItem
		var err error
		switch code {
		case "menu":
			items, err = s.listMenuCatalog(ctx, input)
		case "ecommerce":
			items, err = s.listEcommerceCatalog(ctx, input)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			detailRaw := map[string]any{
				"catalog": item.Raw,
				"source":  "catalog_list_fallback",
			}
			if detail, err := s.fetchUpstreamDetail(ctx, item.TemplateRef, locale); err == nil && detail != nil && detail.DetailRaw != nil {
				detailRaw = detail.DetailRaw
			} else if err != nil {
				detailRaw["detail_sync_status"] = "unavailable"
				detailRaw["detail_sync_error"] = err.Error()
			}
			record := projectionFromUpsert(UpsertTemplateInput{
				ProductCode:    item.ProductCode,
				TemplateID:     item.TemplateID,
				Slug:           item.Slug,
				Name:           item.Name,
				Summary:        item.Summary,
				Status:         "active",
				Scope:          firstNonEmpty(item.Scope, "official"),
				ManagedSource:  "external_sync",
				CoverAssetID:   item.CoverAssetID,
				CoverAssetURL:  item.CoverAssetURL,
				RecommendScore: item.RecommendScore,
				Tags:           item.Tags,
				Platforms:      item.Platforms,
				Series:         item.Series,
				CapabilityType: item.CapabilityType,
				Modality:       item.Modality,
				Raw:            item.Raw,
				DetailRaw:      detailRaw,
			})
			now := time.Now()
			record.LastSyncedAt = now
			record.SourceUpdatedAt = now
			var existing models.TemplateProjection
			err = s.db.WithContext(ctx).Where("template_ref = ?", record.TemplateRef).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				record.PublishStatus = "published"
				record.PublishedAt = &now
				if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
					return nil, err
				}
			} else if err != nil {
				return nil, err
			} else {
				record.PublishStatus = existing.PublishStatus
				record.PublishedAt = existing.PublishedAt
				if record.PublishStatus == "" {
					record.PublishStatus = "published"
					record.PublishedAt = &now
				}
				if err := s.db.WithContext(ctx).Model(&models.TemplateProjection{}).Where("template_ref = ?", record.TemplateRef).Updates(map[string]any{
					"product_code":      record.ProductCode,
					"template_id":       record.TemplateID,
					"slug":              record.Slug,
					"name":              record.Name,
					"summary":           record.Summary,
					"status":            record.Status,
					"scope":             record.Scope,
					"managed_source":    record.ManagedSource,
					"cover_asset_id":    record.CoverAssetID,
					"cover_asset_url":   record.CoverAssetURL,
					"recommend_score":   record.RecommendScore,
					"platforms_json":    record.PlatformsJSON,
					"tags_json":         record.TagsJSON,
					"series":            record.Series,
					"capability_type":   record.CapabilityType,
					"modality":          record.Modality,
					"raw_json":          record.RawJSON,
					"detail_json":       record.DetailJSON,
					"last_synced_at":    record.LastSyncedAt,
					"source_updated_at": record.SourceUpdatedAt,
					"updated_at":        now,
				}).Error; err != nil {
					return nil, err
				}
			}
			upserted = append(upserted, projectionToItem(record))
		}
	}
	return &TemplateCatalogListResult{Items: upserted, Total: len(upserted), Limit: len(upserted), Offset: 0}, nil
}

func (s *Service) CreateTemplate(ctx context.Context, input UpsertTemplateInput) (*TemplateCatalogDetail, error) {
	record := projectionFromUpsert(input)
	now := time.Now()
	if record.TemplateRef == ":" || strings.TrimSpace(record.TemplateRef) == "" {
		return nil, fmt.Errorf("product_code and template_id are required")
	}
	record.PublishStatus = "draft"
	record.LastSyncedAt = now
	record.SourceUpdatedAt = now
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, record.TemplateRef, "zh")
}

func (s *Service) UpdateTemplate(ctx context.Context, templateRef string, input UpsertTemplateInput) (*TemplateCatalogDetail, error) {
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	record := projectionFromUpsert(input)
	if record.ProductCode != "" {
		updates["product_code"] = record.ProductCode
	}
	if record.TemplateID != "" {
		updates["template_id"] = record.TemplateID
	}
	if record.Slug != "" {
		updates["slug"] = record.Slug
	}
	if record.Name != "" {
		updates["name"] = record.Name
	}
	updates["summary"] = record.Summary
	if record.Status != "" {
		updates["status"] = record.Status
	}
	if record.Scope != "" {
		updates["scope"] = record.Scope
	}
	if record.ManagedSource != "" {
		updates["managed_source"] = record.ManagedSource
	}
	updates["cover_asset_id"] = record.CoverAssetID
	updates["cover_asset_url"] = record.CoverAssetURL
	updates["recommend_score"] = record.RecommendScore
	updates["platforms_json"] = record.PlatformsJSON
	updates["tags_json"] = record.TagsJSON
	updates["series"] = record.Series
	updates["capability_type"] = record.CapabilityType
	updates["modality"] = record.Modality
	updates["raw_json"] = record.RawJSON
	updates["detail_json"] = record.DetailJSON
	if err := s.db.WithContext(ctx).Model(&models.TemplateProjection{}).Where("template_ref = ?", templateRef).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, templateRef, "zh")
}

func (s *Service) ImportCSV(ctx context.Context, input CSVImportInput) (*CSVImportResult, error) {
	reader := csv.NewReader(strings.NewReader(input.Content))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("csv content is empty")
	}
	header := rows[0]
	indexByColumn := map[string]int{}
	for i, column := range header {
		indexByColumn[strings.TrimSpace(column)] = i
	}
	for _, required := range []string{"product_code", "template_id", "name"} {
		if _, ok := indexByColumn[required]; !ok {
			return nil, fmt.Errorf("missing required csv column: %s", required)
		}
	}

	result := &CSVImportResult{Rows: make([]CSVImportRowResult, 0, len(rows)-1)}
	for rowIndex, row := range rows[1:] {
		rowNo := rowIndex + 2
		upsert, parseErr := parseCSVUpsertRow(row, indexByColumn)
		templateRef := buildTemplateRef(upsert.ProductCode, upsert.TemplateID)
		if parseErr != nil {
			result.Rows = append(result.Rows, CSVImportRowResult{Row: rowNo, TemplateRef: templateRef, Action: "invalid", Error: parseErr.Error()})
			continue
		}
		var existing models.TemplateProjection
		err := s.db.WithContext(ctx).Where("template_ref = ?", templateRef).First(&existing).Error
		action := "created"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if _, err := s.CreateTemplate(ctx, upsert); err != nil {
				result.Rows = append(result.Rows, CSVImportRowResult{Row: rowNo, TemplateRef: templateRef, Action: "failed", Error: err.Error()})
				continue
			}
		} else if err != nil {
			result.Rows = append(result.Rows, CSVImportRowResult{Row: rowNo, TemplateRef: templateRef, Action: "failed", Error: err.Error()})
			continue
		} else {
			action = "updated"
			if _, err := s.UpdateTemplate(ctx, templateRef, upsert); err != nil {
				result.Rows = append(result.Rows, CSVImportRowResult{Row: rowNo, TemplateRef: templateRef, Action: "failed", Error: err.Error()})
				continue
			}
		}
		result.ImportedCount++
		if input.Publish {
			if _, err := s.PublishTemplate(ctx, templateRef); err != nil {
				result.Rows = append(result.Rows, CSVImportRowResult{Row: rowNo, TemplateRef: templateRef, Action: action, Error: "publish failed: " + err.Error()})
				continue
			}
			result.PublishedCount++
		}
		result.Rows = append(result.Rows, CSVImportRowResult{Row: rowNo, TemplateRef: templateRef, Action: action})
	}
	return result, nil
}

func (s *Service) PreviewImportCSV(ctx context.Context, input CSVImportPreviewInput) (*CSVImportPreviewResult, error) {
	reader := csv.NewReader(strings.NewReader(input.Content))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("csv content is empty")
	}
	header := rows[0]
	indexByColumn := map[string]int{}
	for i, column := range header {
		indexByColumn[strings.TrimSpace(column)] = i
	}
	for _, required := range []string{"product_code", "template_id", "name"} {
		if _, ok := indexByColumn[required]; !ok {
			return nil, fmt.Errorf("missing required csv column: %s", required)
		}
	}

	result := &CSVImportPreviewResult{
		Rows: make([]CSVImportPreviewRow, 0, len(rows)-1),
	}
	runtimeRepo := storageAssetFinder(nil)
	if s.db != nil {
		runtimeRepo = &templateOpsStorageAssetRepo{db: s.db}
	}
	for rowIndex, row := range rows[1:] {
		rowNo := rowIndex + 2
		upsert, parseErr := parseCSVUpsertRow(row, indexByColumn)
		templateRef := buildTemplateRef(upsert.ProductCode, upsert.TemplateID)
		if parseErr != nil {
			result.Rows = append(result.Rows, CSVImportPreviewRow{
				Row:           rowNo,
				TemplateRef:   templateRef,
				Action:        "invalid",
				Valid:         false,
				ReadyToImport: false,
				Error:         parseErr.Error(),
			})
			result.Summary.InvalidRows++
			continue
		}
		action := "create"
		var existing models.TemplateProjection
		findErr := s.db.WithContext(ctx).Where("template_ref = ?", templateRef).First(&existing).Error
		if findErr == nil {
			action = "update"
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			result.Rows = append(result.Rows, CSVImportPreviewRow{
				Row:           rowNo,
				TemplateRef:   templateRef,
				Action:        "invalid",
				Valid:         false,
				ReadyToImport: false,
				Error:         findErr.Error(),
			})
			result.Summary.InvalidRows++
			continue
		}
		assetChecks, missingAssetCount := buildAssetChecks(upsert, runtimeRepo)
		ready := missingAssetCount == 0
		result.Rows = append(result.Rows, CSVImportPreviewRow{
			Row:           rowNo,
			TemplateRef:   templateRef,
			Action:        action,
			Valid:         true,
			ReadyToImport: ready,
			AssetChecks:   assetChecks,
		})
		result.Summary.ValidRows++
		if action == "create" {
			result.Summary.CreateCount++
		} else {
			result.Summary.UpdateCount++
		}
		if ready {
			result.Summary.ReadyToImportCount++
		} else {
			result.Summary.MissingAssetRows++
			result.Summary.MissingAssetCount += missingAssetCount
		}
	}
	result.Summary.TotalRows = len(result.Rows)
	result.Summary.InvalidRows = result.Summary.TotalRows - result.Summary.ValidRows
	return result, nil
}

func (s *Service) LoadPreparedRealImportBundle() (*PreparedRealImportBundle, error) {
	csvPath := "testdata/templateops/real-import/template_ops_real_import.csv"
	manifestPath := "testdata/templateops/real-import/template_ops_real_asset_manifest.json"
	summaryPath := "testdata/templateops/real-import/template_ops_real_import_summary.json"
	content, err := os.ReadFile(csvPath)
	if err != nil {
		return nil, err
	}
	summaryBody, err := os.ReadFile(summaryPath)
	if err != nil {
		return nil, err
	}
	var summary struct {
		TemplateCount          int `json:"templateCount"`
		MenuTemplateCount      int `json:"menuTemplateCount"`
		EcommerceTemplateCount int `json:"ecommerceTemplateCount"`
		AssetManifestItemCount int `json:"assetManifestItemCount"`
		MissingAssetCount      int `json:"missingAssetCount"`
	}
	if err := json.Unmarshal(summaryBody, &summary); err != nil {
		return nil, err
	}
	return &PreparedRealImportBundle{
		Content:                string(content),
		CSVPath:                csvPath,
		AssetManifestPath:      manifestPath,
		SummaryPath:            summaryPath,
		TemplateCount:          summary.TemplateCount,
		MenuTemplateCount:      summary.MenuTemplateCount,
		EcommerceTemplateCount: summary.EcommerceTemplateCount,
		AssetManifestItemCount: summary.AssetManifestItemCount,
		MissingAssetCount:      summary.MissingAssetCount,
	}, nil
}

func (s *Service) loadPreparedRealAssetManifest() (string, []PreparedAssetManifestItem, error) {
	manifestPath := "testdata/templateops/real-import/template_ops_real_asset_manifest.json"
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", nil, err
	}
	var payload struct {
		Items []PreparedAssetManifestItem `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil, err
	}
	return manifestPath, payload.Items, nil
}

func (s *Service) ImportPreparedRealAssets(ctx context.Context, input PreparedAssetImportInput) (*PreparedAssetImportResult, error) {
	if s.assetStorage == nil {
		return nil, fmt.Errorf("asset storage service is not configured")
	}
	manifestPath, items, err := s.loadPreparedRealAssetManifest()
	if err != nil {
		return nil, err
	}
	onlyMissing := true
	if !input.OnlyMissing {
		onlyMissing = false
	}
	result := &PreparedAssetImportResult{
		ManifestPath: manifestPath,
		Items:        make([]PreparedAssetImportItemResult, 0, len(items)),
	}
	for _, item := range items {
		if onlyMissing {
			record, findErr := s.assetStorage.FindAssetMetadataBySource(ctx, item.ProductCode, item.Category, item.SourceType, item.SourceRef)
			if findErr == nil && record != nil {
				result.SkippedCount++
				result.Items = append(result.Items, PreparedAssetImportItemResult{
					SourceRef:   item.SourceRef,
					ProductCode: item.ProductCode,
					Status:      "already_ready",
					StorageKey:  record.StorageKey,
				})
				continue
			}
		}
		imported, importErr := s.assetStorage.ImportLocalAsset(ctx, assetstorage.ImportLocalAssetInput{
			ProductCode:     item.ProductCode,
			Category:        item.Category,
			SourceType:      firstNonEmpty(item.SourceType, "template_example"),
			SourceRef:       item.SourceRef,
			SourcePath:      firstNonEmpty(item.SourcePath, item.AssetRef),
			StorageFileName: item.StorageFileName,
			Title:           item.Title,
			Description:     item.Description,
			Tags:            item.Tags,
			Metadata:        item.Metadata,
		})
		if importErr != nil {
			result.FailedCount++
			result.Items = append(result.Items, PreparedAssetImportItemResult{
				SourceRef:   item.SourceRef,
				ProductCode: item.ProductCode,
				Status:      "failed",
				Error:       importErr.Error(),
			})
			continue
		}
		result.ImportedCount++
		result.Items = append(result.Items, PreparedAssetImportItemResult{
			SourceRef:   item.SourceRef,
			ProductCode: item.ProductCode,
			Status:      "imported",
			StorageKey:  imported.StorageKey,
		})
	}
	return result, nil
}

func (s *Service) BatchUploadAssets(ctx context.Context, input BatchUploadAssetsInput) (*BatchUploadAssetsResult, error) {
	if s.assetStorage == nil {
		return nil, fmt.Errorf("asset storage service is not configured")
	}
	result := &BatchUploadAssetsResult{
		Items: make([]BatchUploadAssetItemResult, 0, len(input.Items)),
	}
	for _, item := range input.Items {
		productCode := strings.TrimSpace(item.ProductCode)
		category := firstNonEmpty(item.Category, "template-examples")
		sourceType := firstNonEmpty(item.SourceType, "template_example")
		sourceRef := strings.TrimSpace(item.SourceRef)
		if productCode == "" || sourceRef == "" || strings.TrimSpace(item.Payload) == "" {
			result.FailedCount++
			result.Items = append(result.Items, BatchUploadAssetItemResult{
				SourceRef:   sourceRef,
				ProductCode: productCode,
				Status:      "failed",
				Error:       "product_code, source_ref, and payload are required",
			})
			continue
		}
		stored, uploadErr := s.assetStorage.UploadAsset(ctx, assetstorage.UploadAssetInput{
			ProductCode: productCode,
			Category:    category,
			FileName:    item.FileName,
			MimeType:    item.MimeType,
			Payload:     item.Payload,
		})
		if uploadErr != nil {
			result.FailedCount++
			result.Items = append(result.Items, BatchUploadAssetItemResult{
				SourceRef:   sourceRef,
				ProductCode: productCode,
				Status:      "failed",
				Error:       uploadErr.Error(),
			})
			continue
		}
		record, registerErr := s.assetStorage.RegisterAsset(ctx, assetstorage.RegisterAssetInput{
			ProductCode: productCode,
			Category:    category,
			SourceType:  sourceType,
			SourceRef:   sourceRef,
			StorageKey:  stored.StorageKey,
			FileName:    item.FileName,
			MimeType:    stored.MimeType,
			FileSize:    stored.FileSize,
			Title:       item.Title,
			Description: item.Description,
			Tags:        item.Tags,
			Metadata:    item.Metadata,
			Status:      "active",
		})
		if registerErr != nil {
			result.FailedCount++
			result.Items = append(result.Items, BatchUploadAssetItemResult{
				SourceRef:   sourceRef,
				ProductCode: productCode,
				Status:      "failed",
				Error:       registerErr.Error(),
			})
			continue
		}
		result.ImportedCount++
		result.Items = append(result.Items, BatchUploadAssetItemResult{
			SourceRef:   sourceRef,
			ProductCode: productCode,
			Status:      "imported",
			StorageKey:  record.StorageKey,
			FileName:    record.FileName,
		})
	}
	return result, nil
}

func (s *Service) ListTemplateAssets(ctx context.Context, templateRef string) (*TemplateAssetBindingsResult, error) {
	detail, err := s.GetDetail(ctx, templateRef, "zh")
	if err != nil {
		return nil, err
	}
	productCode, _, err := parseTemplateRef(templateRef)
	if err != nil {
		return nil, err
	}
	items := buildTemplateAssetBindings(detail.DetailRaw, productCode)
	if s.assetStorage != nil {
		for idx := range items {
			record, findErr := s.assetStorage.FindAssetMetadataBySource(ctx, items[idx].ProductCode, items[idx].Category, items[idx].SourceType, items[idx].SourceRef)
			if findErr != nil {
				continue
			}
			items[idx].StorageKey = record.StorageKey
			items[idx].AssetID = record.ID
			items[idx].MimeType = record.MimeType
			items[idx].FileName = record.FileName
			items[idx].Checksum = record.Checksum
			items[idx].Status = "ready"
			if record.StorageKey != "" {
				items[idx].PreviewURL = "/api/v1/assets/content?storage_key=" + url.QueryEscape(record.StorageKey)
			}
		}
	}
	return &TemplateAssetBindingsResult{
		TemplateRef: templateRef,
		Items:       items,
	}, nil
}

func (s *Service) UpsertTemplateAsset(ctx context.Context, templateRef string, input UpsertTemplateAssetInput) (*TemplateAssetBindingsResult, error) {
	if s.assetStorage == nil {
		return nil, fmt.Errorf("asset storage service is not configured")
	}
	productCode, _, err := parseTemplateRef(templateRef)
	if err != nil {
		return nil, err
	}
	detail, err := s.GetDetail(ctx, templateRef, "zh")
	if err != nil {
		return nil, err
	}
	normalizedRole := strings.TrimSpace(input.AssetRole)
	if normalizedRole == "" {
		return nil, fmt.Errorf("asset_role is required")
	}
	category := firstNonEmpty(input.Category, "template-examples")
	sourceType := firstNonEmpty(input.SourceType, "template_example")
	sourceRef := strings.TrimSpace(input.SourceRef)
	exampleIndex := 0
	if strings.HasPrefix(normalizedRole, "example_") {
		if number, convErr := strconv.Atoi(strings.TrimPrefix(normalizedRole, "example_")); convErr == nil {
			exampleIndex = number
		}
	}
	if sourceRef == "" {
		sourceRef = inferSourceRefFromRole(detail.DetailRaw, normalizedRole, exampleIndex)
	}
	if strings.TrimSpace(sourceRef) == "" {
		return nil, fmt.Errorf("source_ref could not be resolved for asset_role=%s", normalizedRole)
	}
	var metadata map[string]any
	if input.Metadata != nil {
		metadata = input.Metadata
	} else {
		metadata = map[string]any{}
	}
	metadata["templateRef"] = templateRef
	metadata["assetRole"] = normalizedRole
	if strings.TrimSpace(input.SourcePath) != "" {
		_, err = s.assetStorage.ImportLocalAsset(ctx, assetstorage.ImportLocalAssetInput{
			ProductCode:     productCode,
			Category:        category,
			SourceType:      sourceType,
			SourceRef:       sourceRef,
			SourcePath:      input.SourcePath,
			StorageFileName: firstNonEmpty(input.StorageFileName, input.FileName),
			MimeType:        input.MimeType,
			Title:           input.Title,
			Description:     input.Description,
			Tags:            input.Tags,
			Metadata:        metadata,
			Status:          input.Status,
		})
		if err != nil {
			return nil, err
		}
	} else if strings.TrimSpace(input.Payload) != "" {
		stored, uploadErr := s.assetStorage.UploadAsset(ctx, assetstorage.UploadAssetInput{
			ProductCode: productCode,
			Category:    category,
			FileName:    firstNonEmpty(input.FileName, input.StorageFileName),
			MimeType:    input.MimeType,
			Payload:     input.Payload,
		})
		if uploadErr != nil {
			return nil, uploadErr
		}
		_, err = s.assetStorage.RegisterAsset(ctx, assetstorage.RegisterAssetInput{
			ProductCode: productCode,
			Category:    category,
			SourceType:  sourceType,
			SourceRef:   sourceRef,
			StorageKey:  stored.StorageKey,
			FileName:    firstNonEmpty(input.FileName, input.StorageFileName),
			MimeType:    stored.MimeType,
			FileSize:    stored.FileSize,
			Title:       input.Title,
			Description: input.Description,
			Tags:        input.Tags,
			Metadata:    metadata,
			Status:      input.Status,
		})
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("source_path or payload is required")
	}
	assetRecord, err := s.assetStorage.FindAssetMetadataBySource(ctx, productCode, category, sourceType, sourceRef)
	if err != nil {
		return nil, err
	}
	detailUpdated := upsertTemplateAssetBinding(detail.DetailRaw, normalizedRole, input, sourceRef, assetRecord)
	coverAssetID := detail.Item.CoverAssetID
	coverAssetURL := detail.Item.CoverAssetURL
	if normalizedRole == "cover" || normalizedRole == "example_1" {
		coverAssetID = assetRecord.ID
		coverAssetURL = "/api/v1/assets/content?storage_key=" + url.QueryEscape(assetRecord.StorageKey)
	}
	if _, err := s.UpdateTemplate(ctx, templateRef, UpsertTemplateInput{
		ProductCode:    detail.Item.ProductCode,
		TemplateID:     detail.Item.TemplateID,
		Slug:           detail.Item.Slug,
		Name:           detail.Item.Name,
		Summary:        detail.Item.Summary,
		Status:         "active",
		Scope:          firstNonEmpty(detail.Item.Scope, "official"),
		ManagedSource:  firstNonEmpty(detail.Item.ManagedSource, "ops_manual"),
		CoverAssetURL:  coverAssetURL,
		CoverAssetID:   coverAssetID,
		RecommendScore: detail.Item.RecommendScore,
		Tags:           detail.Item.Tags,
		Platforms:      detail.Item.Platforms,
		Series:         detail.Item.Series,
		CapabilityType: detail.Item.CapabilityType,
		Modality:       detail.Item.Modality,
		Raw:            detail.Item.Raw,
		DetailRaw:      detailUpdated,
	}); err != nil {
		return nil, err
	}
	return s.ListTemplateAssets(ctx, templateRef)
}

func (s *Service) UnbindTemplateAsset(ctx context.Context, templateRef, assetRole string) (*TemplateAssetBindingsResult, error) {
	detail, err := s.GetDetail(ctx, templateRef, "zh")
	if err != nil {
		return nil, err
	}
	normalizedRole := strings.TrimSpace(assetRole)
	if normalizedRole == "" {
		return nil, fmt.Errorf("asset_role is required")
	}
	detailUpdated := removeTemplateAssetBinding(detail.DetailRaw, normalizedRole)
	if _, err := s.UpdateTemplate(ctx, templateRef, UpsertTemplateInput{
		ProductCode:    detail.Item.ProductCode,
		TemplateID:     detail.Item.TemplateID,
		Slug:           detail.Item.Slug,
		Name:           detail.Item.Name,
		Summary:        detail.Item.Summary,
		Status:         "active",
		Scope:          firstNonEmpty(detail.Item.Scope, "official"),
		ManagedSource:  firstNonEmpty(detail.Item.ManagedSource, "ops_manual"),
		CoverAssetURL:  detail.Item.CoverAssetURL,
		CoverAssetID:   detail.Item.CoverAssetID,
		RecommendScore: detail.Item.RecommendScore,
		Tags:           detail.Item.Tags,
		Platforms:      detail.Item.Platforms,
		Series:         detail.Item.Series,
		CapabilityType: detail.Item.CapabilityType,
		Modality:       detail.Item.Modality,
		Raw:            detail.Item.Raw,
		DetailRaw:      detailUpdated,
	}); err != nil {
		return nil, err
	}
	return s.ListTemplateAssets(ctx, templateRef)
}

func (s *Service) ExportCSV(ctx context.Context, input ListCatalogInput) (string, error) {
	result, err := s.ListCatalog(ctx, input)
	if err != nil {
		return "", err
	}
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write(csvColumns); err != nil {
		return "", err
	}
	for _, item := range result.Items {
		detail, err := s.GetDetail(ctx, item.TemplateRef, input.Locale)
		if err != nil {
			return "", err
		}
		row := []string{
			item.ProductCode,
			item.TemplateID,
			item.Slug,
			item.Name,
			item.Summary,
			item.Status,
			item.Scope,
			item.ManagedSource,
			item.CoverAssetURL,
			item.CoverAssetID,
			fmt.Sprintf("%d", item.RecommendScore),
			string(mustJSONSlice(item.Platforms)),
			string(mustJSONSlice(item.Tags)),
			item.Series,
			item.CapabilityType,
			item.Modality,
			string(mustJSONMap(item.Raw)),
			string(mustJSONMap(detail.DetailRaw)),
		}
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func CSVTemplateExample() string {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	_ = writer.Write(csvColumns)
	_ = writer.Write([]string{
		"menu",
		"TPL-DEMO-001",
		"demo-template",
		"Demo Template",
		"Editable by ops in Excel-compatible CSV.",
		"active",
		"official",
		"ops_manual",
		"",
		"",
		"80",
		`["xiaohongshu","instagram"]`,
		`["demo","ops"]`,
		"",
		"",
		"",
		`{"cuisine":"fusion","moods":["fresh"]}`,
		`{"prompt_templates":{"hero":"demo prompt"}}`,
	})
	writer.Flush()
	return buffer.String()
}

func (s *Service) PublishTemplate(ctx context.Context, templateRef string) (*TemplateCatalogDetail, error) {
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&models.TemplateProjection{}).Where("template_ref = ?", templateRef).Updates(map[string]any{
		"publish_status": "published",
		"published_at":   &now,
		"updated_at":     now,
	}).Error; err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, templateRef, "zh")
}

func (s *Service) fetchUpstreamDetail(ctx context.Context, templateRef, locale string) (*TemplateCatalogDetail, error) {
	productCode, templateID, err := parseTemplateRef(templateRef)
	if err != nil {
		return nil, err
	}
	switch productCode {
	case "menu":
		return s.getMenuDetail(ctx, productCode, templateID)
	case "ecommerce":
		return s.getEcommerceDetail(ctx, productCode, templateID, locale)
	default:
		return nil, fmt.Errorf("unsupported template product: %s", productCode)
	}
}

func projectionToItem(record models.TemplateProjection) TemplateCatalogItem {
	raw := map[string]any{}
	if strings.TrimSpace(record.RawJSON) != "" {
		_ = json.Unmarshal([]byte(record.RawJSON), &raw)
	}
	return TemplateCatalogItem{
		TemplateRef:    record.TemplateRef,
		ProductCode:    record.ProductCode,
		TemplateID:     record.TemplateID,
		Slug:           record.Slug,
		Name:           record.Name,
		Summary:        record.Summary,
		Status:         firstNonEmpty(record.PublishStatus, record.Status),
		CoverAssetURL:  record.CoverAssetURL,
		CoverAssetID:   record.CoverAssetID,
		RecommendScore: record.RecommendScore,
		Tags:           decodeStringJSON(record.TagsJSON),
		Platforms:      decodeStringJSON(record.PlatformsJSON),
		Series:         record.Series,
		CapabilityType: record.CapabilityType,
		Modality:       record.Modality,
		Scope:          record.Scope,
		ManagedSource:  record.ManagedSource,
		Raw:            raw,
	}
}

func projectionFromUpsert(input UpsertTemplateInput) models.TemplateProjection {
	rawJSON, _ := json.Marshal(input.Raw)
	detailJSON, _ := json.Marshal(input.DetailRaw)
	productCode := strings.TrimSpace(input.ProductCode)
	templateID := strings.TrimSpace(input.TemplateID)
	return models.TemplateProjection{
		TemplateRef:    buildTemplateRef(productCode, templateID),
		ProductCode:    productCode,
		TemplateID:     templateID,
		Slug:           strings.TrimSpace(input.Slug),
		Name:           strings.TrimSpace(input.Name),
		Summary:        strings.TrimSpace(input.Summary),
		Status:         firstNonEmpty(strings.TrimSpace(input.Status), "active"),
		Scope:          firstNonEmpty(strings.TrimSpace(input.Scope), "official"),
		ManagedSource:  firstNonEmpty(strings.TrimSpace(input.ManagedSource), "ops_manual"),
		CoverAssetID:   strings.TrimSpace(input.CoverAssetID),
		CoverAssetURL:  strings.TrimSpace(input.CoverAssetURL),
		RecommendScore: input.RecommendScore,
		PlatformsJSON:  string(mustJSONSlice(input.Platforms)),
		TagsJSON:       string(mustJSONSlice(input.Tags)),
		Series:         strings.TrimSpace(input.Series),
		CapabilityType: strings.TrimSpace(input.CapabilityType),
		Modality:       strings.TrimSpace(input.Modality),
		RawJSON:        string(rawJSON),
		DetailJSON:     string(detailJSON),
	}
}

func (s *Service) fetchJSON(ctx context.Context, requestURL string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream request failed: %s", resp.Status)
	}
	var envelope upstreamEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Code != 0 {
		if envelope.Error != "" {
			return nil, fmt.Errorf("upstream business error: %s", envelope.Error)
		}
		return nil, fmt.Errorf("upstream business error: %s", envelope.Message)
	}
	return envelope.Data, nil
}

func buildTemplateRef(productCode, templateID string) string {
	return productCode + ":" + templateID
}

func parseTemplateRef(templateRef string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(templateRef), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid template ref: %s", templateRef)
	}
	return parts[0], parts[1], nil
}

func defaultLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "zh"
	}
	return locale
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func intValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func toStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		if stringsValue, ok := value.([]string); ok {
			return stringsValue
		}
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if str, ok := item.(string); ok && str != "" {
			result = append(result, str)
		}
	}
	return result
}

func decodeStringJSON(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var items []string
	_ = json.Unmarshal([]byte(value), &items)
	return items
}

func mustJSONSlice(items []string) []byte {
	encoded, _ := json.Marshal(items)
	return encoded
}

func mustJSONMap(items map[string]any) []byte {
	encoded, _ := json.Marshal(items)
	return encoded
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseCSVUpsertRow(row []string, indexByColumn map[string]int) (UpsertTemplateInput, error) {
	get := func(column string) string {
		index, ok := indexByColumn[column]
		if !ok || index >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[index])
	}
	productCode := get("product_code")
	templateID := get("template_id")
	name := get("name")
	if productCode == "" || templateID == "" || name == "" {
		return UpsertTemplateInput{}, fmt.Errorf("product_code, template_id, and name are required")
	}
	recommendScore := 0
	if value := get("recommend_score"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &recommendScore); err != nil {
			return UpsertTemplateInput{}, fmt.Errorf("invalid recommend_score: %s", value)
		}
	}
	platforms, err := parseJSONStringSlice(get("platforms_json"))
	if err != nil {
		return UpsertTemplateInput{}, fmt.Errorf("invalid platforms_json: %w", err)
	}
	tags, err := parseJSONStringSlice(get("tags_json"))
	if err != nil {
		return UpsertTemplateInput{}, fmt.Errorf("invalid tags_json: %w", err)
	}
	raw, err := parseJSONMap(get("raw_json"))
	if err != nil {
		return UpsertTemplateInput{}, fmt.Errorf("invalid raw_json: %w", err)
	}
	detailRaw, err := parseJSONMap(get("detail_json"))
	if err != nil {
		return UpsertTemplateInput{}, fmt.Errorf("invalid detail_json: %w", err)
	}
	return UpsertTemplateInput{
		ProductCode:    productCode,
		TemplateID:     templateID,
		Slug:           get("slug"),
		Name:           name,
		Summary:        get("summary"),
		Status:         firstNonEmpty(get("status"), "active"),
		Scope:          firstNonEmpty(get("scope"), "official"),
		ManagedSource:  firstNonEmpty(get("managed_source"), "ops_manual"),
		CoverAssetURL:  get("cover_asset_url"),
		CoverAssetID:   get("cover_asset_id"),
		RecommendScore: recommendScore,
		Platforms:      platforms,
		Tags:           tags,
		Series:         get("series"),
		CapabilityType: get("capability_type"),
		Modality:       get("modality"),
		Raw:            raw,
		DetailRaw:      detailRaw,
	}, nil
}

func parseJSONStringSlice(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseJSONMap(value string) (map[string]any, error) {
	if strings.TrimSpace(value) == "" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func buildTemplateAssetBindings(detail map[string]any, productCode string) []TemplateAssetBinding {
	items := make([]TemplateAssetBinding, 0)
	examples, _ := detail["examples"].([]any)
	for index, rawExample := range examples {
		example, ok := rawExample.(map[string]any)
		if !ok {
			continue
		}
		sourceRef := strings.TrimSpace(stringValue(example["sourceRef"]))
		if sourceRef == "" {
			continue
		}
		items = append(items, TemplateAssetBinding{
			AssetRole:    fmt.Sprintf("example_%d", index+1),
			ProductCode:  productCode,
			Category:     "template-examples",
			SourceType:   "template_example",
			SourceRef:    sourceRef,
			Title:        firstNonEmpty(stringValue(example["title"]), stringValue(detail["name"])),
			Description:  stringValue(example["description"]),
			AssetRef:     stringValue(example["assetRef"]),
			StorageKey:   stringValue(example["storageKey"]),
			AssetID:      stringValue(example["assetId"]),
			MimeType:     stringValue(example["mimeType"]),
			FileName:     stringValue(example["storageFileName"]),
			PreviewURL:   stringValue(example["previewAssetUrl"]),
			ExampleIndex: index + 1,
			Status:       "missing",
		})
	}
	return items
}

func inferSourceRefFromRole(detail map[string]any, assetRole string, exampleIndex int) string {
	if exampleIndex > 0 {
		examples, _ := detail["examples"].([]any)
		if exampleIndex-1 < len(examples) {
			if example, ok := examples[exampleIndex-1].(map[string]any); ok {
				if sourceRef := strings.TrimSpace(stringValue(example["sourceRef"])); sourceRef != "" {
					return sourceRef
				}
			}
		}
	}
	toolSlug := ""
	if binding, ok := detail["toolBinding"].(map[string]any); ok {
		toolSlug = strings.TrimSpace(stringValue(binding["toolSlug"]))
	}
	templateCode := firstNonEmpty(
		strings.TrimSpace(stringValue(detail["externalCode"])),
		strings.TrimSpace(stringValue(detail["templateCode"])),
		strings.TrimSpace(stringValue(detail["id"])),
	)
	if toolSlug == "" || templateCode == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(assetRole, "example_") && exampleIndex > 0:
		return fmt.Sprintf("templates/%s/%s/example-%d", toolSlug, templateCode, exampleIndex)
	case assetRole == "cover":
		return fmt.Sprintf("templates/%s/%s/cover", toolSlug, templateCode)
	default:
		return fmt.Sprintf("templates/%s/%s/%s", toolSlug, templateCode, assetRole)
	}
}

func removeTemplateAssetBinding(detail map[string]any, assetRole string) map[string]any {
	cloned := cloneJSONMap(detail)
	if assetRole == "cover" {
		delete(cloned, "coverAssetRef")
		delete(cloned, "coverStorageKey")
		delete(cloned, "coverAssetId")
		return cloned
	}
	index := exampleIndexFromRole(assetRole)
	if index <= 0 {
		return cloned
	}
	examples := ensureExamples(cloned)
	if index-1 >= len(examples) {
		return cloned
	}
	example, ok := examples[index-1].(map[string]any)
	if !ok {
		return cloned
	}
	delete(example, "assetRef")
	delete(example, "sourceRef")
	delete(example, "storageKey")
	delete(example, "assetId")
	delete(example, "mimeType")
	delete(example, "storageFileName")
	delete(example, "previewAssetUrl")
	examples[index-1] = example
	cloned["examples"] = examples
	syncExampleRefs(cloned, examples)
	return cloned
}

func upsertTemplateAssetBinding(detail map[string]any, assetRole string, input UpsertTemplateAssetInput, sourceRef string, assetRecord *assetstorage.AssetRecord) map[string]any {
	cloned := cloneJSONMap(detail)
	previewURL := ""
	if assetRecord != nil && assetRecord.StorageKey != "" {
		previewURL = "/api/v1/assets/content?storage_key=" + url.QueryEscape(assetRecord.StorageKey)
	}
	if assetRole == "cover" {
		cloned["coverAssetRef"] = firstNonEmpty(input.AssetRef, stringValue(cloned["coverAssetRef"]))
		cloned["coverSourceRef"] = sourceRef
		if assetRecord != nil {
			cloned["coverStorageKey"] = firstNonEmpty(assetRecord.StorageKey, stringValue(cloned["coverStorageKey"]))
			cloned["coverAssetId"] = firstNonEmpty(assetRecord.ID, stringValue(cloned["coverAssetId"]))
		}
		cloned["coverPreviewUrl"] = previewURL
		return cloned
	}
	index := exampleIndexFromRole(assetRole)
	if index <= 0 {
		return cloned
	}
	examples := ensureExamples(cloned)
	for len(examples) < index {
		examples = append(examples, map[string]any{})
	}
	example, ok := examples[index-1].(map[string]any)
	if !ok {
		example = map[string]any{}
	}
	example["sourceRef"] = sourceRef
	if strings.TrimSpace(input.AssetRef) != "" {
		example["assetRef"] = input.AssetRef
	}
	if strings.TrimSpace(input.StorageFileName) != "" {
		example["storageFileName"] = input.StorageFileName
	} else if assetRecord != nil && assetRecord.FileName != "" {
		example["storageFileName"] = assetRecord.FileName
	}
	if assetRecord != nil {
		example["storageKey"] = assetRecord.StorageKey
		example["assetId"] = assetRecord.ID
		example["mimeType"] = assetRecord.MimeType
	}
	if previewURL != "" {
		example["previewAssetUrl"] = previewURL
	}
	if strings.TrimSpace(input.Title) != "" {
		example["title"] = input.Title
	}
	if strings.TrimSpace(input.Description) != "" {
		example["description"] = input.Description
	}
	examples[index-1] = example
	cloned["examples"] = examples
	syncExampleRefs(cloned, examples)
	return cloned
}

func exampleIndexFromRole(assetRole string) int {
	if !strings.HasPrefix(assetRole, "example_") {
		return 0
	}
	number, err := strconv.Atoi(strings.TrimPrefix(assetRole, "example_"))
	if err != nil {
		return 0
	}
	return number
}

func cloneJSONMap(input map[string]any) map[string]any {
	encoded, _ := json.Marshal(input)
	out := map[string]any{}
	_ = json.Unmarshal(encoded, &out)
	return out
}

func ensureExamples(detail map[string]any) []any {
	examples, ok := detail["examples"].([]any)
	if !ok {
		examples = []any{}
	}
	return examples
}

func syncExampleRefs(detail map[string]any, examples []any) {
	sourceRefs := make([]string, 0)
	assetRefs := make([]string, 0)
	for _, rawExample := range examples {
		example, ok := rawExample.(map[string]any)
		if !ok {
			continue
		}
		if value := strings.TrimSpace(stringValue(example["sourceRef"])); value != "" {
			sourceRefs = append(sourceRefs, value)
		}
		if value := strings.TrimSpace(stringValue(example["assetRef"])); value != "" {
			assetRefs = append(assetRefs, value)
		}
	}
	detail["exampleSourceRefs"] = sourceRefs
	detail["exampleAssetRefs"] = assetRefs
}

type templateOpsStorageAssetRepo struct {
	db *gorm.DB
}

func (r *templateOpsStorageAssetRepo) FindStorageAssetBySource(productCode, category, sourceType, sourceRef string) (*models.StorageAsset, error) {
	var item models.StorageAsset
	if err := r.db.
		Where("product_code = ? AND category = ? AND source_type = ? AND source_ref = ?", productCode, category, sourceType, sourceRef).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func buildAssetChecks(input UpsertTemplateInput, finder storageAssetFinder) ([]CSVImportPreviewAssetCheck, int) {
	checks := make([]CSVImportPreviewAssetCheck, 0)
	if finder == nil {
		return checks, 0
	}
	detail := input.DetailRaw
	examples, _ := detail["examples"].([]any)
	missing := 0
	for _, rawExample := range examples {
		example, ok := rawExample.(map[string]any)
		if !ok {
			continue
		}
		sourceRef := stringValue(example["sourceRef"])
		if strings.TrimSpace(sourceRef) == "" {
			continue
		}
		check := CSVImportPreviewAssetCheck{
			ProductCode: input.ProductCode,
			Category:    "template-examples",
			SourceType:  "template_example",
			SourceRef:   sourceRef,
			Status:      "missing",
		}
		record, err := finder.FindStorageAssetBySource(input.ProductCode, check.Category, check.SourceType, sourceRef)
		if err == nil && record != nil {
			check.Status = "ready"
			check.StorageKey = record.StorageKey
		} else {
			missing++
		}
		checks = append(checks, check)
	}
	return checks, missing
}
