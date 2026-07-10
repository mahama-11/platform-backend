package assetstorage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAssetStorageTestService(t *testing.T) (*Service, *repository.RuntimeRepository, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("assetstorage-%d.db", time.Now().UnixNano()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.StorageBinding{}, &models.StorageAsset{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewRuntimeRepository(db)
	baseDir := t.TempDir()
	if err := repo.CreateStorageBinding(&models.StorageBinding{
		ID:           utils.GenerateID(),
		ProductCode:  "ecommerce",
		Category:     "ecommerce-assets",
		ProviderCode: "local",
		LocalBaseDir: baseDir,
		Priority:     100,
		Enabled:      true,
		Metadata:     "{}",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}); err != nil {
		t.Fatalf("CreateStorageBinding: %v", err)
	}
	if err := repo.CreateStorageBinding(&models.StorageBinding{
		ID:           utils.GenerateID(),
		ProductCode:  "ecommerce",
		Category:     "*",
		ProviderCode: "local",
		LocalBaseDir: baseDir,
		Priority:     200,
		Enabled:      true,
		Metadata:     "{}",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}); err != nil {
		t.Fatalf("CreateStorageBinding wildcard: %v", err)
	}
	return NewService(repo), repo, baseDir
}

func TestUploadRegisterResolveAndDataURL(t *testing.T) {
	service, _, baseDir := newAssetStorageTestService(t)
	payload := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-binary-content"))
	stored, err := service.UploadAsset(context.Background(), UploadAssetInput{
		ProductCode: "ecommerce",
		Category:    "ecommerce-assets",
		FileName:    "hero-image",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("UploadAsset: %v", err)
	}
	if stored.StorageKey == "" || !strings.HasSuffix(stored.StorageKey, ".png") {
		t.Fatalf("unexpected stored asset: %+v", stored)
	}
	if _, err := os.Stat(filepath.Join(baseDir, filepath.FromSlash(stored.StorageKey))); err != nil {
		t.Fatalf("expected stored file: %v", err)
	}
	record, err := service.RegisterAsset(context.Background(), RegisterAssetInput{
		ProductCode: "ecommerce",
		Category:    "ecommerce-assets",
		SourceType:  "template_result",
		SourceRef:   "job-1",
		StorageKey:  stored.StorageKey,
		FileName:    "hero-image.png",
		MimeType:    "image/png",
		FileSize:    stored.FileSize,
		Metadata:    map[string]any{"scene": "hero"},
		Tags:        []string{"hero"},
	})
	if err != nil {
		t.Fatalf("RegisterAsset: %v", err)
	}
	if record.ID == "" || record.StorageKey != stored.StorageKey || record.Metadata["scene"] != "hero" {
		t.Fatalf("unexpected record: %+v", record)
	}
	records, err := service.ResolveAssets(context.Background(), ResolveAssetsInput{
		Items: []ResolveAssetInput{
			{StorageKey: stored.StorageKey},
			{ProductCode: "ecommerce", Category: "ecommerce-assets", SourceType: "template_result", SourceRef: "job-1"},
		},
	})
	if err != nil || len(records) != 2 {
		t.Fatalf("ResolveAssets: %+v err=%v", records, err)
	}
	dataURL, err := service.DataURLFromStorageKey(stored.StorageKey, "")
	if err != nil || !strings.HasPrefix(dataURL, "data:") || !strings.Contains(dataURL, ";base64,") {
		t.Fatalf("DataURLFromStorageKey: %s err=%v", dataURL, err)
	}
}

func TestImportLocalAssetUpdateAndConflict(t *testing.T) {
	service, _, _ := newAssetStorageTestService(t)
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(sourcePath, []byte("v1-content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	record, err := service.ImportLocalAsset(context.Background(), ImportLocalAssetInput{
		ProductCode:     "ecommerce",
		Category:        "ecommerce-assets",
		SourceType:      "template_example",
		SourceRef:       "example-1",
		SourcePath:      sourcePath,
		StorageFileName: "example.png",
		MimeType:        "image/png",
	})
	if err != nil {
		t.Fatalf("ImportLocalAsset: %v", err)
	}
	if record.StorageKey == "" {
		t.Fatalf("expected storage key")
	}
	if err := os.WriteFile(sourcePath, []byte("v2-content"), 0o644); err != nil {
		t.Fatalf("WriteFile update: %v", err)
	}
	updated, err := service.ImportLocalAsset(context.Background(), ImportLocalAssetInput{
		ProductCode:     "ecommerce",
		Category:        "ecommerce-assets",
		SourceType:      "template_example",
		SourceRef:       "example-1",
		SourcePath:      sourcePath,
		StorageFileName: "example.png",
		MimeType:        "image/png",
	})
	if err != nil {
		t.Fatalf("ImportLocalAsset update: %v", err)
	}
	if updated.StorageKey != record.StorageKey {
		t.Fatalf("expected same storage key on update")
	}
	otherStored, err := service.UploadAsset(context.Background(), UploadAssetInput{
		ProductCode: "ecommerce",
		Category:    "ecommerce-assets",
		FileName:    "other.png",
		Payload:     base64.StdEncoding.EncodeToString([]byte("other-binary-content")),
	})
	if err != nil {
		t.Fatalf("UploadAsset otherStored: %v", err)
	}
	if _, err := service.RegisterAsset(context.Background(), RegisterAssetInput{
		ProductCode: "ecommerce",
		Category:    "ecommerce-assets",
		SourceType:  "template_example",
		SourceRef:   "other-ref",
		StorageKey:  otherStored.StorageKey,
	}); err != nil {
		t.Fatalf("RegisterAsset other-ref: %v", err)
	}
	if _, err := service.RegisterAsset(context.Background(), RegisterAssetInput{
		ProductCode: "ecommerce",
		Category:    "ecommerce-assets",
		SourceType:  "template_example",
		SourceRef:   "example-1",
		StorageKey:  otherStored.StorageKey,
	}); !errors.Is(err, ErrAssetConflict) {
		t.Fatalf("expected ErrAssetConflict, got %v", err)
	}
}

func TestImportRemoteAssetAndHelperFunctions(t *testing.T) {
	service, _, _ := newAssetStorageTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("remote-image"))
	}))
	defer server.Close()

	stored, err := service.ImportRemoteAsset(context.Background(), "ecommerce", "ecommerce-assets", "", "", server.URL+"/image.jpg")
	if err != nil {
		t.Fatalf("ImportRemoteAsset: %v", err)
	}
	if stored.StorageKey == "" || stored.MimeType == "" {
		t.Fatalf("unexpected stored remote asset: %+v", stored)
	}
	if _, _, err := decodePayload("", ""); err == nil {
		t.Fatalf("expected empty payload error")
	}
	if buildStorageKey("ecommerce", "ecommerce-assets", "name", "image/png") == "" {
		t.Fatalf("expected buildStorageKey output")
	}
	if sanitizeFileName("../hello.png") != "hello.png" {
		t.Fatalf("unexpected sanitizeFileName result")
	}
	if extensionForMimeType("image/webp") != ".webp" {
		t.Fatalf("expected webp extension")
	}
	if normalizeImageMimeType("image/jpg") != "image/jpeg" {
		t.Fatalf("expected normalized jpeg mime")
	}
	if _, _, err := parseStorageKey("invalid"); err == nil {
		t.Fatalf("expected invalid storage key error")
	}
	if _, err := readAssetBytes(strings.NewReader("12345"), 4); err == nil {
		t.Fatalf("expected oversized asset error")
	}
	body := mustJSON(map[string]any{"a": 1}, "{}")
	var parsed map[string]any
	_ = json.Unmarshal([]byte(body), &parsed)
	if parsed["a"].(float64) != 1 {
		t.Fatalf("expected valid mustJSON output")
	}
}
