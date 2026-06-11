package router

import (
	"testing"

	"platform-service/internal/config"
	access "platform-service/internal/modules/access"
	assetstorage "platform-service/internal/modules/assetstorage"
	audit "platform-service/internal/modules/audit"
	catalog "platform-service/internal/modules/catalog"
	commercial "platform-service/internal/modules/commercial"
	control "platform-service/internal/modules/control"
	identity "platform-service/internal/modules/identity"
	incentive "platform-service/internal/modules/incentive"
	metering "platform-service/internal/modules/metering"
	organization "platform-service/internal/modules/organization"
	runtime "platform-service/internal/modules/runtime"
	templateops "platform-service/internal/modules/templateops"
	wallet "platform-service/internal/modules/wallet"
)

func TestPlatformSharedCapabilityContractRoutes(t *testing.T) {
	cfg := config.Config{}
	cfg.GinMode = "test"
	cfg.Security.JWTSecret = "jwt-secret"
	cfg.Security.InternalServiceSecret = "internal-secret"
	cfg.Monitoring.Tracing.ServiceName = "platform-service"

	engine := New(
		cfg,
		assetstorage.NewHandler(nil),
		identity.NewHandler(nil),
		organization.NewHandler(nil),
		access.NewHandler(nil),
		catalog.NewHandler(nil, nil, nil),
		commercial.NewHandler(nil, nil),
		control.NewHandler(nil, nil),
		wallet.NewHandler(nil, nil),
		incentive.NewHandler(nil, nil),
		metering.NewHandler(nil, nil),
		runtime.NewHandler(nil, nil),
		templateops.NewHandler(nil),
		audit.NewHandler(nil),
		nil,
	)
	paths := map[string]bool{}
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	required := []string{
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/auth/me",
		"GET /api/v1/audit/diagnostics/requests/:requestID",
		"POST /api/v1/runtime/providers/:providerCode/callback",
		"POST /internal/v1/runtime/providers",
		"GET /internal/v1/runtime/providers",
		"GET /internal/v1/runtime/capabilities",
		"POST /internal/v1/runtime/jobs",
		"GET /internal/v1/runtime/jobs/:runtimeJobID",
		"PUT /internal/v1/runtime/jobs/:runtimeJobID",
		"POST /internal/v1/runtime/jobs/:runtimeJobID/cancel",
		"POST /internal/v1/runtime/jobs/:runtimeJobID/attempts",
		"POST /internal/v1/controls/reservations",
		"POST /internal/v1/controls/reservations/:reservationID/commit",
		"POST /internal/v1/controls/reservations/:reservationID/release",
		"POST /internal/v1/controls/quota/grants",
		"GET /internal/v1/controls/quota/balance",
		"POST /internal/v1/storage/assets",
		"POST /internal/v1/storage/assets/register",
		"POST /internal/v1/storage/assets/import-local",
		"POST /internal/v1/storage/assets/resolve",
		"GET /internal/v1/storage/assets/metadata",
		"GET /internal/v1/storage/assets/content",
		"POST /internal/v1/runtime/charge-sessions",
		"GET /internal/v1/runtime/charge-sessions/:chargeSessionID",
		"PUT /internal/v1/runtime/charge-sessions/:chargeSessionID",
		"GET /internal/v1/wallet/summary",
		"POST /internal/v1/wallet/ledger",
		"POST /internal/v1/metering/events",
		"POST /internal/v1/metering/finalizations",
		"GET /internal/v1/metering/settlements",
	}
	for _, key := range required {
		if !paths[key] {
			t.Fatalf("expected Platform shared capability contract route %s to be registered", key)
		}
	}
}
