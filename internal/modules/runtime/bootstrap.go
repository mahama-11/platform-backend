package runtime

import (
	"errors"
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

func ensureProductEndpoint(repo *repository.RuntimeRepository, input config.BootstrapRuntimeProductEndpoint) error {
	if input.ProductCode == "" || input.BaseURL == "" {
		return nil
	}
	if input.CallbackKind == "" {
		return errors.New("runtime product endpoint callback_kind is required")
	}
	if item, err := repo.FindActiveProductEndpoint(input.ProductCode); err == nil {
		changed := false
		if item.CallbackKind != input.CallbackKind {
			item.CallbackKind = input.CallbackKind
			changed = true
		}
		if item.BaseURL != input.BaseURL {
			item.BaseURL = input.BaseURL
			changed = true
		}
		if input.Secret != "" && item.Secret != input.Secret {
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
	item := &models.RuntimeProductEndpoint{
		ID:           utils.GenerateID(),
		ProductCode:  input.ProductCode,
		CallbackKind: input.CallbackKind,
		BaseURL:      input.BaseURL,
		Secret:       input.Secret,
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
