package runtime

import (
	"testing"

	"platform-service/internal/models"
)

func TestDecodeEncodeRouteSnapshot(t *testing.T) {
	snapshot := decodeRouteSnapshot(`{"objective":"quality","preferred_providers":["comfyui_bridge"]}`)
	if snapshot.Objective != "quality" || len(snapshot.PreferredProviders) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if decodeRouteSnapshot("").Objective != "balanced" {
		t.Fatalf("expected balanced default objective")
	}
	encoded := encodeRouteSnapshot(snapshot)
	if decodeRouteSnapshot(encoded).Objective != "quality" {
		t.Fatalf("expected round-trip objective")
	}
}

func TestRankProviderBindingsPrefersPreferredAndScores(t *testing.T) {
	bindings := []models.RuntimeProviderBinding{
		{ProviderCode: "volcengine", Priority: 100, Metadata: `{"objective_scores":{"quality":80}}`},
		{ProviderCode: "comfyui_bridge", Priority: 50, Metadata: `{"objective_scores":{"quality":92}}`},
		{ProviderCode: "mock", Priority: 1, Metadata: `{"objective_scores":{"quality":10}}`},
	}
	ranked := rankProviderBindings(bindings, RuntimeRouteSnapshot{
		Objective:          "quality",
		PreferredProviders: []string{"volcengine"},
	})
	if ranked[0].ProviderCode != "volcengine" {
		t.Fatalf("expected preferred provider first, got %+v", ranked)
	}
}

func TestCandidateProviderCodesAndFallbackAllowed(t *testing.T) {
	bindings := []models.RuntimeProviderBinding{
		{ProviderCode: "comfyui_bridge"},
		{ProviderCode: "volcengine"},
		{ProviderCode: "comfyui_bridge"},
	}
	candidates := candidateProviderCodes(bindings)
	if len(candidates) != 2 || candidates[0] != "comfyui_bridge" || candidates[1] != "volcengine" {
		t.Fatalf("unexpected candidates: %#v", candidates)
	}
	if !fallbackAllowed(&models.RuntimeProviderBinding{Metadata: `{"fallback_on":["provider_timeout"]}`}, "provider_timeout") {
		t.Fatalf("expected explicit fallback rule to allow provider_timeout")
	}
	if fallbackAllowed(nil, "retryable_provider") {
		t.Fatalf("nil binding should not allow fallback")
	}
	if !fallbackAllowed(&models.RuntimeProviderBinding{Metadata: `{}`}, "retryable_provider") {
		t.Fatalf("default fallback should allow retryable_provider")
	}
}
