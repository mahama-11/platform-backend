package templateops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"platform-service/internal/models"

	"gorm.io/gorm"
)

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
