package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
)

func TestBuildProductCallbackClientMenuAndEcommerce(t *testing.T) {
	type requestCapture struct {
		Path   string
		Header string
		Body   map[string]any
	}
	var captures []requestCapture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		captures = append(captures, requestCapture{
			Path:   r.URL.Path,
			Header: r.Header.Get(platformconst.HeaderInternalServiceSecret),
			Body:   body,
		})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	menuClient := buildProductCallbackClient(&models.RuntimeProductEndpoint{
		ProductCode:  "menu",
		CallbackKind: "menu_internal",
		BaseURL:      server.URL,
		Secret:       "menu-secret",
	})
	ecommerceClient := buildProductCallbackClient(&models.RuntimeProductEndpoint{
		ProductCode:  "ecommerce",
		CallbackKind: "ecommerce_internal",
		BaseURL:      server.URL,
		Secret:       "ecom-secret",
	})
	if menuClient == nil || ecommerceClient == nil {
		t.Fatalf("expected callback clients to be built")
	}
	if err := menuClient.UpdateJobRuntime(context.Background(), "job1", ProductUpdateRuntimeInput{Status: "processing"}); err != nil {
		t.Fatalf("menu UpdateJobRuntime: %v", err)
	}
	if err := ecommerceClient.RecordJobResults(context.Background(), "job2", ProductRecordResultsInput{Status: "completed"}); err != nil {
		t.Fatalf("ecommerce RecordJobResults: %v", err)
	}
	if len(captures) != 2 {
		t.Fatalf("unexpected capture count: %d", len(captures))
	}
	if captures[0].Path != "/internal/v1/menu/studio/jobs/job1/runtime" || captures[0].Header != "menu-secret" {
		t.Fatalf("unexpected menu callback capture: %+v", captures[0])
	}
	if captures[1].Path != "/internal/v1/ecommerce/jobs/job2/results" || captures[1].Header != "ecom-secret" {
		t.Fatalf("unexpected ecommerce callback capture: %+v", captures[1])
	}
}

func TestProductHTTPCallbackClientReturnsErrorOnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	client := newProductHTTPCallbackClient(
		server.URL,
		"secret",
		"test callback",
		func(sourceID string) string { return "/runtime/" + sourceID },
		func(sourceID string) string { return "/results/" + sourceID },
	)
	err := client.UpdateJobRuntime(context.Background(), "job-x", ProductUpdateRuntimeInput{Status: "processing"})
	if err == nil {
		t.Fatalf("expected callback error")
	}
}
