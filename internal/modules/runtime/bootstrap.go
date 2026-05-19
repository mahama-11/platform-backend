package runtime

import (
	"errors"
	"fmt"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

func SeedLocalDefaults(db *gorm.DB, cfg *config.Config) error {
	repo := repository.NewRuntimeRepository(db)
	if cfg == nil {
		return nil
	}
	for _, item := range defaultProviderDefinitions(cfg) {
		if err := ensureProviderDefinition(repo, item); err != nil {
			return err
		}
	}
	for _, item := range cfg.Bootstrap.Runtime.ProductEndpoints {
		if err := ensureProductEndpoint(repo, item); err != nil {
			return err
		}
	}
	for _, item := range cfg.Bootstrap.Runtime.ProviderBindings {
		if err := ensureProviderBinding(repo, item); err != nil {
			return err
		}
	}
	return nil
}

func defaultProviderDefinitions(cfg *config.Config) []config.BootstrapRuntimeProviderDefinition {
	items := make([]config.BootstrapRuntimeProviderDefinition, 0, len(cfg.Bootstrap.Runtime.ProviderDefinitions)+len(cfg.Bootstrap.Runtime.ProviderBindings))
	items = append(items, cfg.Bootstrap.Runtime.ProviderDefinitions...)
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Code] = true
	}
	for _, binding := range cfg.Bootstrap.Runtime.ProviderBindings {
		code := binding.ProviderCode
		if code == "" || seen[code] {
			continue
		}
		items = append(items, config.BootstrapRuntimeProviderDefinition{
			Code:          code,
			Name:          code,
			ProviderType:  providerTypeForBootstrapCode(code),
			Mode:          providerModeForBootstrapCode(code),
			CredentialRef: credentialRefForBootstrapCode(code),
			Status:        "active",
			Capabilities:  "{}",
			Metadata:      `{"managed_by":"config_bootstrap","source":"provider_binding"}`,
		})
		seen[code] = true
	}
	return items
}

func providerTypeForBootstrapCode(code string) string {
	switch code {
	case "volcengine", "comfyui_bridge", "gemini_image_generation":
		return "image_generation"
	case "gemini_visual_understanding":
		return "image_understanding"
	case "kimi_coding_text", "minimax_text":
		return "text_generation"
	default:
		return "runtime"
	}
}

func providerModeForBootstrapCode(code string) string {
	if code == "comfyui_bridge" {
		return "async"
	}
	return "sync"
}

func credentialRefForBootstrapCode(code string) string {
	switch code {
	case "volcengine":
		return "config:volcengine.api_key"
	case "comfyui_bridge":
		return "config:comfyui_bridge.api_key"
	case "gemini_visual_understanding":
		return "config:gemini_visual.api_key"
	case "gemini_image_generation":
		return "config:gemini_image.api_key"
	case "kimi_coding_text":
		return "config:kimi_coding.api_key"
	case "minimax_text":
		return "config:minimax.api_key"
	default:
		return ""
	}
}

func ensureProviderDefinition(repo *repository.RuntimeRepository, input config.BootstrapRuntimeProviderDefinition) error {
	if input.Code == "" {
		return nil
	}
	if item, err := repo.FindProviderDefinitionByCode(input.Code); err == nil {
		changed := false
		if item.Name != defaultString(input.Name, input.Code) {
			item.Name = defaultString(input.Name, input.Code)
			changed = true
		}
		if item.ProviderType != defaultString(input.ProviderType, "runtime") {
			item.ProviderType = defaultString(input.ProviderType, "runtime")
			changed = true
		}
		if item.Mode != defaultString(input.Mode, "sync") {
			item.Mode = defaultString(input.Mode, "sync")
			changed = true
		}
		if item.CredentialRef != input.CredentialRef {
			item.CredentialRef = input.CredentialRef
			changed = true
		}
		if item.Capabilities != defaultString(input.Capabilities, "{}") {
			item.Capabilities = defaultString(input.Capabilities, "{}")
			changed = true
		}
		if item.Status != defaultString(input.Status, "active") {
			item.Status = defaultString(input.Status, "active")
			changed = true
		}
		if item.Metadata != defaultString(input.Metadata, "{}") {
			item.Metadata = defaultString(input.Metadata, "{}")
			changed = true
		}
		if !changed {
			return nil
		}
		item.UpdatedAt = time.Now()
		return repo.DB().Save(item).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	item := &models.RuntimeProviderDefinition{
		ID:            utils.GenerateID(),
		Code:          input.Code,
		Name:          defaultString(input.Name, input.Code),
		ProviderType:  defaultString(input.ProviderType, "runtime"),
		Mode:          defaultString(input.Mode, "sync"),
		CredentialRef: input.CredentialRef,
		Capabilities:  defaultString(input.Capabilities, "{}"),
		Status:        defaultString(input.Status, "active"),
		Metadata:      defaultString(input.Metadata, "{}"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	return repo.CreateProviderDefinition(item)
}

func findProductEndpointByProductCode(repo *repository.RuntimeRepository, productCode string) (*models.RuntimeProductEndpoint, error) {
	var item models.RuntimeProductEndpoint
	if err := repo.DB().Where("product_code = ?", productCode).Order("created_at ASC").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func bootstrapSecret(secret string) string {
	if isPlaceholderSecret(secret) {
		return ""
	}
	return secret
}

func isPlaceholderSecret(secret string) bool {
	return secret == "change-me-in-prod" || secret == "change-me-in-config" || secret == "change-me"
}

func ensureProductEndpoint(repo *repository.RuntimeRepository, input config.BootstrapRuntimeProductEndpoint) error {
	if input.ProductCode == "" || input.BaseURL == "" {
		return nil
	}
	if input.CallbackKind == "" {
		return errors.New("runtime product endpoint callback_kind is required")
	}
	if item, err := findProductEndpointByProductCode(repo, input.ProductCode); err == nil {
		changed := false
		if item.CallbackKind != input.CallbackKind {
			item.CallbackKind = input.CallbackKind
			changed = true
		}
		if item.BaseURL != input.BaseURL {
			item.BaseURL = input.BaseURL
			changed = true
		}
		if input.Secret != "" && !isPlaceholderSecret(input.Secret) && item.Secret != input.Secret {
			item.Secret = input.Secret
			changed = true
		}
		nextStatus := defaultString(input.Status, "active")
		if item.Status != nextStatus {
			item.Status = nextStatus
			changed = true
		}
		nextMetadata := defaultString(input.Metadata, "{}")
		if item.Metadata != nextMetadata {
			item.Metadata = nextMetadata
			changed = true
		}
		if !changed {
			return nil
		}
		item.UpdatedAt = time.Now()
		return repo.SaveProductEndpoint(item)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if input.CallbackKind != "" && input.Secret != "" && isPlaceholderSecret(input.Secret) {
		return fmt.Errorf("product endpoint %s uses placeholder callback secret", input.ProductCode)
	}
	item := &models.RuntimeProductEndpoint{
		ID:           utils.GenerateID(),
		ProductCode:  input.ProductCode,
		CallbackKind: input.CallbackKind,
		BaseURL:      input.BaseURL,
		Secret:       bootstrapSecret(input.Secret),
		Status:       defaultString(input.Status, "active"),
		Metadata:     defaultString(input.Metadata, "{}"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return repo.CreateProductEndpoint(item)
}

func ensureProviderBinding(repo *repository.RuntimeRepository, input config.BootstrapRuntimeProviderBinding) error {
	if input.ProductCode == "" || input.TaskType == "" || input.ProviderCode == "" {
		return nil
	}
	if item, err := repo.FindProviderBinding(input.ProductCode, input.TaskType, input.ProviderCode); err == nil {
		changed := false
		if item.Model != input.Model {
			item.Model = input.Model
			changed = true
		}
		if item.CredentialRef != input.CredentialRef {
			item.CredentialRef = input.CredentialRef
			changed = true
		}
		nextPriority := defaultInt(input.Priority, 100)
		if item.Priority != nextPriority {
			item.Priority = nextPriority
			changed = true
		}
		if item.Enabled != input.Enabled {
			item.Enabled = input.Enabled
			changed = true
		}
		nextMetadata := defaultString(input.Metadata, "{}")
		if item.Metadata != nextMetadata {
			item.Metadata = nextMetadata
			changed = true
		}
		if !changed {
			return nil
		}
		item.UpdatedAt = time.Now()
		return repo.SaveProviderBinding(item)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	item := &models.RuntimeProviderBinding{
		ID:            utils.GenerateID(),
		ProductCode:   input.ProductCode,
		TaskType:      input.TaskType,
		ProviderCode:  input.ProviderCode,
		Model:         input.Model,
		CredentialRef: input.CredentialRef,
		Priority:      defaultInt(input.Priority, 100),
		Enabled:       input.Enabled,
		Metadata:      defaultString(input.Metadata, "{}"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if !input.Enabled {
		item.Enabled = false
	} else {
		item.Enabled = true
	}
	return repo.CreateProviderBinding(item)
}
