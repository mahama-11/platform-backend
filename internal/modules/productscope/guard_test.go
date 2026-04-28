package productscope

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveRequiresScopedProductOrExplicitAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("product code", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/internal?product_code=ecommerce", nil)

		scope, ok := Resolve(c)
		if !ok {
			t.Fatalf("expected resolve success")
		}
		if scope.ProductCode != "ecommerce" || scope.IncludeAll {
			t.Fatalf("unexpected scope: %+v", scope)
		}
	})

	t.Run("include all", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/internal?include_all_products=true", nil)

		scope, ok := Resolve(c)
		if !ok {
			t.Fatalf("expected resolve success")
		}
		if scope.ProductCode != "" || !scope.IncludeAll {
			t.Fatalf("unexpected scope: %+v", scope)
		}
	})

	t.Run("missing scope", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/internal", nil)

		if _, ok := Resolve(c); ok {
			t.Fatalf("expected resolve failure")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("ambiguous scope", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/internal?product_code=ecommerce&include_all_products=true", nil)

		if _, ok := Resolve(c); ok {
			t.Fatalf("expected resolve failure")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})
}
