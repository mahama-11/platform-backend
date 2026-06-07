package templateops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"platform-service/internal/models"
)

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
		BusinessGoal:   stringValue(raw["business_goal"]),
		InputSlots:     mapSliceValue(raw["input_slots"]),
		TargetOutputs:  mapSliceValue(raw["target_outputs"]),
		StrategyPolicy: mapValue(raw["strategy_policy"]),
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

func mapValue(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil
	}
	return result
}

func mapSliceValue(value any) []map[string]any {
	if result, ok := value.([]map[string]any); ok {
		return result
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result []map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil
	}
	return result
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
