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
