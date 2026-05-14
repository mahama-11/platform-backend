package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
)

type ProductRuntimeCallbackClient interface {
	UpdateJobRuntime(ctx context.Context, sourceID string, input ProductUpdateRuntimeInput) error
	RecordJobResults(ctx context.Context, sourceID string, input ProductRecordResultsInput) error
}

type ProductUpdateRuntimeInput struct {
	Status        string `json:"status,omitempty"`
	Stage         string `json:"stage,omitempty"`
	StageMessage  string `json:"stage_message,omitempty"`
	Progress      *int   `json:"progress,omitempty"`
	EtaSeconds    *int   `json:"eta_seconds,omitempty"`
	ProviderJobID string `json:"provider_job_id,omitempty"`
}

type ProductRecordResultAsset struct {
	AssetType      string         `json:"asset_type"`
	SourceType     string         `json:"source_type"`
	FileName       string         `json:"file_name,omitempty"`
	StorageKey     string         `json:"storage_key,omitempty"`
	StorageAssetID string         `json:"storage_asset_id,omitempty"`
	SourceURL      string         `json:"source_url"`
	PreviewURL     string         `json:"preview_url,omitempty"`
	MimeType       string         `json:"mime_type,omitempty"`
	FileSize       int64          `json:"file_size,omitempty"`
	Width          int            `json:"width,omitempty"`
	Height         int            `json:"height,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type ProductRecordResultVariant struct {
	Index      int                      `json:"index"`
	Status     string                   `json:"status"`
	IsSelected bool                     `json:"is_selected,omitempty"`
	Asset      ProductRecordResultAsset `json:"asset"`
}

type ProductRecordResultsInput struct {
	Status       string                       `json:"status"`
	Progress     int                          `json:"progress"`
	StageMessage string                       `json:"stage_message,omitempty"`
	Metadata     map[string]any               `json:"metadata,omitempty"`
	Variants     []ProductRecordResultVariant `json:"variants"`
}

type productHTTPCallbackClient struct {
	baseURL            string
	secret             string
	runtimePathBuilder func(sourceID string) string
	resultsPathBuilder func(sourceID string) string
	errorLabel         string
	client             *http.Client
}

func newProductHTTPCallbackClient(baseURL, secret, errorLabel string, runtimePathBuilder, resultsPathBuilder func(string) string) ProductRuntimeCallbackClient {
	return &productHTTPCallbackClient{
		baseURL:            strings.TrimRight(baseURL, "/"),
		secret:             secret,
		runtimePathBuilder: runtimePathBuilder,
		resultsPathBuilder: resultsPathBuilder,
		errorLabel:         defaultString(errorLabel, "product callback"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *productHTTPCallbackClient) UpdateJobRuntime(ctx context.Context, sourceID string, input ProductUpdateRuntimeInput) error {
	return c.post(ctx, c.runtimePathBuilder(sourceID), input)
}

func (c *productHTTPCallbackClient) RecordJobResults(ctx context.Context, sourceID string, input ProductRecordResultsInput) error {
	return c.post(ctx, c.resultsPathBuilder(sourceID), input)
}

func (c *productHTTPCallbackClient) post(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(platformconst.HeaderInternalServiceSecret, c.secret)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s failed: status=%d body=%s", c.errorLabel, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func buildProductCallbackClient(endpoint *models.RuntimeProductEndpoint) ProductRuntimeCallbackClient {
	if endpoint == nil {
		return nil
	}
	switch endpoint.CallbackKind {
	case "menu_internal":
		return newProductHTTPCallbackClient(
			endpoint.BaseURL,
			endpoint.Secret,
			"menu callback",
			func(sourceID string) string { return fmt.Sprintf("/internal/v1/menu/studio/jobs/%s/runtime", sourceID) },
			func(sourceID string) string { return fmt.Sprintf("/internal/v1/menu/studio/jobs/%s/results", sourceID) },
		)
	case "ecommerce_internal":
		return newProductHTTPCallbackClient(
			endpoint.BaseURL,
			endpoint.Secret,
			"ecommerce callback",
			func(sourceID string) string { return fmt.Sprintf("/internal/v1/ecommerce/jobs/%s/runtime", sourceID) },
			func(sourceID string) string { return fmt.Sprintf("/internal/v1/ecommerce/jobs/%s/results", sourceID) },
		)
	default:
		return nil
	}
}
