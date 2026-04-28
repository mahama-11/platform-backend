package assetstorage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

type Service struct {
	repo   *repository.RuntimeRepository
	client *http.Client
}

type UploadAssetInput struct {
	ProductCode string `json:"product_code" binding:"required"`
	Category    string `json:"category" binding:"required"`
	FileName    string `json:"file_name"`
	MimeType    string `json:"mime_type"`
	Payload     string `json:"payload" binding:"required"`
}

type StoredAsset struct {
	StorageKey string `json:"storage_key"`
	MimeType   string `json:"mime_type"`
	FileSize   int64  `json:"file_size"`
}

type RegisterAssetInput struct {
	ProductCode string         `json:"product_code" binding:"required"`
	Category    string         `json:"category" binding:"required"`
	SourceType  string         `json:"source_type" binding:"required"`
	SourceRef   string         `json:"source_ref" binding:"required"`
	StorageKey  string         `json:"storage_key" binding:"required"`
	FileName    string         `json:"file_name"`
	MimeType    string         `json:"mime_type"`
	FileSize    int64          `json:"file_size"`
	Checksum    string         `json:"checksum"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
	Status      string         `json:"status"`
}

type ImportLocalAssetInput struct {
	ProductCode     string         `json:"product_code" binding:"required"`
	Category        string         `json:"category" binding:"required"`
	SourceType      string         `json:"source_type" binding:"required"`
	SourceRef       string         `json:"source_ref" binding:"required"`
	SourcePath      string         `json:"source_path" binding:"required"`
	StorageFileName string         `json:"storage_file_name"`
	MimeType        string         `json:"mime_type"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	Tags            []string       `json:"tags"`
	Metadata        map[string]any `json:"metadata"`
	Status          string         `json:"status"`
}

type AssetRecord struct {
	ID          string         `json:"id"`
	ProductCode string         `json:"product_code"`
	Category    string         `json:"category"`
	SourceType  string         `json:"source_type"`
	SourceRef   string         `json:"source_ref"`
	StorageKey  string         `json:"storage_key"`
	FileName    string         `json:"file_name"`
	MimeType    string         `json:"mime_type"`
	FileSize    int64          `json:"file_size"`
	Checksum    string         `json:"checksum"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
	Status      string         `json:"status"`
	ImportedAt  *time.Time     `json:"imported_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type ResolveAssetInput struct {
	ProductCode string `json:"product_code"`
	Category    string `json:"category"`
	SourceType  string `json:"source_type"`
	SourceRef   string `json:"source_ref"`
	StorageKey  string `json:"storage_key"`
}

type ResolveAssetsInput struct {
	Items []ResolveAssetInput `json:"items"`
}

var ErrAssetConflict = errors.New("storage asset conflict")
var ErrInvalidAssetPayload = errors.New("invalid storage asset payload")
var ErrStorageBindingNotFound = errors.New("storage binding not found")
var ErrUnsupportedStorageProvider = errors.New("unsupported storage provider")

func NewService(repo *repository.RuntimeRepository) *Service {
	return &Service{
		repo:   repo,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *Service) UploadAsset(_ context.Context, input UploadAssetInput) (*StoredAsset, error) {
	data, mimeType, err := decodePayload(input.Payload, input.MimeType)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidAssetPayload, err)
	}
	item, err := s.storeBytes(input.ProductCode, input.Category, input.FileName, mimeType, data)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: product=%s category=%s", ErrStorageBindingNotFound, strings.TrimSpace(input.ProductCode), strings.TrimSpace(input.Category))
		}
		return nil, err
	}
	return item, nil
}

func (s *Service) RegisterAsset(_ context.Context, input RegisterAssetInput) (*AssetRecord, error) {
	if strings.TrimSpace(input.StorageKey) == "" {
		return nil, fmt.Errorf("storage_key is required")
	}
	return s.upsertAssetRecord(input)
}

func (s *Service) ImportLocalAsset(_ context.Context, input ImportLocalAssetInput) (*AssetRecord, error) {
	sourcePath := filepath.Clean(strings.TrimSpace(input.SourcePath))
	if sourcePath == "" {
		return nil, fmt.Errorf("source_path is required")
	}
	body, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}
	mimeType := normalizeImageMimeType(input.MimeType)
	if mimeType == "" {
		mimeType = normalizeImageMimeType(http.DetectContentType(body))
	}
	checksum := checksumForBytes(body)
	existing, err := s.repo.FindStorageAssetBySource(
		strings.TrimSpace(input.ProductCode),
		strings.TrimSpace(input.Category),
		strings.TrimSpace(input.SourceType),
		strings.TrimSpace(input.SourceRef),
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	storageKey := ""
	fileSize := int64(len(body))
	fileName := filepath.Base(strings.TrimSpace(input.StorageFileName))
	if existing != nil {
		storageKey = strings.TrimSpace(existing.StorageKey)
		fileName = existing.FileName
		if existing.Checksum != checksum {
			stored, err := s.storeBytesToStorageKey(storageKey, mimeType, body)
			if err != nil {
				return nil, err
			}
			fileSize = stored.FileSize
			mimeType = stored.MimeType
		} else {
			mimeType = normalizeImageMimeType(existing.MimeType)
			if mimeType == "" {
				mimeType = normalizeImageMimeType(http.DetectContentType(body))
			}
			fileSize = existing.FileSize
		}
	} else {
		stored, err := s.storeBytes(input.ProductCode, input.Category, input.StorageFileName, mimeType, body)
		if err != nil {
			return nil, err
		}
		storageKey = stored.StorageKey
		mimeType = stored.MimeType
		fileSize = stored.FileSize
	}
	return s.upsertAssetRecord(RegisterAssetInput{
		ProductCode: input.ProductCode,
		Category:    input.Category,
		SourceType:  input.SourceType,
		SourceRef:   input.SourceRef,
		StorageKey:  storageKey,
		FileName:    fileName,
		MimeType:    mimeType,
		FileSize:    fileSize,
		Checksum:    checksum,
		Title:       input.Title,
		Description: input.Description,
		Tags:        input.Tags,
		Metadata:    mergeMetadata(input.Metadata, map[string]any{"source_path": sourcePath}),
		Status:      input.Status,
	})
}

func (s *Service) FindAssetMetadataByStorageKey(_ context.Context, storageKey string) (*AssetRecord, error) {
	item, err := s.repo.FindStorageAssetByStorageKey(strings.TrimSpace(storageKey))
	if err != nil {
		return nil, err
	}
	return storageAssetToRecord(item), nil
}

func (s *Service) FindAssetMetadataBySource(_ context.Context, productCode, category, sourceType, sourceRef string) (*AssetRecord, error) {
	item, err := s.repo.FindStorageAssetBySource(
		strings.TrimSpace(productCode),
		strings.TrimSpace(category),
		strings.TrimSpace(sourceType),
		strings.TrimSpace(sourceRef),
	)
	if err != nil {
		return nil, err
	}
	return storageAssetToRecord(item), nil
}

func (s *Service) ResolveAssets(ctx context.Context, input ResolveAssetsInput) ([]AssetRecord, error) {
	results := make([]AssetRecord, 0, len(input.Items))
	for _, item := range input.Items {
		var record *AssetRecord
		var err error
		if strings.TrimSpace(item.StorageKey) != "" {
			record, err = s.FindAssetMetadataByStorageKey(ctx, item.StorageKey)
		} else {
			record, err = s.FindAssetMetadataBySource(ctx, item.ProductCode, item.Category, item.SourceType, item.SourceRef)
		}
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		results = append(results, *record)
	}
	return results, nil
}

func (s *Service) ResolveLocalPath(storageKey string) (string, error) {
	productCode, category, err := parseStorageKey(storageKey)
	if err != nil {
		return "", err
	}
	binding, err := s.repo.FindPreferredStorageBinding(productCode, category)
	if err != nil {
		return "", err
	}
	if binding.ProviderCode != "local" {
		return "", fmt.Errorf("unsupported storage provider %s", binding.ProviderCode)
	}
	return filepath.Join(binding.LocalBaseDir, filepath.FromSlash(storageKey)), nil
}

func (s *Service) DataURLFromStorageKey(storageKey, mimeType string) (string, error) {
	absPath, err := s.ResolveLocalPath(storageKey)
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return "", fmt.Errorf("stored asset is empty")
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(body)
	}
	mimeType = normalizeImageMimeType(mimeType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func (s *Service) ImportRemoteAsset(ctx context.Context, productCode, category, fileName, mimeType, sourceURL string) (*StoredAsset, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("download remote asset failed: status=%d", resp.StatusCode)
	}
	if mimeType == "" {
		mimeType = resp.Header.Get("Content-Type")
	}
	if fileName == "" {
		if parsed, parseErr := url.Parse(sourceURL); parseErr == nil {
			fileName = path.Base(parsed.Path)
		}
	}
	limited := io.LimitReader(resp.Body, 20<<20)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	return s.storeBytes(productCode, category, fileName, mimeType, data)
}

func decodePayload(payload, fallbackMimeType string) ([]byte, string, error) {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return nil, "", fmt.Errorf("asset payload is required")
	}
	if strings.HasPrefix(trimmed, "data:") {
		header, raw, ok := strings.Cut(trimmed, ",")
		if !ok {
			return nil, "", fmt.Errorf("invalid data url payload")
		}
		mimeType := fallbackMimeType
		if meta := strings.TrimPrefix(header, "data:"); meta != "" {
			parts := strings.Split(meta, ";")
			if len(parts) > 0 && parts[0] != "" {
				mimeType = parts[0]
			}
		}
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, "", err
		}
		return data, normalizeImageMimeType(mimeType), nil
	}
	data, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, "", err
	}
	mimeType := normalizeImageMimeType(fallbackMimeType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	return data, mimeType, nil
}

func (s *Service) storeBytes(productCode, category, fileName, mimeType string, data []byte) (*StoredAsset, error) {
	binding, err := s.repo.FindPreferredStorageBinding(productCode, category)
	if err != nil {
		return nil, err
	}
	if binding.ProviderCode != "local" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStorageProvider, binding.ProviderCode)
	}
	storageKey := buildStorageKey(productCode, category, fileName, mimeType)
	return s.storeBytesToStorageKey(storageKey, mimeType, data)
}

func (s *Service) storeBytesToStorageKey(storageKey, mimeType string, data []byte) (*StoredAsset, error) {
	productCode, category, err := parseStorageKey(storageKey)
	if err != nil {
		return nil, err
	}
	binding, err := s.repo.FindPreferredStorageBinding(productCode, category)
	if err != nil {
		return nil, err
	}
	if binding.ProviderCode != "local" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStorageProvider, binding.ProviderCode)
	}
	absPath := filepath.Join(binding.LocalBaseDir, filepath.FromSlash(storageKey))
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return nil, err
	}
	return &StoredAsset{
		StorageKey: storageKey,
		MimeType:   normalizeImageMimeType(mimeType),
		FileSize:   int64(len(data)),
	}, nil
}

func buildStorageKey(productCode, category, fileName, mimeType string) string {
	ext := extensionForMimeType(mimeType)
	baseName := sanitizeFileName(fileName)
	if baseName == "" {
		baseName = fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	} else if filepath.Ext(baseName) == "" {
		baseName += ext
	}
	return filepath.ToSlash(filepath.Join(productCode, category, baseName))
}

func sanitizeFileName(value string) string {
	base := filepath.Base(strings.TrimSpace(value))
	if base == "." || base == "/" {
		return ""
	}
	return strings.ReplaceAll(base, "..", "")
}

func extensionForMimeType(mimeType string) string {
	switch normalizeImageMimeType(mimeType) {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func normalizeImageMimeType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return "image/jpeg"
	case "image/webp":
		return "image/webp"
	case "image/png":
		return "image/png"
	default:
		return strings.TrimSpace(mimeType)
	}
}

func parseStorageKey(storageKey string) (string, string, error) {
	trimmed := strings.Trim(strings.TrimSpace(storageKey), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("invalid storage key")
	}
	return parts[0], parts[1], nil
}

func (s *Service) upsertAssetRecord(input RegisterAssetInput) (*AssetRecord, error) {
	now := time.Now()
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "active"
	}
	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		fileName = filepath.Base(strings.TrimSpace(input.StorageKey))
	}
	metadataJSON := mustJSON(input.Metadata, "{}")
	tagsJSON := mustJSON(input.Tags, "[]")
	itemBySource, err := s.repo.FindStorageAssetBySource(input.ProductCode, input.Category, input.SourceType, input.SourceRef)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	itemByStorageKey, err := s.repo.FindStorageAssetByStorageKey(input.StorageKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if itemBySource != nil && itemByStorageKey != nil && itemBySource.ID != itemByStorageKey.ID {
		return nil, ErrAssetConflict
	}
	if itemBySource == nil && itemByStorageKey != nil {
		if itemByStorageKey.SourceType != strings.TrimSpace(input.SourceType) || itemByStorageKey.SourceRef != strings.TrimSpace(input.SourceRef) {
			return nil, ErrAssetConflict
		}
	}
	item := itemBySource
	if item == nil {
		item = itemByStorageKey
	}
	if item == nil {
		item = &models.StorageAsset{
			ID:        utils.GenerateID(),
			CreatedAt: now,
		}
	}
	item.ProductCode = strings.TrimSpace(input.ProductCode)
	item.Category = strings.TrimSpace(input.Category)
	item.SourceType = strings.TrimSpace(input.SourceType)
	item.SourceRef = strings.TrimSpace(input.SourceRef)
	item.StorageKey = strings.TrimSpace(input.StorageKey)
	item.FileName = fileName
	item.MimeType = normalizeImageMimeType(input.MimeType)
	item.FileSize = input.FileSize
	item.Checksum = strings.TrimSpace(input.Checksum)
	item.Title = strings.TrimSpace(input.Title)
	item.Description = strings.TrimSpace(input.Description)
	item.Tags = tagsJSON
	item.Metadata = metadataJSON
	item.Status = status
	item.ImportedAt = &now
	item.UpdatedAt = now
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if itemBySource == nil && itemByStorageKey == nil {
		if item.ID == "" {
			item.ID = utils.GenerateID()
		}
		if err := s.repo.CreateStorageAsset(item); err != nil {
			return nil, err
		}
		return storageAssetToRecord(item), nil
	}
	if err := s.repo.UpdateStorageAsset(item); err != nil {
		return nil, err
	}
	return storageAssetToRecord(item), nil
}

func storageAssetToRecord(item *models.StorageAsset) *AssetRecord {
	if item == nil {
		return nil
	}
	record := &AssetRecord{
		ID:          item.ID,
		ProductCode: item.ProductCode,
		Category:    item.Category,
		SourceType:  item.SourceType,
		SourceRef:   item.SourceRef,
		StorageKey:  item.StorageKey,
		FileName:    item.FileName,
		MimeType:    item.MimeType,
		FileSize:    item.FileSize,
		Checksum:    item.Checksum,
		Title:       item.Title,
		Description: item.Description,
		Status:      item.Status,
		ImportedAt:  item.ImportedAt,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(item.Tags)), &record.Tags)
	_ = json.Unmarshal([]byte(strings.TrimSpace(item.Metadata)), &record.Metadata)
	if record.Tags == nil {
		record.Tags = []string{}
	}
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	return record
}

func checksumForBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func mergeMetadata(base map[string]any, extras map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extras {
		out[key] = value
	}
	return out
}

func mustJSON(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	body, err := json.Marshal(value)
	if err != nil {
		return fallback
	}
	return string(body)
}
