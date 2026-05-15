package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	access "platform-service/internal/modules/access"
	assetstorage "platform-service/internal/modules/assetstorage"
	commercial "platform-service/internal/modules/commercial"
	devseed "platform-service/internal/modules/devseed"
	runtime "platform-service/internal/modules/runtime"

	"github.com/go-redis/redis/v8"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func InitDB(appCfg *config.Config) (*gorm.DB, error) {
	cfg := appCfg.Database
	db, err := ConnectDB(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.AutoMigrateEnabled {
		if err := validateAutoMigratePolicy(cfg, appCfg.GinMode); err != nil {
			return nil, err
		}
		if err := RunSchemaBootstrap(db); err != nil {
			return nil, err
		}
	}
	if cfg.AutoMigrateEnabled {
		if err := RunAutoMigrateBootstrap(db, appCfg); err != nil {
			return nil, err
		}
	} else if appCfg.Bootstrap.SyncEnabled {
		if err := RunConfigSyncBootstrap(db, appCfg); err != nil {
			return nil, err
		}
	}
	return db, nil
}

func RunAutoMigrateBootstrap(db *gorm.DB, appCfg *config.Config) error {
	if err := access.SeedDefaults(db); err != nil {
		return fmt.Errorf("seed access defaults: %w", err)
	}
	if err := commercial.SeedLocalDefaults(db, appCfg); err != nil {
		return fmt.Errorf("seed commercial defaults: %w", err)
	}
	if err := runtime.SeedLocalDefaults(db, appCfg); err != nil {
		return fmt.Errorf("seed runtime defaults: %w", err)
	}
	if err := assetstorage.SeedLocalDefaults(db, appCfg); err != nil {
		return fmt.Errorf("seed storage defaults: %w", err)
	}
	if err := devseed.SeedLocalDefaults(db, appCfg); err != nil {
		return fmt.Errorf("seed dev identity defaults: %w", err)
	}
	return nil
}

func RunConfigSyncBootstrap(db *gorm.DB, appCfg *config.Config) error {
	if err := commercial.SeedLocalDefaults(db, appCfg); err != nil {
		return fmt.Errorf("sync commercial defaults: %w", err)
	}
	if err := runtime.SeedLocalDefaults(db, appCfg); err != nil {
		return fmt.Errorf("sync runtime defaults: %w", err)
	}
	if err := assetstorage.SeedLocalDefaults(db, appCfg); err != nil {
		return fmt.Errorf("sync storage defaults: %w", err)
	}
	return nil
}

func ConnectDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	newLogger := gormlogger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormlogger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	var (
		db  *gorm.DB
		err error
	)

	switch cfg.Driver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(cfg.SQLitePath), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
		db, err = gorm.Open(sqlite.Open(cfg.SQLitePath), &gorm.Config{Logger: newLogger})
	default:
		dsn := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host,
			cfg.Port,
			cfg.User,
			cfg.Password,
			cfg.DBName,
			cfg.SSLMode,
		)
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: newLogger})
	}
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	connMaxLifetime := cfg.ConnMaxLifetime
	if connMaxLifetime <= 0 {
		connMaxLifetime = 5 * time.Minute
	}
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	return db, nil
}

func RunSchemaBootstrap(db *gorm.DB) error {
	if err := preAutoMigrate(db); err != nil {
		return fmt.Errorf("pre auto migrate: %w", err)
	}
	if err := autoMigrate(db); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}

func validateAutoMigratePolicy(cfg config.DatabaseConfig, ginMode string) error {
	if !cfg.AutoMigrateEnabled {
		return nil
	}
	if strings.EqualFold(cfg.Driver, "sqlite") {
		return nil
	}
	if strings.EqualFold(ginMode, "debug") {
		return nil
	}
	if cfg.AllowStartupMigrate {
		return nil
	}
	return fmt.Errorf("startup auto migrate blocked for driver=%s gin_mode=%s: use explicit versioned migrations or set database.allow_startup_migrate_in_non_dev=true for break-glass only", cfg.Driver, ginMode)
}

func InitRedis(cfg config.RedisConfig) (*redis.Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return client, nil
}

func preAutoMigrate(db *gorm.DB) error {
	if err := EnsurePlatformAuditTableIsolation(db); err != nil {
		return err
	}
	if err := widenIncentiveCodeColumns(db); err != nil {
		return err
	}
	return nil
}

func EnsurePlatformAuditTableIsolation(db *gorm.DB) error {
	migrator := db.Migrator()
	platformTable := "platform_audit_logs"
	if migrator.HasTable(platformTable) {
		return nil
	}
	if !migrator.HasTable("audit_logs") {
		return nil
	}
	if !migrator.HasColumn("audit_logs", "target_type") {
		return nil
	}
	return db.Exec(`ALTER TABLE audit_logs RENAME TO platform_audit_logs`).Error
}

func widenIncentiveCodeColumns(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	statements := []string{
		`ALTER TABLE commission_ledgers ALTER COLUMN currency TYPE varchar(64)`,
		`ALTER TABLE referral_programs ALTER COLUMN commission_currency TYPE varchar(64)`,
		`ALTER TABLE referral_conversions ALTER COLUMN commission_currency TYPE varchar(64)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil && !strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return err
		}
	}
	return nil
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.AuditLog{},
		&models.User{},
		&models.Organization{},
		&models.OrganizationMember{},
		&models.Role{},
		&models.Permission{},
		&models.RolePermission{},
		&models.Product{},
		&models.SKU{},
		&models.CommercialPackage{},
		&models.BillableItem{},
		&models.RateCard{},
		&models.CommercialEntity{},
		&models.MerchantAccount{},
		&models.SettlementAccount{},
		&models.BillingProfile{},
		&models.RoutingPolicy{},
		&models.OrgBillingProfile{},
		&models.RuntimeProviderDefinition{},
		&models.RuntimeProductEndpoint{},
		&models.RuntimeProviderBinding{},
		&models.StorageBinding{},
		&models.StorageAsset{},
		&models.RuntimeJob{},
		&models.RuntimeAttempt{},
		&models.RuntimeCallbackDelivery{},
		&models.ChargeSession{},
		&models.MeterEvent{},
		&models.UsageRecord{},
		&models.UsageAgg{},
		&models.QuotaGrantPolicy{},
		&models.PackageCapabilityPolicy{},
		&models.CapabilityGrant{},
		&models.QuotaLedger{},
		&models.CreditsLedger{},
		&models.BillingLedger{},
		&models.ResourceReservation{},
		&models.SettlementRecord{},
		&models.AssetDefinition{},
		&models.AllowancePolicy{},
		&models.WalletAccount{},
		&models.WalletBucket{},
		&models.WalletLedger{},
		&models.DiscountLedger{},
		&models.RewardLedger{},
		&models.CommissionLedger{},
		&models.ReferralProgram{},
		&models.ReferralCode{},
		&models.ReferralConversion{},
		&models.ChannelPartner{},
		&models.ChannelProgram{},
		&models.ChannelPartnerProgram{},
		&models.ChannelPartnerBinding{},
		&models.ChannelCommissionPolicy{},
		&models.ChannelCommissionPolicyVersion{},
		&models.ChannelCommissionPolicyAssignment{},
		&models.ChannelProfitSnapshot{},
		&models.ChannelPolicyResolutionAudit{},
		&models.ChannelCommissionLedger{},
		&models.ChannelClawbackLedger{},
		&models.ChannelCommissionAdjustmentLedger{},
		&models.ChannelSettlementBatch{},
		&models.ChannelSettlementItem{},
		&models.ChannelSettlementItemLedger{},
		&models.ChannelSettlementItemClawback{},
		&models.ChannelSettlementItemAdjustment{},
		&models.TemplateProjection{},
	)
}
