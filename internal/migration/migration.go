package migration

import (
	"fmt"
	"sort"
	"time"

	"platform-service/internal/models"
	access "platform-service/internal/modules/access"
	"platform-service/internal/storage"

	"gorm.io/gorm"
)

const metadataTable = "schema_migrations"

type Step struct {
	Version int64
	Name    string
	Up      func(*gorm.DB) error
}

type Status struct {
	Version   int64
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

type record struct {
	Version   int64     `gorm:"primaryKey;autoIncrement:false"`
	Name      string    `gorm:"not null"`
	AppliedAt time.Time `gorm:"not null"`
}

func (record) TableName() string { return metadataTable }

func Steps() []Step {
	steps := []Step{
		{
			Version: 202604170001,
			Name:    "baseline_schema_bootstrap",
			Up: func(db *gorm.DB) error {
				return storage.RunSchemaBootstrap(db)
			},
		},
		{
			Version: 202604170002,
			Name:    "seed_access_defaults",
			Up: func(db *gorm.DB) error {
				return access.SeedDefaults(db)
			},
		},
		{
			Version: 202604170003,
			Name:    "backfill_credits_ledger_into_wallet",
			Up: func(db *gorm.DB) error {
				return backfillCreditsLedgerIntoWallet(db)
			},
		},
		{
			Version: 202604170004,
			Name:    "seed_menu_offerings",
			Up: func(db *gorm.DB) error {
				return seedMenuOfferings(db)
			},
		},
		{
			Version: 202604170005,
			Name:    "reconcile_menu_offerings_product_links",
			Up: func(db *gorm.DB) error {
				return seedMenuOfferings(db)
			},
		},
		{
			Version: 202604170006,
			Name:    "refresh_menu_offerings_landing_copy",
			Up: func(db *gorm.DB) error {
				return refreshMenuOfferingsLandingCopy(db)
			},
		},
		{
			Version: 202604170007,
			Name:    "refresh_menu_payment_cash_asset",
			Up: func(db *gorm.DB) error {
				return refreshMenuPaymentCashAsset(db)
			},
		},
		{
			Version: 202604170008,
			Name:    "seed_ecommerce_offerings",
			Up: func(db *gorm.DB) error {
				return seedEcommerceOfferings(db)
			},
		},
		{
			Version: 202604170009,
			Name:    "refresh_ecommerce_offerings_product_links",
			Up: func(db *gorm.DB) error {
				return seedEcommerceOfferings(db)
			},
		},
		{
			Version: 202604280001,
			Name:    "runtime_callback_delivery_schema",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.RuntimeCallbackDelivery{})
			},
		},
		{
			Version: 202604280002,
			Name:    "commercial_entitlement_schema",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.QuotaGrantPolicy{}, &models.PackageCapabilityPolicy{}, &models.CapabilityGrant{})
			},
		},
		{
			Version: 202604280003,
			Name:    "cleanup_legacy_allowance_policies",
			Up: func(db *gorm.DB) error {
				return db.Where("product_code IN ?", []string{"menu", "ecommerce"}).Delete(&models.AllowancePolicy{}).Error
			},
		},
		{
			Version: 202604280004,
			Name:    "commercial_policy_unique_indexes",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.QuotaGrantPolicy{}, &models.PackageCapabilityPolicy{})
			},
		},
		{
			Version: 202604290001,
			Name:    "split_platform_audit_logs",
			Up: func(db *gorm.DB) error {
				if err := storage.EnsurePlatformAuditTableIsolation(db); err != nil {
					return err
				}
				return db.AutoMigrate(&models.AuditLog{})
			},
		},
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].Version < steps[j].Version })
	return steps
}

func Up(db *gorm.DB) error {
	if err := ensureMetadataTable(db); err != nil {
		return err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}
	for _, step := range Steps() {
		if _, ok := applied[step.Version]; ok {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := step.Up(tx); err != nil {
				return err
			}
			return tx.Create(&record{
				Version:   step.Version,
				Name:      step.Name,
				AppliedAt: time.Now(),
			}).Error
		}); err != nil {
			return fmt.Errorf("apply migration %d_%s: %w", step.Version, step.Name, err)
		}
	}
	return nil
}

func CurrentVersion(db *gorm.DB) (int64, error) {
	if err := ensureMetadataTable(db); err != nil {
		return 0, err
	}
	var item record
	err := db.Order("version desc").First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return item.Version, nil
}

func ListStatus(db *gorm.DB) ([]Status, error) {
	if err := ensureMetadataTable(db); err != nil {
		return nil, err
	}
	var records []record
	if err := db.Order("version asc").Find(&records).Error; err != nil {
		return nil, err
	}
	byVersion := make(map[int64]record, len(records))
	for _, item := range records {
		byVersion[item.Version] = item
	}
	out := make([]Status, 0, len(Steps()))
	for _, step := range Steps() {
		status := Status{Version: step.Version, Name: step.Name}
		if item, ok := byVersion[step.Version]; ok {
			status.Applied = true
			appliedAt := item.AppliedAt
			status.AppliedAt = &appliedAt
		}
		out = append(out, status)
	}
	return out, nil
}

func ensureMetadataTable(db *gorm.DB) error {
	return db.AutoMigrate(&record{})
}

func appliedVersions(db *gorm.DB) (map[int64]struct{}, error) {
	var records []record
	if err := db.Find(&records).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]struct{}, len(records))
	for _, item := range records {
		out[item.Version] = struct{}{}
	}
	return out, nil
}
