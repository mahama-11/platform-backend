package assetstorage

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAssetStorageHandlerHappyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _ := newAssetStorageTestService(t)
	handler := NewHandler(service)
	payload := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("asset-bytes"))

	resp := performAssetJSON(t, handler.UploadAsset, http.MethodPost, "/assets/upload", UploadAssetInput{
		ProductCode: "ecommerce",
		Category:    "ecommerce-assets",
		FileName:    "hero",
		Payload:     payload,
	})
	var uploaded map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &uploaded)
	data := uploaded["data"].(map[string]any)
	storageKey := data["storage_key"].(string)

	performAssetJSON(t, handler.RegisterAsset, http.MethodPost, "/assets/register", RegisterAssetInput{
		ProductCode: "ecommerce",
		Category:    "ecommerce-assets",
		SourceType:  "template_result",
		SourceRef:   "job-1",
		StorageKey:  storageKey,
	})
	performAssetQuery(t, handler.GetAssetMetadata, "/assets/meta?storage_key="+storageKey)
	performAssetQuery(t, handler.GetAssetMetadata, "/assets/meta?product_code=ecommerce&category=ecommerce-assets&source_type=template_result&source_ref=job-1")
	performAssetJSON(t, handler.ResolveAssets, http.MethodPost, "/assets/resolve", ResolveAssetsInput{
		Items: []ResolveAssetInput{{StorageKey: storageKey}},
	})
	performAssetQuery(t, handler.GetAssetContent, "/assets/content?storage_key="+storageKey)

	localPath := filepath.Join(t.TempDir(), "source.png")
	if err := os.WriteFile(localPath, []byte("local-asset"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	performAssetJSON(t, handler.ImportLocalAsset, http.MethodPost, "/assets/import-local", ImportLocalAssetInput{
		ProductCode:     "ecommerce",
		Category:        "ecommerce-assets",
		SourceType:      "template_example",
		SourceRef:       "local-job",
		SourcePath:      localPath,
		StorageFileName: "local.png",
		MimeType:        "image/png",
	})
}

func TestAssetStorageHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _ := newAssetStorageTestService(t)
	handler := NewHandler(service)
	resp := performAssetRaw(t, handler.UploadAsset, http.MethodPost, "/assets/upload", []byte("{bad"))
	if resp.Code == http.StatusOK {
		t.Fatalf("expected bind error")
	}
	resp = performAssetQuery(t, handler.GetAssetMetadata, "/assets/meta")
	if resp.Code == http.StatusOK {
		t.Fatalf("expected missing parameter error")
	}
	resp = performAssetQuery(t, handler.GetAssetContent, "/assets/content?storage_key=missing/key.png")
	if resp.Code == http.StatusOK {
		t.Fatalf("expected missing content error")
	}
}

func TestAssetStorageHandlerUploadInvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _ := newAssetStorageTestService(t)
	handler := NewHandler(service)

	resp := performAssetJSON(t, handler.UploadAsset, http.MethodPost, "/assets/upload", UploadAssetInput{
		ProductCode: "ecommerce",
		Category:    "ecommerce-assets",
		FileName:    "bad.png",
		MimeType:    "image/png",
		Payload:     "blob:http://localhost/not-supported",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", resp.Code, resp.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if body["error_code"] != "STORAGE_ASSET_PAYLOAD_INVALID" {
		t.Fatalf("unexpected error_code: %+v", body)
	}
}

func performAssetJSON(t *testing.T, fn func(*gin.Context), method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performAssetRaw(t, fn, method, path, payload)
}

func performAssetQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performAssetRaw(t, fn, http.MethodGet, path, nil)
}

func performAssetRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	fn(c)
	if w.Code >= 500 {
		t.Fatalf("unexpected handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func TestAssetStorageHandlerBindErrorMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _ := newAssetStorageTestService(t)
	handler := NewHandler(service)
	cases := []struct {
		name string
		fn   func(*gin.Context)
		path string
	}{
		{"upload", handler.UploadAsset, "/assets/upload"},
		{"register", handler.RegisterAsset, "/assets/register"},
		{"import_local", handler.ImportLocalAsset, "/assets/import-local"},
		{"resolve", handler.ResolveAssets, "/assets/resolve"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performAssetRaw(t, tc.fn, http.MethodPost, tc.path, []byte("{bad"))
			if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
				t.Fatalf("expected bind error, got %d: %s", resp.Code, resp.Body.String())
			}
		})
	}
	missingMeta := performAssetQuery(t, handler.GetAssetMetadata, "/assets/meta?storage_key=missing")
	if missingMeta.Code == http.StatusOK {
		t.Fatalf("expected missing metadata error")
	}
}
