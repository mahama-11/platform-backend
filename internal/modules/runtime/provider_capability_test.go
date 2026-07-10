package runtime

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeVideoCapabilityProvider struct {
	*fakeProvider
	balanceCtx context.Context
	uploadKind string
	action     string
}

func (p *fakeVideoCapabilityProvider) Balance(ctx context.Context) (map[string]any, error) {
	p.balanceCtx = ctx
	return map[string]any{"credit_package": 100}, nil
}

func (p *fakeVideoCapabilityProvider) TTSVoices(context.Context) (map[string]any, error) {
	return map[string]any{"voices": []any{}}, nil
}

func (p *fakeVideoCapabilityProvider) UploadFile(_ context.Context, kind, _ string, _ io.Reader) (map[string]any, error) {
	p.uploadKind = kind
	return map[string]any{"media_id": 1}, nil
}

func (p *fakeVideoCapabilityProvider) UploadURL(context.Context, string) (map[string]any, error) {
	return map[string]any{"media_id": 2}, nil
}

func (p *fakeVideoCapabilityProvider) ProviderAction(_ context.Context, action string, _ map[string]any) (map[string]any, error) {
	p.action = action
	return map[string]any{"ok": true}, nil
}

func TestProviderCapabilityServiceAndHandlers(t *testing.T) {
	service, _, _ := newRuntimeServiceForTest(t)
	provider := &fakeVideoCapabilityProvider{fakeProvider: &fakeProvider{name: "pai_video"}}
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(provider)
	service.UseRuntime(nil, registry)
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request"), "req-1")
	if _, err := service.ProviderBalance(ctx, "pai_video"); err != nil {
		t.Fatalf("ProviderBalance: %v", err)
	}
	if provider.balanceCtx == nil || provider.balanceCtx.Value(contextKey("request")) != "req-1" {
		t.Fatalf("caller context was not propagated")
	}

	handler := NewHandler(service, nil)
	resp := performRuntimeParam(t, handler.ProviderBalance, http.MethodGet, "/runtime/providers/pai_video/balance", "providerCode", "pai_video", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("balance status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = performRuntimeParam(t, handler.ProviderTTSVoices, http.MethodGet, "/runtime/providers/pai_video/tts-voices", "providerCode", "pai_video", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("voices status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = performRuntimeParam(t, handler.ProviderUploadURL, http.MethodPost, "/runtime/providers/pai_video/media-upload-url", "providerCode", "pai_video", ProviderUploadURLInput{FileURL: "https://example.com/video.mp4"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("upload url status=%d body=%s", resp.Code, resp.Body.String())
	}
	actionBody := []byte(`{"payload":{}}`)
	actionRecorder := httptest.NewRecorder()
	actionContext, _ := gin.CreateTestContext(actionRecorder)
	actionContext.Request = httptest.NewRequest(http.MethodPost, "/runtime/providers/pai_video/actions/restyle_list", bytes.NewReader(actionBody))
	actionContext.Request.Header.Set("Content-Type", "application/json")
	actionContext.Params = gin.Params{{Key: "providerCode", Value: "pai_video"}, {Key: "action", Value: "restyle_list"}}
	handler.ProviderAction(actionContext)
	if actionRecorder.Code != http.StatusOK || provider.action != "restyle_list" {
		t.Fatalf("action status=%d body=%s action=%s", actionRecorder.Code, actionRecorder.Body.String(), provider.action)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "source.mp4")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = part.Write([]byte("video"))
	_ = writer.Close()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/runtime/providers/pai_video/media-upload", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Params = gin.Params{{Key: "providerCode", Value: "pai_video"}}
	handler.ProviderUploadMedia(c)
	if w.Code != http.StatusCreated || provider.uploadKind != "media" {
		t.Fatalf("media upload status=%d body=%s kind=%s", w.Code, w.Body.String(), provider.uploadKind)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPost, "/runtime/providers/pai_video/image-upload", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Params = gin.Params{{Key: "providerCode", Value: "pai_video"}}
	handler.ProviderUploadImage(c)
	if w.Code != http.StatusCreated || provider.uploadKind != "image" {
		t.Fatalf("image upload status=%d body=%s kind=%s", w.Code, w.Body.String(), provider.uploadKind)
	}
}

func TestProviderCapabilityHandlersRejectInvalidOrUnsupportedRequests(t *testing.T) {
	service, _, _ := newRuntimeServiceForTest(t)
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(&fakeProvider{name: "plain_provider"})
	service.UseRuntime(nil, registry)
	handler := NewHandler(service, nil)
	perform := func(handle gin.HandlerFunc, method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, path, nil)
		c.Params = gin.Params{{Key: "providerCode", Value: "plain_provider"}}
		handle(c)
		return w
	}

	for name, tc := range map[string]struct {
		method string
		path   string
		handle gin.HandlerFunc
	}{
		"balance": {http.MethodGet, "/runtime/providers/plain_provider/balance", handler.ProviderBalance},
		"voices":  {http.MethodGet, "/runtime/providers/plain_provider/tts-voices", handler.ProviderTTSVoices},
	} {
		t.Run(name, func(t *testing.T) {
			resp := perform(tc.handle, tc.method, tc.path)
			if resp.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}

	resp := performRuntimeParam(t, handler.ProviderUploadImage, http.MethodPost, "/runtime/providers/plain_provider/image-upload", "providerCode", "plain_provider", nil)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("missing upload status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = performRuntimeParam(t, handler.ProviderUploadURL, http.MethodPost, "/runtime/providers/plain_provider/media-upload-url", "providerCode", "plain_provider", map[string]any{})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid upload url status=%d body=%s", resp.Code, resp.Body.String())
	}
	if _, err := service.ProviderBalance(context.Background(), "missing"); err == nil {
		t.Fatalf("missing provider should fail")
	}

	actionRecorder := httptest.NewRecorder()
	actionContext, _ := gin.CreateTestContext(actionRecorder)
	actionContext.Request = httptest.NewRequest(http.MethodPost, "/runtime/providers/plain_provider/actions/restyle_list", bytes.NewBufferString("{"))
	actionContext.Request.Header.Set("Content-Type", "application/json")
	actionContext.Params = gin.Params{{Key: "providerCode", Value: "plain_provider"}, {Key: "action", Value: "restyle_list"}}
	handler.ProviderAction(actionContext)
	if actionRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid action status=%d body=%s", actionRecorder.Code, actionRecorder.Body.String())
	}
}
