package assetstorage

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
	if cfg == nil {
		return nil
	}
	repo := repository.NewRuntimeRepository(db)
	for _, item := range cfg.Bootstrap.Storage.Bindings {
		if err := ensureStorageBinding(repo, item); err != nil {
			return err
		}
	}
	return nil
}

func ensureStorageBinding(repo *repository.RuntimeRepository, input config.BootstrapStorageBinding) error {
	if input.ProductCode == "" || input.Category == "" || input.ProviderCode == "" {
		return nil
	}
	if _, err := repo.FindPreferredStorageBinding(input.ProductCode, input.Category); err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	item := &models.StorageBinding{
		ID:            utils.GenerateID(),
		ProductCode:   input.ProductCode,
		Category:      input.Category,
		ProviderCode:  input.ProviderCode,
		LocalBaseDir:  input.LocalBaseDir,
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
	return repo.CreateStorageBinding(item)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
