package router

import (
	"testing"

	"platform-service/internal/config"
	access "platform-service/internal/modules/access"
	assetstorage "platform-service/internal/modules/assetstorage"
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

func TestNewRegistersCoreRoutes(t *testing.T) {
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
		nil,
	)
	if engine == nil {
		t.Fatalf("expected gin engine")
	}
	paths := map[string]bool{}
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	required := []string{
		"GET /healthz",
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"POST /api/v1/runtime/providers/:providerCode/callback",
		"GET /api/v1/template-ops/catalog",
		"POST /internal/v1/runtime/jobs",
		"POST /internal/v1/commercial/route/resolve",
		"GET /internal/v1/wallet/summary",
	}
	for _, key := range required {
		if !paths[key] {
			t.Fatalf("expected route %s to be registered", key)
		}
	}
}
