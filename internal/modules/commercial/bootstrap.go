package commercial

import (
	"errors"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

func SeedLocalDefaults(db *gorm.DB, cfg *config.Config) error {
	repo := repository.NewCommercialRepository(db)
	if cfg == nil {
		return nil
	}
	entityByCode := make(map[string]string)
	for _, item := range cfg.Bootstrap.Commercial.Products {
		if _, err := ensureProduct(repo, item); err != nil {
			return err
		}
	}
	for _, item := range cfg.Bootstrap.Commercial.CommercialEntities {
		entity, err := ensureCommercialEntity(repo, item)
		if err != nil {
			return err
		}
		entityByCode[item.Code] = entity.ID
	}
	for _, item := range cfg.Bootstrap.Commercial.BillingProfiles {
		if _, err := ensureBillingProfile(repo, item, entityByCode[item.CommercialEntityCode]); err != nil {
			return err
		}
	}
	for _, item := range cfg.Bootstrap.Commercial.BillableItems {
		if _, err := ensureCreditsBillableItem(repo, item); err != nil {
			return err
		}
	}
	return nil
}

func ensureProduct(repo *repository.CommercialRepository, input config.BootstrapProduct) (*models.Product, error) {
	var item models.Product
	if err := repo.DB().Where("code = ?", input.Code).First(&item).Error; err == nil {
		return &item, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item = models.Product{
		ID:        utils.GenerateID(),
		Code:      input.Code,
		Name:      input.Name,
		Status:    defaultString(input.Status, platformconst.StatusActive),
		OwnerTeam: defaultString(input.OwnerTeam, "platform"),
		Metadata:  defaultString(input.Metadata, "{}"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateProduct(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func ensureCommercialEntity(repo *repository.CommercialRepository, input config.BootstrapCommercialEntity) (*models.CommercialEntity, error) {
	var item models.CommercialEntity
	if err := repo.DB().Where("code = ?", input.Code).First(&item).Error; err == nil {
		return &item, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item = models.CommercialEntity{
		ID:          utils.GenerateID(),
		Code:        input.Code,
		Name:        input.Name,
		EntityType:  defaultString(input.EntityType, "internal"),
		CountryCode: defaultString(input.CountryCode, "CN"),
		Currency:    defaultString(input.Currency, "CNY"),
		Status:      defaultString(input.Status, platformconst.StatusActive),
		Metadata:    defaultString(input.Metadata, "{}"),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateCommercialEntity(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func ensureBillingProfile(repo *repository.CommercialRepository, input config.BootstrapBillingProfile, entityID string) (*models.BillingProfile, error) {
	if item, err := repo.FindBillingProfileByCode(input.Code); err == nil {
		return item, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item := &models.BillingProfile{
		ID:                 utils.GenerateID(),
		Code:               input.Code,
		ProductID:          input.ProductCode,
		CommercialEntityID: entityID,
		RegionScope:        defaultString(input.RegionScope, "CN"),
		Currency:           defaultString(input.Currency, "CNY"),
		PricingStrategy:    defaultString(input.PricingStrategy, "standard"),
		TaxStrategy:        defaultString(input.TaxStrategy, "default"),
		Status:             defaultString(input.Status, platformconst.StatusActive),
		Metadata:           defaultString(input.Metadata, "{}"),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := repo.CreateBillingProfile(item); err != nil {
		return nil, err
	}
	return item, nil
}

func ensureCreditsBillableItem(repo *repository.CommercialRepository, input config.BootstrapBillableItem) (*models.BillableItem, error) {
	if item, err := repo.FindBillableItemByCode(input.Code); err == nil {
		return item, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item := &models.BillableItem{
		ID:              utils.GenerateID(),
		ProductID:       input.ProductCode,
		Code:            input.Code,
		Name:            defaultString(input.Name, input.Code),
		MeterUnit:       defaultString(input.MeterUnit, "action"),
		BillingScope:    defaultString(input.BillingScope, platformconst.SubjectTypeOrganization),
		SettlementMode:  defaultString(input.SettlementMode, platformconst.SettlementModeCredits),
		PricingBehavior: defaultString(input.PricingBehavior, "standard"),
		Status:          defaultString(input.Status, platformconst.StatusActive),
		Metadata:        defaultString(input.Metadata, "{}"),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := repo.CreateBillableItem(item); err != nil {
		return nil, err
	}
	return item, nil
}
