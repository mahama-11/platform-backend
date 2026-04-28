package metrics

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetricsHelpers(t *testing.T) {
	Configure("platform", "service")
	RecordHTTPRequest("GET", "/healthz", 200, 10*time.Millisecond)
	IncBusinessCounter("catalog_product_created_total")
	handler := Handler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("metrics handler status=%d body=%s", w.Code, w.Body.String())
	}
}
