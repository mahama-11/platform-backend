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
	productbilling "platform-service/internal/modules/productbilling"
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
		productbilling.NewHandler(nil),
		templateops.NewHandler(nil),
		audit.NewHandler(nil),
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
		"GET /docs/internal-access",
		"GET /docs/error-codes",
		"GET /api/v1/docs/internal-access",
		"GET /api/v1/docs/error-codes",
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/auth/me",
		"POST /api/v1/runtime/providers/:providerCode/callback",
		"GET /api/v1/template-ops/catalog",
		"GET /api/v1/audit/logs",
		"GET /api/v1/audit/logs/:auditID",
		"GET /api/v1/audit/diagnostics/requests/:requestID",
		"POST /internal/v1/runtime/jobs",
		"GET /internal/v1/runtime/providers/:providerCode/balance",
		"GET /internal/v1/runtime/providers/:providerCode/tts-voices",
		"POST /internal/v1/runtime/providers/:providerCode/image-upload",
		"POST /internal/v1/runtime/providers/:providerCode/media-upload",
		"POST /internal/v1/runtime/providers/:providerCode/media-upload-url",
		"POST /internal/v1/runtime/providers/:providerCode/actions/:action",
		"POST /internal/v1/commercial/route/resolve",
		"GET /internal/v1/wallet/summary",
	}
	for _, key := range required {
		if !paths[key] {
			t.Fatalf("expected route %s to be registered", key)
		}
	}
}
