package runtime

import (
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"
)

func TestEnsureProductEndpointRequiresCallbackKind(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	repo := repository.NewRuntimeRepository(db)
	err := ensureProductEndpoint(repo, config.BootstrapRuntimeProductEndpoint{
		ProductCode: "ecommerce",
		BaseURL:     "http://ecommerce",
	})
	if err == nil {
		t.Fatalf("expected callback_kind validation error")
	}
}

func TestSeedLocalDefaultsCreatesEndpointAndBinding(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	cfg := &config.Config{
		Bootstrap: config.BootstrapConfig{
			Runtime: config.RuntimeBootstrapConfig{
				ProductEndpoints: []config.BootstrapRuntimeProductEndpoint{
					{
						ProductCode:  "ecommerce",
						CallbackKind: "ecommerce_internal",
						BaseURL:      "http://ecommerce",
						Secret:       "secret",
					},
				},
				ProviderBindings: []config.BootstrapRuntimeProviderBinding{
					{
						ProductCode:  "ecommerce",
						TaskType:     "image_generation",
						ProviderCode: "comfyui_bridge",
						Priority:     50,
						Enabled:      true,
					},
				},
			},
		},
	}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	repo := repository.NewRuntimeRepository(db)
	if _, err := repo.FindActiveProductEndpoint("ecommerce"); err != nil {
		t.Fatalf("FindActiveProductEndpoint: %v", err)
	}
	bindings, err := repo.ListProviderBindings("ecommerce", "image_generation")
	if err != nil || len(bindings) != 1 || bindings[0].ProviderCode != "comfyui_bridge" {
		t.Fatalf("unexpected provider bindings: %+v err=%v", bindings, err)
	}
}

func TestEnsureProductEndpointUpdatesExistingActiveEndpoint(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	repo := repository.NewRuntimeRepository(db)
	item := &models.RuntimeProductEndpoint{
		ID:           "rpe_existing",
		ProductCode:  "menu",
		CallbackKind: "menu_internal",
		BaseURL:      "http://v-menu-backend-dev:8395",
		Secret:       "old-secret",
		Status:       platformconst.StatusActive,
		Metadata:     "{}",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.CreateProductEndpoint(item); err != nil {
		t.Fatalf("CreateProductEndpoint: %v", err)
	}

	err := ensureProductEndpoint(repo, config.BootstrapRuntimeProductEndpoint{
		ProductCode:  "menu",
		CallbackKind: "menu_internal",
		BaseURL:      "http://v-menu-backend-dev:8196",
		Secret:       "menu-service-secret",
		Status:       "active",
		Metadata:     "{\"output_storage_category\":\"studio-assets\"}",
	})
	if err != nil {
		t.Fatalf("ensureProductEndpoint: %v", err)
	}

	updated, err := repo.FindActiveProductEndpoint("menu")
	if err != nil {
		t.Fatalf("FindActiveProductEndpoint: %v", err)
	}
	if updated.BaseURL != "http://v-menu-backend-dev:8196" {
		t.Fatalf("expected updated base_url, got %s", updated.BaseURL)
	}
	if updated.Secret != "menu-service-secret" {
		t.Fatalf("expected updated secret, got %s", updated.Secret)
	}
}

func TestSeedLocalDefaultsConvergesProviderDefinitions(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	cfg := &config.Config{Bootstrap: config.BootstrapConfig{Runtime: config.RuntimeBootstrapConfig{ProviderBindings: []config.BootstrapRuntimeProviderBinding{
		{ProductCode: "ecommerce", TaskType: "text_reasoning", ProviderCode: "kimi_coding_text", Enabled: true},
		{ProductCode: "ecommerce", TaskType: "image_generation", ProviderCode: "comfyui_bridge", Enabled: true},
	}}}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	repo := repository.NewRuntimeRepository(db)
	for _, code := range []string{"comfyui_bridge", "kimi_coding_text"} {
		def, err := repo.FindProviderDefinitionByCode(code)
		if err != nil {
			t.Fatalf("expected provider definition %s: %v", code, err)
		}
		if def.Status != platformconst.StatusActive {
			t.Fatalf("expected active provider %s, got %s", code, def.Status)
		}
		if def.Capabilities != "{}" {
			t.Fatalf("provider definition should stay provider-generic, got capabilities=%s", def.Capabilities)
		}
	}
}

func TestEnqueueCallbackDeliveryFailsWhenProductEndpointMissing(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	service := NewService(repository.NewRuntimeRepository(db), runtimeConfigForTest(), runtimeSecurityForTest(), runtimeComfyForTest())
	job := &models.RuntimeJob{
		ID:             "rt_missing_callback",
		ProductCode:    "ecommerce",
		TaskType:       "text_reasoning",
		ProviderCode:   "kimi_coding_text",
		ProviderMode:   "sync",
		OrganizationID: "org-1",
		SourceType:     "smoke",
		SourceID:       "source-1",
		Status:         platformconst.StatusProcessing,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := service.notifyProductResults(job, ProductRecordResultsInput{Status: platformconst.StatusCompleted}); err == nil {
		t.Fatalf("expected missing callback endpoint error")
	}
	var deliveries int64
	if err := db.Model(&models.RuntimeCallbackDelivery{}).Count(&deliveries).Error; err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if deliveries != 0 {
		t.Fatalf("missing endpoint should not create a silently pending delivery, got %d", deliveries)
	}
}

func TestEnsureProductEndpointRepairsInactiveEndpointAndPreservesSecretPlaceholder(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	repo := repository.NewRuntimeRepository(db)
	item := &models.RuntimeProductEndpoint{
		ID:           "rpe_inactive",
		ProductCode:  "ecommerce",
		CallbackKind: "ecommerce_internal",
		BaseURL:      "http://v-ecommerce-backend-dev:8396",
		Secret:       "real-secret",
		Status:       "inactive",
		Metadata:     "{}",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.CreateProductEndpoint(item); err != nil {
		t.Fatalf("CreateProductEndpoint: %v", err)
	}
	if err := ensureProductEndpoint(repo, config.BootstrapRuntimeProductEndpoint{ProductCode: "ecommerce", CallbackKind: "ecommerce_internal", BaseURL: "http://v-ecommerce-backend:8296", Secret: "change-me-in-prod", Status: "active"}); err != nil {
		t.Fatalf("ensureProductEndpoint: %v", err)
	}
	updated, err := repo.FindActiveProductEndpoint("ecommerce")
	if err != nil {
		t.Fatalf("FindActiveProductEndpoint: %v", err)
	}
	if updated.ID != "rpe_inactive" || updated.BaseURL != "http://v-ecommerce-backend:8296" || updated.Secret != "real-secret" {
		t.Fatalf("endpoint was not repaired safely: %+v", updated)
	}
	var count int64
	if err := db.Model(&models.RuntimeProductEndpoint{}).Where("product_code = ?", "ecommerce").Count(&count).Error; err != nil {
		t.Fatalf("count endpoints: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected repaired endpoint without duplicate, got count=%d", count)
	}
}

func TestRuntimeBootstrapConvergesProviderDefinitionBindingAndPlaceholderRules(t *testing.T) {
	db := newRuntimeFullTestDB(t)
	repo := repository.NewRuntimeRepository(db)
	if err := repo.CreateProviderDefinition(&models.RuntimeProviderDefinition{ID: "def-old", Code: "minimax_text", Name: "Old", ProviderType: "runtime", Mode: "async", CredentialRef: "old", Capabilities: `{"old":true}`, Status: "inactive", Metadata: "{}", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatalf("seed provider definition: %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{ID: "bind-old", ProductCode: "ecommerce", TaskType: "prompt_planning", ProviderCode: "minimax_text", Model: "old-model", CredentialRef: "old", Priority: 10, Enabled: true, Metadata: "{}", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatalf("seed binding: %v", err)
	}
	cfg := &config.Config{Bootstrap: config.BootstrapConfig{Runtime: config.RuntimeBootstrapConfig{
		ProviderDefinitions: []config.BootstrapRuntimeProviderDefinition{{Code: "minimax_text", Name: "MiniMax", ProviderType: "text_generation", Mode: "sync", CredentialRef: "config:minimax.api_key", Capabilities: `{"json":true}`, Status: "active", Metadata: `{"managed":true}`}},
		ProductEndpoints:    []config.BootstrapRuntimeProductEndpoint{{ProductCode: "menu", CallbackKind: "menu_internal", BaseURL: "http://menu", Secret: "real-secret", Metadata: `{"managed":true}`}, {ProductCode: "skip-empty", BaseURL: ""}},
		ProviderBindings: []config.BootstrapRuntimeProviderBinding{
			{ProductCode: "ecommerce", TaskType: "prompt_planning", ProviderCode: "minimax_text", Model: "MiniMax-M2", CredentialRef: "config:minimax.api_key", Priority: 25, Enabled: false, Metadata: `{"managed":true}`},
			{ProductCode: "ecommerce", TaskType: "image_understanding", ProviderCode: "gemini_visual_understanding", Priority: 5, Enabled: true},
			{ProductCode: "", TaskType: "image_generation", ProviderCode: "skip"},
		},
	}}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	def, err := repo.FindProviderDefinitionByCode("minimax_text")
	if err != nil || def.Name != "MiniMax" || def.Mode != "sync" || def.Status != "active" || def.Metadata != `{"managed":true}` {
		t.Fatalf("provider definition did not converge: %+v err=%v", def, err)
	}
	visual, err := repo.FindProviderDefinitionByCode("gemini_visual_understanding")
	if err != nil || visual.ProviderType != "image_understanding" || visual.CredentialRef != "config:gemini_visual.api_key" {
		t.Fatalf("derived visual provider definition mismatch: %+v err=%v", visual, err)
	}
	binding, err := repo.FindProviderBinding("ecommerce", "prompt_planning", "minimax_text")
	if err != nil || binding.Model != "MiniMax-M2" || binding.Priority != 25 || binding.Enabled || binding.Metadata != `{"managed":true}` {
		t.Fatalf("provider binding did not converge: %+v err=%v", binding, err)
	}
	endpoint, err := repo.FindActiveProductEndpoint("menu")
	if err != nil || endpoint.Secret != "real-secret" || endpoint.Metadata != `{"managed":true}` {
		t.Fatalf("endpoint mismatch: %+v err=%v", endpoint, err)
	}
	if err := ensureProductEndpoint(repo, config.BootstrapRuntimeProductEndpoint{ProductCode: "new-placeholder", CallbackKind: "internal", BaseURL: "http://x", Secret: "change-me"}); err == nil {
		t.Fatalf("expected placeholder callback secret to be rejected for new endpoint")
	}
	if !isPlaceholderSecret("change-me-in-config") || bootstrapSecret("change-me") != "" || bootstrapSecret("real") != "real" {
		t.Fatalf("unexpected placeholder helper behavior")
	}
	for _, code := range []string{"volcengine", "gemini_image_generation", "minimax_image_generation", "unknown"} {
		if providerTypeForBootstrapCode(code) == "" || providerModeForBootstrapCode(code) == "" {
			t.Fatalf("expected bootstrap provider helpers for %s", code)
		}
	}
}
