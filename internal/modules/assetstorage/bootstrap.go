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

func findStorageBinding(repo *repository.RuntimeRepository, productCode, category string) (*models.StorageBinding, error) {
	var item models.StorageBinding
	if err := repo.DB().Where("product_code = ? AND category = ?", productCode, category).Order("created_at ASC").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func ensureStorageBinding(repo *repository.RuntimeRepository, input config.BootstrapStorageBinding) error {
	if input.ProductCode == "" || input.Category == "" || input.ProviderCode == "" {
		return nil
	}
	if item, err := findStorageBinding(repo, input.ProductCode, input.Category); err == nil {
		changed := false
		if item.ProviderCode != input.ProviderCode {
			item.ProviderCode = input.ProviderCode
			changed = true
		}
		if item.LocalBaseDir != input.LocalBaseDir {
			item.LocalBaseDir = input.LocalBaseDir
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
		return repo.SaveStorageBinding(item)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	item := &models.StorageBinding{
		ID:           utils.GenerateID(),
		ProductCode:  input.ProductCode,
		Category:     input.Category,
		ProviderCode: input.ProviderCode,
		LocalBaseDir: input.LocalBaseDir,
		Priority:     defaultInt(input.Priority, 100),
		Enabled:      input.Enabled,
		Metadata:     defaultString(input.Metadata, "{}"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
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
