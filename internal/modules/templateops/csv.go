package templateops

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"platform-service/internal/models"

	"gorm.io/gorm"
)

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
