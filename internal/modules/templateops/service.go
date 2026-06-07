package templateops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	TemplateRef    string           `json:"template_ref"`
	ProductCode    string           `json:"product_code"`
	TemplateID     string           `json:"template_id"`
	Slug           string           `json:"slug"`
	Name           string           `json:"name"`
	Summary        string           `json:"summary"`
	Status         string           `json:"status"`
	CoverAssetURL  string           `json:"cover_asset_url,omitempty"`
	CoverAssetID   string           `json:"cover_asset_id,omitempty"`
	RecommendScore int              `json:"recommend_score"`
	Tags           []string         `json:"tags,omitempty"`
	Platforms      []string         `json:"platforms,omitempty"`
	Series         string           `json:"series,omitempty"`
	CapabilityType string           `json:"capability_type,omitempty"`
	Modality       string           `json:"modality,omitempty"`
	Scope          string           `json:"scope,omitempty"`
	ManagedSource  string           `json:"managed_source,omitempty"`
	BusinessGoal   string           `json:"business_goal,omitempty"`
	InputSlots     []map[string]any `json:"input_slots,omitempty"`
	TargetOutputs  []map[string]any `json:"target_outputs,omitempty"`
	StrategyPolicy map[string]any   `json:"strategy_policy,omitempty"`
	Raw            map[string]any   `json:"raw,omitempty"`
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
