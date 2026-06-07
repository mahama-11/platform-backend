package templateops

import (
	"fmt"
	"strings"
	"testing"

	assetstorage "platform-service/internal/modules/assetstorage"
)

func TestTemplateAssetBindingHelpersExtra(t *testing.T) {
	detail := map[string]any{
		"name":         "Template Name",
		"externalCode": "M1-T09",
		"toolBinding":  map[string]any{"toolSlug": "changing-model"},
		"examples": []any{
			map[string]any{"sourceRef": "templates/changing-model/M1-T09/example-1", "title": "One", "description": "desc"},
			"bad-example",
			map[string]any{"assetRef": "without-source"},
		},
	}
	items := buildTemplateAssetBindings(detail, "ecommerce")
	if len(items) != 1 || items[0].AssetRole != "example_1" || items[0].Title != "One" {
		t.Fatalf("unexpected bindings: %+v", items)
	}
	if got := inferSourceRefFromRole(detail, "example_2", 2); got != "templates/changing-model/M1-T09/example-2" {
		t.Fatalf("infer example source ref=%q", got)
	}
	if got := inferSourceRefFromRole(detail, "cover", 0); got != "templates/changing-model/M1-T09/cover" {
		t.Fatalf("infer cover source ref=%q", got)
	}
	if got := inferSourceRefFromRole(map[string]any{}, "example_1", 1); got != "" {
		t.Fatalf("expected empty source ref without tool/template code, got %q", got)
	}

	updated := upsertTemplateAssetBinding(detail, "example_3", UpsertTemplateAssetInput{StorageFileName: "example-3.png", AssetRef: "asset-ref-3"}, "templates/changing-model/M1-T09/example-3", &assetstorage.AssetRecord{ID: "asset-3", StorageKey: "ecommerce/template-examples/example-3.png", MimeType: "image/png", FileName: "example-3.png"})
	examples, _ := updated["examples"].([]any)
	if len(examples) < 3 {
		t.Fatalf("expected expanded examples, got %+v", examples)
	}
	example3, _ := examples[2].(map[string]any)
	if example3["assetId"] != "asset-3" || !strings.Contains(example3["previewAssetUrl"].(string), "storage_key=") {
		t.Fatalf("unexpected upserted example: %+v", example3)
	}
	if refs := updated["exampleAssetRefs"]; !strings.Contains(fmt.Sprintf("%v", refs), "asset-ref-3") {
		t.Fatalf("expected synced exampleAssetRefs, got %+v", refs)
	}

	cover := upsertTemplateAssetBinding(detail, "cover", UpsertTemplateAssetInput{AssetRef: "cover-ref"}, "templates/changing-model/M1-T09/cover", &assetstorage.AssetRecord{ID: "cover-asset", StorageKey: "ecommerce/template-examples/cover.png"})
	if cover["coverAssetId"] != "cover-asset" || cover["coverSourceRef"] == "" {
		t.Fatalf("unexpected cover binding: %+v", cover)
	}
	removedCover := removeTemplateAssetBinding(cover, "cover")
	if removedCover["coverAssetId"] != nil || removedCover["coverStorageKey"] != nil {
		t.Fatalf("cover fields should be removed: %+v", removedCover)
	}
	removedExample := removeTemplateAssetBinding(updated, "example_3")
	examples, _ = removedExample["examples"].([]any)
	example3, _ = examples[2].(map[string]any)
	if example3["sourceRef"] != nil || example3["storageKey"] != nil || example3["assetId"] != nil {
		t.Fatalf("example asset fields should be removed: %+v", example3)
	}
	unchanged := removeTemplateAssetBinding(updated, "unknown")
	if len(unchanged) == 0 {
		t.Fatalf("unknown role should keep cloned detail")
	}
}

func TestTemplateAssetSmallHelpersExtra(t *testing.T) {
	if exampleIndexFromRole("example_12") != 12 || exampleIndexFromRole("cover") != 0 || exampleIndexFromRole("example_bad") != 0 {
		t.Fatalf("unexpected exampleIndexFromRole")
	}
	items := ensureExamples(map[string]any{"examples": []any{map[string]any{"x": 1}}})
	if len(items) != 1 {
		t.Fatalf("expected converted examples, got %+v", items)
	}
	detail := map[string]any{"examples": []any{map[string]any{"sourceRef": "a", "assetRef": "asset-a"}}}
	syncExampleRefs(detail, ensureExamples(detail))
	if refs := detail["exampleAssetRefs"]; !strings.Contains(fmt.Sprintf("%v", refs), "asset-a") {
		t.Fatalf("expected synced refs, got %+v", refs)
	}
}
