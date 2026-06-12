package app

import (
	"context"
	"fmt"
	"strings"

	"platform-service/internal/config"
	"platform-service/internal/migration"
	access "platform-service/internal/modules/access"
	assetstorage "platform-service/internal/modules/assetstorage"
	audit "platform-service/internal/modules/audit"
	"platform-service/internal/modules/catalog"
	commercial "platform-service/internal/modules/commercial"
	control "platform-service/internal/modules/control"
	identity "platform-service/internal/modules/identity"
	incentive "platform-service/internal/modules/incentive"
	metering "platform-service/internal/modules/metering"
	organization "platform-service/internal/modules/organization"
	productbilling "platform-service/internal/modules/productbilling"
	runtime "platform-service/internal/modules/runtime"
	templateops "platform-service/internal/modules/templateops"
	wallet "platform-service/internal/modules/wallet"
	"platform-service/internal/repository"
	"platform-service/internal/router"
	"platform-service/internal/storage"
	"platform-service/internal/telemetry"
	"platform-service/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type App struct {
	Config      config.Config
	DB          *gorm.DB
	RedisClient *redis.Client
	Router      *gin.Engine
	Shutdown    func(context.Context) error
}

func New(configFile string) (*App, error) {
	cfg, err := config.Load(configFile)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if err := validateDatabaseConfig(cfg.Database); err != nil {
		return nil, err
	}
	db, err := storage.InitDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}
	if cfg.Database.AutoMigrateEnabled {
		if err := migration.Up(db); err != nil {
			return nil, fmt.Errorf("run versioned migrations: %w", err)
		}
	}
	redisClient, err := storage.InitRedis(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("init redis: %w", err)
	}

	logger.Init(cfg.LogLevel, cfg.Monitoring.Tracing.ServiceName)
	shutdown, err := telemetry.InitTracing(cfg.Monitoring.Tracing)
	if err != nil {
		return nil, fmt.Errorf("init tracing: %w", err)
	}
	repo := repository.NewCoreRepository(db)
	commercialRepo := repository.NewCommercialRepository(db)
	controlRepo := repository.NewControlRepository(db)
	financeRepo := repository.NewFinanceRepository(db)
	runtimeRepo := repository.NewRuntimeRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	accessService := access.NewService(repo)
	auditService := audit.NewService(auditRepo)
	identityService := identity.NewService(repo, accessService, *cfg)
	orgService := organization.NewService(repo, identityService)
	catalogService := catalog.NewService(commercialRepo)
	commercialService := commercial.NewService(commercialRepo)
	walletService := wallet.NewService(financeRepo)
	controlService := control.NewService(controlRepo, walletService)
	incentiveService := incentive.NewService(financeRepo)
	meteringService := metering.NewService(commercialRepo, financeRepo, walletService)
	runtimeService := runtime.NewService(runtimeRepo, cfg.Runtime, cfg.Security, cfg.ComfyUIBridge)
	productBillingService := productbilling.NewService(commercialRepo, controlService, meteringService, runtimeService, walletService)
	assetStorageService := assetstorage.NewService(runtimeRepo)
	templateOpsService := templateops.NewService(*cfg, db)
	runtimeRegistry := runtime.NewProviderRegistry(cfg.Volcengine, cfg.ComfyUIBridge, cfg.GeminiVisual, cfg.GeminiImage, cfg.Minimax, cfg.MinimaxImage, cfg.KimiCoding, cfg.PaiVideo)
	runtimeService.UseRuntime(nil, runtimeRegistry)
	runtimeService.UseAssetStorage(assetStorageService)
	var runtimeQueue *runtime.AsynqRuntime
	if cfg.Runtime.WorkerEnabled {
		if redisClient == nil {
			return nil, fmt.Errorf("runtime worker enabled but redis is not configured")
		}
		runtimeQueue, err = runtime.NewAsynqRuntime(cfg.Redis, cfg.Runtime, runtimeService)
		if err != nil {
			return nil, fmt.Errorf("init runtime queue: %w", err)
		}
		runtimeService.UseRuntime(runtimeQueue, runtimeRegistry)
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	if cfg.Tasks.Enabled {
		walletService.StartLifecycleScheduler(lifecycleCtx, cfg.Tasks.ExpireInterval, cfg.Tasks.CycleInterval)
	}
	if runtimeQueue != nil {
		_ = runtimeQueue.Start()
	}

	app := &App{Config: *cfg, DB: db, RedisClient: redisClient, Shutdown: func(ctx context.Context) error {
		lifecycleCancel()
		if runtimeQueue != nil {
			runtimeQueue.Shutdown()
		}
		if shutdown != nil {
			return shutdown(ctx)
		}
		return nil
	}}
	app.Router = router.New(
		*cfg,
		assetstorage.NewHandler(assetStorageService),
		identity.NewHandler(identityService),
		organization.NewHandler(orgService),
		access.NewHandler(accessService),
		catalog.NewHandler(catalogService, financeRepo, auditService),
		commercial.NewHandler(commercialService, auditService),
		control.NewHandler(controlService, auditService),
		wallet.NewHandler(walletService, auditService),
		incentive.NewHandler(incentiveService, auditService),
		metering.NewHandler(meteringService, auditService),
		runtime.NewHandler(runtimeService, auditService),
		productbilling.NewHandler(productBillingService),
		templateops.NewHandler(templateOpsService),
		audit.NewHandler(auditService),
		identityService,
	)
	return app, nil
}

func validateDatabaseConfig(cfg config.DatabaseConfig) error {
	if strings.EqualFold(cfg.Driver, "sqlite") &&
		(cfg.Host != "database" || cfg.Port != 5432 || cfg.User != "platform" || cfg.DBName != "platform") {
		return fmt.Errorf("database.driver=sqlite but external database fields are configured; set database.driver=postgres to use host=%s dbname=%s", cfg.Host, cfg.DBName)
	}
	return nil
}
