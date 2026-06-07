package runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"platform-service/internal/models"
	assetstorage "platform-service/internal/modules/assetstorage"
)

func TestExecutorProviderErrorClassificationBranches(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil error has no class", err: nil, want: ""},
		{name: "plain provider error defaults retryable", err: errors.New("temporary network failure"), want: "retryable_provider"},
		{name: "explicit non retryable provider error", err: newNonRetryableProviderError("invalid provider request"), want: "non_retryable_provider"},
		{name: "retryable provider error", err: newRetryableProviderError("temporary provider outage"), want: "retryable_provider"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyProviderErrorClass(tt.err); got != tt.want {
				t.Fatalf("classifyProviderErrorClass() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeAssetTypeForTaskContracts(t *testing.T) {
	tests := []struct {
		taskType string
		want     string
	}{
		{taskType: RuntimeTaskImageUnderstanding, want: "json"},
		{taskType: RuntimeTaskTextReasoning, want: "text"},
		{taskType: RuntimeTaskIntentPlanning, want: "text"},
		{taskType: RuntimeTaskPromptPlanning, want: "text"},
		{taskType: RuntimeTaskStrategyReport, want: "text"},
		{taskType: RuntimeTaskImageGeneration, want: "generated"},
		{taskType: "unknown_custom_task", want: "generated"},
	}

	for _, tt := range tests {
		t.Run(tt.taskType, func(t *testing.T) {
			if got := runtimeAssetTypeForTask(tt.taskType); got != tt.want {
				t.Fatalf("runtimeAssetTypeForTask(%q) = %q, want %q", tt.taskType, got, tt.want)
			}
		})
	}
}

func TestProviderCallbackMetadataSanitizerDropsSensitiveAndEmptyValues(t *testing.T) {
	raw := map[string]any{
		"safe":     "visible",
		"zero":     0,
		"false":    false,
		"api_key":  "should-not-leak",
		"":         "blank-key-should-drop",
		"internal": map[string]any{"trace": "drop-container"},
		"nested": map[string]any{
			"caption":        "front view",
			"token":          "drop-token",
			"only_sensitive": map[string]any{"storage_key": "drop-storage-key"},
		},
		"array": []any{
			nil,
			map[string]any{"password": "drop-password"},
			map[string]any{"label": "hero"},
			[]any{"inner", map[string]any{"billing": "drop-billing"}},
		},
	}

	sanitized := sanitizeProviderCallbackMetadata(raw)
	if sanitized == nil {
		t.Fatalf("expected sanitizer to preserve safe metadata")
	}
	body, _ := json.Marshal(sanitized)
	text := string(body)
	for _, leaked := range []string{"should-not-leak", "drop-token", "drop-storage-key", "drop-password", "drop-billing", "api_key", "storage_key", "password", "billing", "internal"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("sanitized metadata leaked %q: %s", leaked, text)
		}
	}
	if sanitized["safe"] != "visible" || sanitized["zero"] != 0 || sanitized["false"] != false {
		t.Fatalf("sanitizer dropped scalar business metadata: %+v", sanitized)
	}
	nested, ok := sanitized["nested"].(map[string]any)
	if !ok || !reflect.DeepEqual(nested, map[string]any{"caption": "front view"}) {
		t.Fatalf("unexpected nested sanitized metadata: %#v", sanitized["nested"])
	}
	array, ok := sanitized["array"].([]any)
	if !ok || len(array) != 2 {
		t.Fatalf("expected array to drop nil and sensitive-only entries, got %#v", sanitized["array"])
	}
	if sanitizeProviderCallbackMetadata(map[string]any{"secret": "drop", "wallet": map[string]any{"id": "w1"}}) != nil {
		t.Fatalf("sensitive-only metadata should sanitize to nil")
	}
}

func TestResolveProviderCodeShortCircuitErrorAndSnapshotReset(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)

	explicitJob := &models.RuntimeJob{ProviderCode: "manual", RouteSnapshot: `{"current_provider_idx":9}`}
	providerCode, err := service.resolveProviderCode(explicitJob)
	if err != nil {
		t.Fatalf("resolveProviderCode explicit provider returned error: %v", err)
	}
	if providerCode != "manual" {
		t.Fatalf("resolveProviderCode explicit provider = %q, want manual", providerCode)
	}
	if explicitJob.RouteSnapshot != `{"current_provider_idx":9}` {
		t.Fatalf("explicit provider should not rewrite route snapshot, got %s", explicitJob.RouteSnapshot)
	}

	missingJob := &models.RuntimeJob{ProductCode: "ecommerce", TaskType: RuntimeTaskImageGeneration}
	if _, err := service.resolveProviderCode(missingJob); err == nil || !strings.Contains(err.Error(), "no enabled provider binding") {
		t.Fatalf("expected no enabled binding error, got %v", err)
	}

	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{
		ID:           "provider-primary",
		ProductCode:  "ecommerce",
		TaskType:     RuntimeTaskImageGeneration,
		ProviderCode: "primary",
		Priority:     100,
		Enabled:      true,
	}); err != nil {
		t.Fatalf("CreateProviderBinding(primary): %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{
		ID:           "provider-secondary",
		ProductCode:  "ecommerce",
		TaskType:     RuntimeTaskImageGeneration,
		ProviderCode: "secondary",
		Priority:     10,
		Enabled:      true,
	}); err != nil {
		t.Fatalf("CreateProviderBinding(secondary): %v", err)
	}
	routedJob := &models.RuntimeJob{
		ProductCode:   "ecommerce",
		TaskType:      RuntimeTaskImageGeneration,
		RouteSnapshot: `{"current_provider_idx":99}`,
	}
	providerCode, err = service.resolveProviderCode(routedJob)
	if err != nil {
		t.Fatalf("resolveProviderCode routed job: %v", err)
	}
	if providerCode != "primary" {
		t.Fatalf("resolveProviderCode routed provider = %q, want primary", providerCode)
	}
	var snapshot RuntimeRouteSnapshot
	if err := json.Unmarshal([]byte(routedJob.RouteSnapshot), &snapshot); err != nil {
		t.Fatalf("route snapshot should be valid json: %v raw=%s", err, routedJob.RouteSnapshot)
	}
	if snapshot.CurrentProviderIdx != 0 || !reflect.DeepEqual(snapshot.CandidateProviders, []string{"primary", "secondary"}) {
		t.Fatalf("unexpected normalized route snapshot: %+v", snapshot)
	}
}

func TestOutputStorageCategoryDefaultsForNilJobAndUnconfiguredProduct(t *testing.T) {
	service, _, _ := newRuntimeServiceForTest(t)

	if got := service.outputStorageCategory(nil); got != "runtime-assets" {
		t.Fatalf("outputStorageCategory(nil) = %q, want runtime-assets", got)
	}
	if got := service.outputStorageCategory(&models.RuntimeJob{ProductCode: "unconfigured"}); got != "runtime-assets" {
		t.Fatalf("outputStorageCategory(unconfigured) = %q, want runtime-assets", got)
	}
}

func TestHydrateRuntimeSourceAssetsFromStorageKeys(t *testing.T) {
	service, repo, _ := newRuntimeServiceForTest(t)
	storage := assetstorage.NewService(repo)
	service.UseAssetStorage(storage)
	baseDir := t.TempDir()
	if err := repo.CreateStorageBinding(&models.StorageBinding{
		ID:           "source-runtime-assets",
		ProductCode:  "ecommerce",
		Category:     "runtime-assets",
		ProviderCode: "local",
		LocalBaseDir: baseDir,
		Priority:     1,
		Enabled:      true,
	}); err != nil {
		t.Fatalf("CreateStorageBinding: %v", err)
	}
	payload := []byte("source-image-bytes")
	stored, err := storage.UploadAsset(context.Background(), assetstorage.UploadAssetInput{
		ProductCode: "ecommerce",
		Category:    "runtime-assets",
		FileName:    "source.png",
		MimeType:    "image/png",
		Payload:     base64.StdEncoding.EncodeToString(payload),
	})
	if err != nil {
		t.Fatalf("UploadAsset: %v", err)
	}

	input := RuntimeInputManifest{SourceAssets: []ProviderSourceAsset{
		{StorageKey: stored.StorageKey, MimeType: stored.MimeType, SourceURL: "stale", PreviewURL: "stale"},
		{SourceURL: "https://cdn.example.com/remote.png", PreviewURL: "https://cdn.example.com/preview.png", MimeType: "image/png"},
	}}
	if err := service.hydrateRuntimeSourceAssets(&input); err != nil {
		t.Fatalf("hydrateRuntimeSourceAssets: %v", err)
	}
	wantPrefix := "data:image/png;base64,"
	if !strings.HasPrefix(input.SourceAssets[0].SourceURL, wantPrefix) || input.SourceAssets[0].SourceURL != input.SourceAssets[0].PreviewURL {
		t.Fatalf("expected storage-backed asset to hydrate to matching data URLs, got %+v", input.SourceAssets[0])
	}
	if !strings.Contains(input.SourceAssets[0].SourceURL, base64.StdEncoding.EncodeToString(payload)) {
		t.Fatalf("hydrated data URL does not contain stored payload: %s", input.SourceAssets[0].SourceURL)
	}
	if input.SourceAssets[1].SourceURL != "https://cdn.example.com/remote.png" || input.SourceAssets[1].PreviewURL != "https://cdn.example.com/preview.png" {
		t.Fatalf("asset without storage key should be left unchanged: %+v", input.SourceAssets[1])
	}

	badInput := RuntimeInputManifest{SourceAssets: []ProviderSourceAsset{{StorageKey: "invalid", MimeType: "image/png"}}}
	if err := service.hydrateRuntimeSourceAssets(&badInput); err == nil {
		t.Fatalf("expected invalid storage key hydration to return an error")
	}
	if err := service.hydrateRuntimeSourceAssets(nil); err != nil {
		t.Fatalf("nil input should be a no-op, got %v", err)
	}
}
