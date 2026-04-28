package response

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-service/pkg/platformconst"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func TestResponseHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")

	JSONSuccess(c, gin.H{"ok": true})
	if w.Code != http.StatusOK {
		t.Fatalf("JSONSuccess status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")
	JSONError(c, CodeForbidden, "forbidden")
	if w.Code != GetHTTPStatusCode(CodeForbidden) {
		t.Fatalf("JSONError status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")
	JSONPaginated(c, []string{"a"}, 1, 10, 1)
	if w.Code != http.StatusOK {
		t.Fatalf("JSONPaginated status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")
	JSONSuccessWithStatus(c, http.StatusCreated, gin.H{"created": true})
	if w.Code != http.StatusCreated {
		t.Fatalf("JSONSuccessWithStatus status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")
	JSONErrorSemantic(c, CodeConflict, "conflict", "E_CONFLICT", "retry later")
	if w.Code != http.StatusConflict {
		t.Fatalf("JSONErrorSemantic status=%d body=%s", w.Code, w.Body.String())
	}
	if code, errorCode, errorHint, errorMessage := ResponseMeta(c); code != int(CodeConflict) || errorCode != "E_CONFLICT" || errorHint != "retry later" || errorMessage != "conflict" {
		t.Fatalf("unexpected response meta: code=%d errorCode=%s errorHint=%s errorMessage=%s", code, errorCode, errorHint, errorMessage)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")
	JSONErrorWithFields(c, CodeInvalidParameter, "invalid", []FieldError{{Field: "name", Message: "required"}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("JSONErrorWithFields status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")
	JSONErrorWithStatus(c, CodeForbidden, "forbidden", http.StatusAccepted)
	if w.Code != http.StatusAccepted {
		t.Fatalf("JSONErrorWithStatus status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")
	JSONErrorWithStatusSemantic(c, CodeForbidden, "forbidden", "E_FORBIDDEN", "no access", http.StatusAccepted)
	if w.Code != http.StatusAccepted {
		t.Fatalf("JSONErrorWithStatusSemantic status=%d body=%s", w.Code, w.Body.String())
	}
	ObserveError(c, assertErr{})
	if len(c.Errors) != 1 {
		t.Fatalf("expected observed error to be attached to gin context")
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = req
	c.Set(platformconst.CtxRequestID, "req-1")
	WriteObservedSemanticError(c, assertErr{}, CodeConflict, "conflict", "E_CONFLICT_HELPER", "retry helper")
	if w.Code != http.StatusConflict {
		t.Fatalf("WriteObservedSemanticError status=%d body=%s", w.Code, w.Body.String())
	}
	if len(c.Errors) != 1 {
		t.Fatalf("expected helper to attach observed error")
	}
}

func TestResponseBindAndHelpers(t *testing.T) {
	validate := validator.New()
	type payload struct {
		Name string `validate:"required"`
		Kind string `validate:"oneof=a b"`
	}
	err := validate.Struct(payload{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set(platformconst.CtxRequestID, "req-2")
	JSONBindError(c, err, "invalid payload")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("JSONBindError status=%d body=%s", w.Code, w.Body.String())
	}
	if NewSemanticErrorResponse(CodeConflict, "conflict", "E_CONFLICT", "retry").ErrorCode != "E_CONFLICT" {
		t.Fatalf("expected semantic error code")
	}
	if len(NewErrorResponseWithFields(CodeInvalidParameter, "bad", []FieldError{{Field: "name"}}).Errors) != 1 {
		t.Fatalf("expected field errors")
	}
	if GetHTTPStatusCode(ResponseCode(9999)) != http.StatusInternalServerError {
		t.Fatalf("expected 500 fallback for unknown server code")
	}
	if validationMessage(err.(validator.ValidationErrors)[0]) == "" {
		t.Fatalf("expected validation message")
	}
	var fieldErrs validator.ValidationErrors
	if !errorAs(err, &fieldErrs) || len(fieldErrs) == 0 {
		t.Fatalf("expected errorAs validator success")
	}
	if errorAs(assertErr{}, &fieldErrs) {
		t.Fatalf("expected errorAs false for custom error")
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "assert" }
