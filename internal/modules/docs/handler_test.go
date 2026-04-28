package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDocsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/internal/docs/access", nil)
	h.InternalAccessDoc(c)
	if w.Code != http.StatusOK {
		t.Fatalf("InternalAccessDoc status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/internal/docs/errors", nil)
	h.ErrorCodesDoc(c)
	if w.Code != http.StatusOK {
		t.Fatalf("ErrorCodesDoc status=%d body=%s", w.Code, w.Body.String())
	}
}
