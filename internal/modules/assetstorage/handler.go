package assetstorage

import (
	"errors"
	"net/http"
	"strings"

	"platform-service/internal/telemetry"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) UploadAsset(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/asset-storage-handler", "asset_storage.upload")
	defer span.End()
	var req UploadAssetInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid asset upload request")
		return
	}
	item, err := h.service.UploadAsset(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, ErrInvalidAssetPayload) {
			response.WriteObservedStatusSemanticError(c, err, response.CodeInvalidParameter, "invalid asset upload payload", "STORAGE_ASSET_PAYLOAD_INVALID", "Send a valid data URL or base64-encoded image payload.", http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrStorageBindingNotFound) {
			response.WriteObservedStatusSemanticError(c, err, response.CodeServiceUnavailable, "storage binding is not configured", "STORAGE_BINDING_NOT_FOUND", "Configure a storage binding for this product and category, then retry.", http.StatusServiceUnavailable)
			return
		}
		if errors.Is(err, ErrUnsupportedStorageProvider) {
			response.WriteObservedStatusSemanticError(c, err, response.CodeServiceUnavailable, "storage provider is not supported", "STORAGE_PROVIDER_UNAVAILABLE", "Configure a supported storage provider for this binding, then retry.", http.StatusServiceUnavailable)
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to upload asset", "ASSET_STORAGE_UPLOAD_FAILED", "Check platform logs with request_id, product_code, category, and payload metadata to identify the storage failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) RegisterAsset(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/asset-storage-handler", "asset_storage.register")
	defer span.End()
	var req RegisterAssetInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid asset register request")
		return
	}
	item, err := h.service.RegisterAsset(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, ErrAssetConflict) {
			response.WriteObservedSemanticError(c, err, response.CodeConflict, "asset metadata conflicts with existing registration", "ASSET_STORAGE_REGISTER_CONFLICT", "Use the existing source_ref mapping or reconcile conflicting storage metadata before retrying.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to register asset metadata", "ASSET_STORAGE_REGISTER_FAILED", "Check platform logs with request_id, product_code, category, source_type, and source_ref.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ImportLocalAsset(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/asset-storage-handler", "asset_storage.import_local")
	defer span.End()
	var req ImportLocalAssetInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid asset local import request")
		return
	}
	item, err := h.service.ImportLocalAsset(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, ErrAssetConflict) {
			response.WriteObservedSemanticError(c, err, response.CodeConflict, "asset import conflicts with existing registration", "ASSET_STORAGE_IMPORT_CONFLICT", "Use a unique source_ref or reconcile the existing imported asset before retrying.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to import local asset", "ASSET_STORAGE_IMPORT_FAILED", "Check platform logs with request_id, source_path, product_code, and source_ref to identify the import failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) GetAssetMetadata(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/asset-storage-handler", "asset_storage.get_metadata")
	defer span.End()
	storageKey := strings.TrimSpace(c.Query("storage_key"))
	if storageKey != "" {
		item, err := h.service.FindAssetMetadataByStorageKey(c.Request.Context(), storageKey)
		if err != nil {
			span.RecordError(err)
			response.WriteObservedSemanticError(c, err, response.CodeNotFound, "asset metadata not found", "ASSET_STORAGE_METADATA_NOT_FOUND", "Verify the storage_key or source mapping before retrying.")
			return
		}
		response.JSONSuccess(c, item)
		return
	}
	productCode := strings.TrimSpace(c.Query("product_code"))
	category := strings.TrimSpace(c.Query("category"))
	sourceType := strings.TrimSpace(c.Query("source_type"))
	sourceRef := strings.TrimSpace(c.Query("source_ref"))
	if productCode == "" || category == "" || sourceType == "" || sourceRef == "" {
		response.JSONError(c, response.CodeMissingParameter, "storage_key or product_code/category/source_type/source_ref is required")
		return
	}
	item, err := h.service.FindAssetMetadataBySource(c.Request.Context(), productCode, category, sourceType, sourceRef)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "asset metadata not found", "ASSET_STORAGE_METADATA_NOT_FOUND", "Verify product_code, category, source_type, and source_ref before retrying.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) ResolveAssets(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/asset-storage-handler", "asset_storage.resolve")
	defer span.End()
	var req ResolveAssetsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid asset resolve request")
		return
	}
	items, err := h.service.ResolveAssets(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to resolve assets", "ASSET_STORAGE_RESOLVE_FAILED", "Check platform logs with request_id and the resolve payload to identify the failing asset lookup.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) GetAssetContent(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/asset-storage-handler", "asset_storage.get_content")
	defer span.End()
	storageKey := strings.TrimSpace(c.Query("storage_key"))
	if storageKey == "" {
		response.JSONError(c, response.CodeMissingParameter, "storage_key is required")
		return
	}
	absPath, err := h.service.ResolveLocalPath(storageKey)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "asset not found", "ASSET_STORAGE_CONTENT_NOT_FOUND", "Verify the storage_key and local storage binding before retrying.")
		return
	}
	c.File(absPath)
}
