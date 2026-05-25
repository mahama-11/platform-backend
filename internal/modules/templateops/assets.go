package templateops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	assetstorage "platform-service/internal/modules/assetstorage"
)

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
