package router

import (
	"platform-service/internal/config"
	"platform-service/internal/middleware"
	access "platform-service/internal/modules/access"
	assetstorage "platform-service/internal/modules/assetstorage"
	audit "platform-service/internal/modules/audit"
	catalog "platform-service/internal/modules/catalog"
	commercial "platform-service/internal/modules/commercial"
	control "platform-service/internal/modules/control"
	docs "platform-service/internal/modules/docs"
	identity "platform-service/internal/modules/identity"
	incentive "platform-service/internal/modules/incentive"
	metering "platform-service/internal/modules/metering"
	organization "platform-service/internal/modules/organization"
	runtime "platform-service/internal/modules/runtime"
	templateops "platform-service/internal/modules/templateops"
	wallet "platform-service/internal/modules/wallet"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func New(
	cfg config.Config,
	assetStorageHandler *assetstorage.Handler,
	identityHandler *identity.Handler,
	orgHandler *organization.Handler,
	accessHandler *access.Handler,
	catalogHandler *catalog.Handler,
	commercialHandler *commercial.Handler,
	controlHandler *control.Handler,
	walletHandler *wallet.Handler,
	incentiveHandler *incentive.Handler,
	meteringHandler *metering.Handler,
	runtimeHandler *runtime.Handler,
	templateOpsHandler *templateops.Handler,
	auditHandler *audit.Handler,
	identityService *identity.Service,
) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	r := gin.New()
	serviceName := cfg.Monitoring.Tracing.ServiceName
	if serviceName == "" {
		serviceName = "platform-service"
	}
	r.Use(otelgin.Middleware(serviceName))
	r.Use(middleware.RequestContext(), middleware.BodySizeLimit(cfg.Security.MaxBodyBytes), middleware.RateLimit(cfg.Security.RateLimitPerSecond, cfg.Security.RateLimitBurst), middleware.Metrics(cfg.Monitoring.Metrics.Namespace, cfg.Monitoring.Metrics.Subsystem), middleware.AccessLog(), gin.Recovery())

	healthHandler := func(c *gin.Context) {
		response.JSONSuccess(c, gin.H{"service": "v-platform-backend", "status": "ok"})
	}
	r.GET("/healthz", healthHandler)
	r.HEAD("/healthz", healthHandler)
	if cfg.Monitoring.Metrics.Enabled {
		r.GET(cfg.Monitoring.Metrics.Path, middleware.MetricsHandler(cfg.Monitoring.Metrics.Namespace, cfg.Monitoring.Metrics.Subsystem))
	}
	docHandler := docs.NewHandler()
	r.GET("/docs", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, `<!doctype html><html><head><title>Platform Docs</title></head><body><h1>Platform Docs</h1><ul><li><a href="/docs/internal-access">Internal Access Guide</a></li><li><a href="/docs/error-codes">Error Codes</a></li><li><a href="/api/v1/docs/internal-access">API v1 Internal Access Guide</a></li><li><a href="/api/v1/docs/error-codes">API v1 Error Codes</a></li></ul><p>Internal OpenAPI generation is available via the repository scripts, and these endpoints provide the current browser-readable docs entry.</p></body></html>`)
	})
	r.GET("/docs/internal-access", docHandler.InternalAccessDoc)
	r.GET("/docs/error-codes", docHandler.ErrorCodesDoc)

	v1 := r.Group("/api/v1")
	{
		docsGroup := v1.Group("/docs")
		{
			docsGroup.GET("/internal-access", docHandler.InternalAccessDoc)
			docsGroup.GET("/error-codes", docHandler.ErrorCodesDoc)
		}
		v1.POST("/runtime/providers/:providerCode/callback", runtimeHandler.ProviderCallback)

		auth := v1.Group("/auth")
		{
			auth.POST("/register", identityHandler.Register)
			auth.POST("/login", identityHandler.Login)
			auth.GET("/me", middleware.JWTAuth(identityService, cfg.Security.JWTSecret), identityHandler.Me)
		}

		orgs := v1.Group("/orgs")
		orgs.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret))
		{
			orgs.GET("", orgHandler.List)
			orgs.POST("/switch", orgHandler.Switch)
		}

		accessGroup := v1.Group("/access")
		accessGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret))
		{
			accessGroup.GET("/permissions/me", accessHandler.MePermissions)
			accessGroup.GET("/permissions", middleware.RequirePermission("platform.admin"), accessHandler.ListPermissions)
			accessGroup.POST("/permissions", middleware.RequirePermission("platform.admin"), accessHandler.CreatePermission)
			accessGroup.PUT("/permissions/:permissionID", middleware.RequirePermission("platform.admin"), accessHandler.UpdatePermission)
			accessGroup.DELETE("/permissions/:permissionID", middleware.RequirePermission("platform.admin"), accessHandler.DeletePermission)
			accessGroup.GET("/roles", middleware.RequirePermission("platform.admin"), accessHandler.ListRoles)
			accessGroup.POST("/roles", middleware.RequirePermission("platform.admin"), accessHandler.CreateRole)
			accessGroup.PUT("/roles/:roleID", middleware.RequirePermission("platform.admin"), accessHandler.UpdateRole)
			accessGroup.DELETE("/roles/:roleID", middleware.RequirePermission("platform.admin"), accessHandler.DeleteRole)
			accessGroup.GET("/roles/:roleID/permissions", middleware.RequirePermission("platform.admin"), accessHandler.GetRolePermissions)
			accessGroup.PUT("/roles/:roleID/permissions", middleware.RequirePermission("platform.admin"), accessHandler.SetRolePermissions)
		}

		catalogGroup := v1.Group("/catalog")
		catalogGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			catalogGroup.GET("/products", catalogHandler.ListProducts)
			catalogGroup.POST("/products", catalogHandler.CreateProduct)
			catalogGroup.GET("/offerings", catalogHandler.Offerings)
			catalogGroup.PUT("/products/:productID", catalogHandler.UpdateProduct)
			catalogGroup.DELETE("/products/:productID", catalogHandler.DeleteProduct)
			catalogGroup.GET("/skus", catalogHandler.ListSKUs)
			catalogGroup.POST("/skus", catalogHandler.CreateSKU)
			catalogGroup.PUT("/skus/:skuID", catalogHandler.UpdateSKU)
			catalogGroup.DELETE("/skus/:skuID", catalogHandler.DeleteSKU)
			catalogGroup.GET("/billable-items", catalogHandler.ListBillableItems)
			catalogGroup.POST("/billable-items", catalogHandler.CreateBillableItem)
			catalogGroup.PUT("/billable-items/:billableItemID", catalogHandler.UpdateBillableItem)
			catalogGroup.DELETE("/billable-items/:billableItemID", catalogHandler.DeleteBillableItem)
			catalogGroup.GET("/packages", catalogHandler.ListPackages)
			catalogGroup.POST("/packages", catalogHandler.CreatePackage)
			catalogGroup.PUT("/packages/:packageID", catalogHandler.UpdatePackage)
			catalogGroup.DELETE("/packages/:packageID", catalogHandler.DeletePackage)
			catalogGroup.GET("/rate-cards", catalogHandler.ListRateCards)
			catalogGroup.POST("/rate-cards", catalogHandler.CreateRateCard)
			catalogGroup.PUT("/rate-cards/:rateCardID", catalogHandler.UpdateRateCard)
			catalogGroup.DELETE("/rate-cards/:rateCardID", catalogHandler.DeleteRateCard)
		}

		assetsGroup := v1.Group("/assets")
		assetsGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			assetsGroup.GET("/metadata", assetStorageHandler.GetAssetMetadata)
			assetsGroup.GET("/content", assetStorageHandler.GetAssetContent)
		}

		commercialGroup := v1.Group("/commercial")
		commercialGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			commercialGroup.GET("/entities", commercialHandler.ListCommercialEntities)
			commercialGroup.POST("/entities", commercialHandler.CreateCommercialEntity)
			commercialGroup.GET("/billing-profiles", commercialHandler.ListBillingProfiles)
			commercialGroup.POST("/billing-profiles", commercialHandler.CreateBillingProfile)
			commercialGroup.GET("/routing-policies", commercialHandler.ListRoutingPolicies)
			commercialGroup.POST("/routing-policies", commercialHandler.CreateRoutingPolicy)
			commercialGroup.PUT("/routing-policies/:routingPolicyID", commercialHandler.UpdateRoutingPolicy)
			commercialGroup.DELETE("/routing-policies/:routingPolicyID", commercialHandler.DeleteRoutingPolicy)
			commercialGroup.POST("/route/resolve", commercialHandler.ResolveRoute)
		}

		controlGroup := v1.Group("/controls")
		controlGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			controlGroup.POST("/quota/grants", controlHandler.GrantQuota)
			controlGroup.GET("/quota/balance", controlHandler.QuotaBalance)
			controlGroup.GET("/quota/policies", controlHandler.ListQuotaGrantPolicies)
			controlGroup.POST("/quota/policies", controlHandler.CreateQuotaGrantPolicy)
			controlGroup.PUT("/quota/policies/:policyID", controlHandler.UpdateQuotaGrantPolicy)
			controlGroup.DELETE("/quota/policies/:policyID", controlHandler.DeleteQuotaGrantPolicy)
			controlGroup.POST("/credits/grants", controlHandler.GrantCredits)
			controlGroup.GET("/credits/balance", controlHandler.CreditsBalance)
			controlGroup.POST("/package-activations", controlHandler.ActivatePackage)
			controlGroup.GET("/capability/policies", controlHandler.ListPackageCapabilityPolicies)
			controlGroup.POST("/capability/policies", controlHandler.CreatePackageCapabilityPolicy)
			controlGroup.PUT("/capability/policies/:policyID", controlHandler.UpdatePackageCapabilityPolicy)
			controlGroup.DELETE("/capability/policies/:policyID", controlHandler.DeletePackageCapabilityPolicy)
			controlGroup.POST("/capability/grants", controlHandler.GrantCapability)
			controlGroup.GET("/capability/resolve", controlHandler.ResolveCapability)
			controlGroup.POST("/reservations", controlHandler.Reserve)
			controlGroup.POST("/reservations/:reservationID/commit", controlHandler.CommitReservation)
			controlGroup.POST("/reservations/:reservationID/release", controlHandler.ReleaseReservation)
		}

		walletGroup := v1.Group("/wallet")
		walletGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			walletGroup.GET("/assets", walletHandler.ListAssetDefinitions)
			walletGroup.POST("/assets", walletHandler.CreateAssetDefinition)
			walletGroup.PUT("/assets/:assetCode", walletHandler.UpdateAssetDefinition)
			walletGroup.DELETE("/assets/:assetCode", walletHandler.DeleteAssetDefinition)
			walletGroup.GET("/allowance-policies", walletHandler.ListAllowancePolicies)
			walletGroup.POST("/allowance-policies", walletHandler.CreateAllowancePolicy)
			walletGroup.PUT("/allowance-policies/:policyID", walletHandler.UpdateAllowancePolicy)
			walletGroup.DELETE("/allowance-policies/:policyID", walletHandler.DeleteAllowancePolicy)
			walletGroup.GET("/accounts", walletHandler.ListAccounts)
			walletGroup.POST("/accounts", walletHandler.CreateAccount)
			walletGroup.GET("/summary", walletHandler.GetSummary)
			walletGroup.GET("/buckets", walletHandler.ListBuckets)
			walletGroup.GET("/ledger", walletHandler.ListLedger)
			walletGroup.POST("/ledger", walletHandler.PostLedger)
			walletGroup.POST("/cycle-allowances", walletHandler.GrantCycleAllowance)
			walletGroup.POST("/expire", walletHandler.ExpireBuckets)
			walletGroup.POST("/lifecycle/run", walletHandler.RunLifecycle)
		}

		incentiveGroup := v1.Group("/incentives")
		incentiveGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			incentiveGroup.GET("/rewards", incentiveHandler.ListRewards)
			incentiveGroup.POST("/rewards", incentiveHandler.CreateReward)
			incentiveGroup.PUT("/rewards/:rewardID", incentiveHandler.UpdateReward)
			incentiveGroup.GET("/commissions", incentiveHandler.ListCommissions)
			incentiveGroup.POST("/commissions", incentiveHandler.CreateCommission)
			incentiveGroup.POST("/commissions/redeem", incentiveHandler.RedeemCommissions)
			incentiveGroup.PUT("/commissions/:commissionID", incentiveHandler.UpdateCommission)
			incentiveGroup.GET("/referral-programs", incentiveHandler.ListReferralPrograms)
			incentiveGroup.POST("/referral-programs", incentiveHandler.CreateReferralProgram)
			incentiveGroup.PUT("/referral-programs/:programID", incentiveHandler.UpdateReferralProgram)
			incentiveGroup.GET("/referral-codes", incentiveHandler.ListReferralCodes)
			incentiveGroup.GET("/referral-codes/:code/resolve", incentiveHandler.ResolveReferralCode)
			incentiveGroup.POST("/referral-codes", incentiveHandler.CreateReferralCode)
			incentiveGroup.PUT("/referral-codes/:code", incentiveHandler.UpdateReferralCode)
			incentiveGroup.GET("/referral-conversions", incentiveHandler.ListReferralConversions)
			incentiveGroup.POST("/referral-conversions", incentiveHandler.CreateReferralConversion)
			incentiveGroup.PUT("/referral-conversions/:conversionID", incentiveHandler.UpdateReferralConversion)
			incentiveGroup.GET("/channel-partners", incentiveHandler.ListChannelPartners)
			incentiveGroup.POST("/channel-partners", incentiveHandler.CreateChannelPartner)
			incentiveGroup.GET("/channel-programs", incentiveHandler.ListChannelPrograms)
			incentiveGroup.POST("/channel-programs", incentiveHandler.CreateChannelProgram)
			incentiveGroup.GET("/channel-bindings", incentiveHandler.ListChannelBindings)
			incentiveGroup.POST("/channel-bindings", incentiveHandler.CreateChannelBinding)
			incentiveGroup.GET("/channel-policies", incentiveHandler.ListChannelCommissionPolicies)
			incentiveGroup.POST("/channel-policies", incentiveHandler.CreateChannelCommissionPolicy)
			incentiveGroup.GET("/channel-policy-versions", incentiveHandler.ListChannelCommissionPolicyVersions)
			incentiveGroup.POST("/channel-policy-versions", incentiveHandler.CreateChannelCommissionPolicyVersion)
			incentiveGroup.GET("/channel-policy-assignments", incentiveHandler.ListChannelCommissionPolicyAssignments)
			incentiveGroup.POST("/channel-policy-assignments", incentiveHandler.CreateChannelCommissionPolicyAssignment)
			incentiveGroup.GET("/channel-profit-snapshots", incentiveHandler.ListChannelProfitSnapshots)
			incentiveGroup.GET("/channel-adjustments", incentiveHandler.ListChannelCommissionAdjustments)
			incentiveGroup.POST("/channel-adjustments", incentiveHandler.CreateChannelCommissionAdjustment)
			incentiveGroup.POST("/channel-policy-resolution-preview", incentiveHandler.PreviewChannelPolicyResolution)
			incentiveGroup.GET("/channel-commissions", incentiveHandler.ListChannelCommissions)
			incentiveGroup.GET("/channel-clawbacks", incentiveHandler.ListChannelClawbacks)
			incentiveGroup.GET("/channel-settlement-batches", incentiveHandler.ListChannelSettlementBatches)
			incentiveGroup.GET("/channel-settlement-batches/:batchID", incentiveHandler.GetChannelSettlementBatch)
			incentiveGroup.POST("/channel-settlement-batches", incentiveHandler.GenerateChannelSettlementBatch)
			incentiveGroup.POST("/channel-settlement-batches/:batchID/confirm", incentiveHandler.ConfirmChannelSettlementBatch)
			incentiveGroup.POST("/channel-settlement-batches/:batchID/process", incentiveHandler.ProcessChannelSettlementBatch)
			incentiveGroup.POST("/channel-settlement-batches/:batchID/close", incentiveHandler.CloseChannelSettlementBatch)
			incentiveGroup.POST("/channel-settlement-batches/:batchID/cancel", incentiveHandler.CancelChannelSettlementBatch)
			incentiveGroup.GET("/channel-settlement-items", incentiveHandler.ListChannelSettlementItems)
			incentiveGroup.POST("/channel-events/charges", incentiveHandler.RecordChannelCharge)
			incentiveGroup.POST("/channel-events/refunds", incentiveHandler.RecordChannelRefund)
		}

		meteringGroup := v1.Group("/metering")
		meteringGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret))
		{
			meteringGroup.POST("/events", meteringHandler.IngestEvent)
			meteringGroup.GET("/usage/summary", meteringHandler.UsageSummary)
			meteringGroup.GET("/settlements", middleware.RequirePermission("platform.admin"), meteringHandler.ListSettlements)
			meteringGroup.GET("/settlements/:eventID", middleware.RequirePermission("platform.admin"), meteringHandler.GetSettlement)
			meteringGroup.POST("/settlements/:eventID/reverse", middleware.RequirePermission("platform.admin"), meteringHandler.ReverseSettlement)
			meteringGroup.GET("/discounts", middleware.RequirePermission("platform.admin"), meteringHandler.ListDiscounts)
		}

		runtimeGroup := v1.Group("/runtime")
		runtimeGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			runtimeGroup.GET("/jobs", runtimeHandler.ListRuntimeJobs)
			runtimeGroup.GET("/jobs/:runtimeJobID", runtimeHandler.GetRuntimeJob)
			runtimeGroup.GET("/charge-sessions", runtimeHandler.ListChargeSessions)
			runtimeGroup.GET("/charge-sessions/:chargeSessionID", runtimeHandler.GetChargeSession)
		}

		templateOpsGroup := v1.Group("/template-ops")
		templateOpsGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			templateOpsGroup.GET("/catalog", templateOpsHandler.ListCatalog)
			templateOpsGroup.GET("/catalog/:templateRef", templateOpsHandler.GetDetail)
			templateOpsGroup.GET("/catalog/:templateRef/assets", templateOpsHandler.ListTemplateAssets)
			templateOpsGroup.POST("/sync", templateOpsHandler.SyncCatalog)
			templateOpsGroup.POST("/catalog", templateOpsHandler.CreateCatalog)
			templateOpsGroup.PUT("/catalog/:templateRef", templateOpsHandler.UpdateCatalog)
			templateOpsGroup.PUT("/catalog/:templateRef/assets/:assetRole", templateOpsHandler.UpsertTemplateAsset)
			templateOpsGroup.DELETE("/catalog/:templateRef/assets/:assetRole", templateOpsHandler.UnbindTemplateAsset)
			templateOpsGroup.POST("/catalog/:templateRef/publish", templateOpsHandler.PublishCatalog)
			templateOpsGroup.POST("/import/csv/preview", templateOpsHandler.PreviewImportCSV)
			templateOpsGroup.POST("/import/csv", templateOpsHandler.ImportCSV)
			templateOpsGroup.POST("/import/assets/prepared", templateOpsHandler.ImportPreparedRealAssets)
			templateOpsGroup.POST("/import/assets/upload", templateOpsHandler.BatchUploadAssets)
			templateOpsGroup.GET("/export/csv", templateOpsHandler.ExportCSV)
			templateOpsGroup.GET("/export/csv-template", templateOpsHandler.ExportCSVTemplate)
			templateOpsGroup.GET("/export/csv-real-sample", templateOpsHandler.ExportPreparedRealImportCSV)
		}

		auditGroup := v1.Group("/audit")
		auditGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			auditGroup.GET("/logs", auditHandler.ListLogs)
			auditGroup.GET("/logs/:auditID", auditHandler.GetLog)
		}

		opsGroup := v1.Group("/ops")
		opsGroup.Use(middleware.JWTAuth(identityService, cfg.Security.JWTSecret), middleware.RequirePermission("platform.admin"))
		{
			opsGroup.GET("/organizations", orgHandler.ListAll)
			opsGroup.POST("/organizations", orgHandler.Create)
			opsGroup.PUT("/organizations/:orgID", orgHandler.Update)
			opsGroup.DELETE("/organizations/:orgID", orgHandler.Delete)
			opsGroup.GET("/organizations/:orgID/members", orgHandler.ListMembers)
			opsGroup.POST("/organizations/:orgID/members", orgHandler.CreateMember)
			opsGroup.PUT("/organizations/:orgID/members/:userID", orgHandler.UpdateMember)
			opsGroup.DELETE("/organizations/:orgID/members/:userID", orgHandler.DeleteMember)
			opsGroup.GET("/users", identityHandler.ListUsers)
			opsGroup.POST("/users", identityHandler.CreateUser)
			opsGroup.PUT("/users/:userID", identityHandler.UpdateUser)
			opsGroup.DELETE("/users/:userID", identityHandler.DeleteUser)
		}
	}

	internal := r.Group("/internal/v1")
	internal.Use(middleware.RequireInternalService(cfg.Security.InternalServiceSecret))
	{
		internal.POST("/storage/assets", assetStorageHandler.UploadAsset)
		internal.POST("/storage/assets/register", assetStorageHandler.RegisterAsset)
		internal.POST("/storage/assets/import-local", assetStorageHandler.ImportLocalAsset)
		internal.POST("/storage/assets/resolve", assetStorageHandler.ResolveAssets)
		internal.GET("/storage/assets/metadata", assetStorageHandler.GetAssetMetadata)
		internal.GET("/storage/assets/content", assetStorageHandler.GetAssetContent)
		internal.GET("/catalog/offerings", catalogHandler.Offerings)
		internal.GET("/template-ops/catalog", templateOpsHandler.ListCatalog)
		internal.GET("/template-ops/catalog/:templateRef", templateOpsHandler.GetDetail)
		internal.GET("/users/:userID/profile", identityHandler.InternalProfile)
		internal.PUT("/users/:userID/profile", identityHandler.InternalUpdateProfile)
		internal.PUT("/orgs/:orgID/profile", orgHandler.InternalUpdateProfile)
		internal.GET("/access/users/:userID/orgs/:orgID", accessHandler.InternalMembershipAccess)
		internal.POST("/controls/quota/grants", controlHandler.GrantQuota)
		internal.GET("/controls/quota/balance", controlHandler.QuotaBalance)
		internal.GET("/controls/quota/policies", controlHandler.ListQuotaGrantPolicies)
		internal.POST("/controls/credits/grants", controlHandler.GrantCredits)
		internal.GET("/controls/credits/balance", controlHandler.CreditsBalance)
		internal.POST("/controls/package-activations", controlHandler.ActivatePackage)
		internal.GET("/controls/capability/policies", controlHandler.ListPackageCapabilityPolicies)
		internal.POST("/controls/capability/grants", controlHandler.GrantCapability)
		internal.GET("/controls/capability/resolve", controlHandler.ResolveCapability)
		internal.POST("/controls/reservations", controlHandler.Reserve)
		internal.POST("/controls/reservations/:reservationID/commit", controlHandler.CommitReservation)
		internal.POST("/controls/reservations/:reservationID/release", controlHandler.ReleaseReservation)
		internal.POST("/metering/events", meteringHandler.IngestEvent)
		internal.POST("/metering/finalizations", meteringHandler.Finalize)
		internal.GET("/metering/settlements", meteringHandler.ListSettlements)
		internal.GET("/metering/settlements/:eventID", meteringHandler.GetSettlement)
		internal.POST("/metering/settlements/:eventID/reverse", meteringHandler.ReverseSettlement)
		internal.GET("/metering/discounts", meteringHandler.ListDiscounts)
		internal.POST("/runtime/providers", runtimeHandler.CreateProviderDefinition)
		internal.GET("/runtime/providers", runtimeHandler.ListProviderDefinitions)
		internal.GET("/runtime/capabilities", runtimeHandler.ListRuntimeCapabilities)
		internal.POST("/runtime/jobs", runtimeHandler.CreateRuntimeJob)
		internal.GET("/runtime/jobs/:runtimeJobID", runtimeHandler.GetRuntimeJob)
		internal.PUT("/runtime/jobs/:runtimeJobID", runtimeHandler.UpdateRuntimeJob)
		internal.POST("/runtime/jobs/:runtimeJobID/cancel", runtimeHandler.CancelRuntimeJob)
		internal.POST("/runtime/jobs/:runtimeJobID/attempts", runtimeHandler.RecordRuntimeAttempt)
		internal.POST("/runtime/charge-sessions", runtimeHandler.CreateChargeSession)
		internal.GET("/runtime/charge-sessions/:chargeSessionID", runtimeHandler.GetChargeSession)
		internal.PUT("/runtime/charge-sessions/:chargeSessionID", runtimeHandler.UpdateChargeSession)
		internal.POST("/commercial/route/resolve", commercialHandler.ResolveRoute)
		internal.GET("/wallet/assets", walletHandler.ListAssetDefinitions)
		internal.POST("/wallet/assets", walletHandler.CreateAssetDefinition)
		internal.GET("/wallet/allowance-policies", walletHandler.ListAllowancePolicies)
		internal.POST("/wallet/allowance-policies", walletHandler.CreateAllowancePolicy)
		internal.GET("/wallet/accounts", walletHandler.ListAccounts)
		internal.GET("/wallet/summary", walletHandler.GetSummary)
		internal.GET("/wallet/buckets", walletHandler.ListBuckets)
		internal.GET("/wallet/ledger", walletHandler.ListLedger)
		internal.POST("/wallet/ledger", walletHandler.PostLedger)
		internal.POST("/wallet/cycle-allowances", walletHandler.GrantCycleAllowance)
		internal.POST("/wallet/expire", walletHandler.ExpireBuckets)
		internal.POST("/wallet/lifecycle/run", walletHandler.RunLifecycle)
		internal.POST("/incentives/rewards", incentiveHandler.CreateReward)
		internal.GET("/incentives/rewards", incentiveHandler.ListRewards)
		internal.GET("/incentives/commissions", incentiveHandler.ListCommissions)
		internal.POST("/incentives/commissions/redeem", incentiveHandler.RedeemCommissions)
		internal.GET("/incentives/referral-programs", incentiveHandler.ListReferralPrograms)
		internal.POST("/incentives/referral-programs", incentiveHandler.CreateReferralProgram)
		internal.GET("/incentives/referral-codes", incentiveHandler.ListReferralCodes)
		internal.GET("/incentives/referral-codes/:code/resolve", incentiveHandler.ResolveReferralCode)
		internal.POST("/incentives/referral-codes", incentiveHandler.CreateReferralCode)
		internal.GET("/incentives/referral-conversions", incentiveHandler.ListReferralConversions)
		internal.POST("/incentives/referral-conversions", incentiveHandler.CreateReferralConversion)
		internal.GET("/incentives/channel-partners", incentiveHandler.ListChannelPartners)
		internal.POST("/incentives/channel-partners", incentiveHandler.CreateChannelPartner)
		internal.GET("/incentives/channel-programs", incentiveHandler.ListChannelPrograms)
		internal.POST("/incentives/channel-programs", incentiveHandler.CreateChannelProgram)
		internal.GET("/incentives/channel-bindings", incentiveHandler.ListChannelBindings)
		internal.POST("/incentives/channel-bindings", incentiveHandler.CreateChannelBinding)
		internal.GET("/incentives/channel-policies", incentiveHandler.ListChannelCommissionPolicies)
		internal.POST("/incentives/channel-policies", incentiveHandler.CreateChannelCommissionPolicy)
		internal.GET("/incentives/channel-policy-versions", incentiveHandler.ListChannelCommissionPolicyVersions)
		internal.POST("/incentives/channel-policy-versions", incentiveHandler.CreateChannelCommissionPolicyVersion)
		internal.GET("/incentives/channel-policy-assignments", incentiveHandler.ListChannelCommissionPolicyAssignments)
		internal.POST("/incentives/channel-policy-assignments", incentiveHandler.CreateChannelCommissionPolicyAssignment)
		internal.GET("/incentives/channel-profit-snapshots", incentiveHandler.ListChannelProfitSnapshots)
		internal.GET("/incentives/channel-adjustments", incentiveHandler.ListChannelCommissionAdjustments)
		internal.POST("/incentives/channel-adjustments", incentiveHandler.CreateChannelCommissionAdjustment)
		internal.POST("/incentives/channel-policy-resolution-preview", incentiveHandler.PreviewChannelPolicyResolution)
		internal.GET("/incentives/channel-commissions", incentiveHandler.ListChannelCommissions)
		internal.GET("/incentives/channel-clawbacks", incentiveHandler.ListChannelClawbacks)
		internal.GET("/incentives/channel-settlement-batches", incentiveHandler.ListChannelSettlementBatches)
		internal.GET("/incentives/channel-settlement-batches/:batchID", incentiveHandler.GetChannelSettlementBatch)
		internal.POST("/incentives/channel-settlement-batches", incentiveHandler.GenerateChannelSettlementBatch)
		internal.POST("/incentives/channel-settlement-batches/:batchID/confirm", incentiveHandler.ConfirmChannelSettlementBatch)
		internal.POST("/incentives/channel-settlement-batches/:batchID/process", incentiveHandler.ProcessChannelSettlementBatch)
		internal.POST("/incentives/channel-settlement-batches/:batchID/close", incentiveHandler.CloseChannelSettlementBatch)
		internal.POST("/incentives/channel-settlement-batches/:batchID/cancel", incentiveHandler.CancelChannelSettlementBatch)
		internal.GET("/incentives/channel-settlement-items", incentiveHandler.ListChannelSettlementItems)
		internal.POST("/incentives/channel-events/charges", incentiveHandler.RecordChannelCharge)
		internal.POST("/incentives/channel-events/refunds", incentiveHandler.RecordChannelRefund)
	}
	return r
}
