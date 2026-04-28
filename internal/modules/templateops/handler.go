package templateops

import (
	"strconv"

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

func (h *Handler) ListCatalog(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.catalog.list")
	defer span.End()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	result, err := h.service.ListCatalog(c.Request.Context(), ListCatalogInput{
		ProductCode:   c.Query("product_code"),
		Query:         c.Query("query"),
		Locale:        c.Query("locale"),
		Limit:         limit,
		Offset:        offset,
		PublishedOnly: c.Query("published_only") == "true",
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load template catalog", "TEMPLATE_OPS_LIST_FAILED", "Check platform logs with request_id and template filters to identify the template aggregation failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) SyncCatalog(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.catalog.sync")
	defer span.End()
	result, err := h.service.SyncFromUpstream(c.Request.Context(), c.Query("product_code"), c.DefaultQuery("locale", "zh"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to sync template catalog", "TEMPLATE_OPS_SYNC_FAILED", "Check upstream business template APIs and platform logs, then retry sync.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) CreateCatalog(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.catalog.create")
	defer span.End()
	var req UpsertTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid template create request")
		return
	}
	result, err := h.service.CreateTemplate(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create template projection", "TEMPLATE_OPS_CREATE_FAILED", "Check template payload and platform logs to identify the creation failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, result)
}

func (h *Handler) UpdateCatalog(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.catalog.update")
	defer span.End()
	var req UpsertTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid template update request")
		return
	}
	result, err := h.service.UpdateTemplate(c.Request.Context(), c.Param("templateRef"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update template projection", "TEMPLATE_OPS_UPDATE_FAILED", "Check template_ref and payload, then inspect platform logs for the update failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) PublishCatalog(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.catalog.publish")
	defer span.End()
	result, err := h.service.PublishTemplate(c.Request.Context(), c.Param("templateRef"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to publish template projection", "TEMPLATE_OPS_PUBLISH_FAILED", "Check template_ref and platform logs to identify the publish failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ImportCSV(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.csv.import")
	defer span.End()
	var req CSVImportInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid csv import request")
		return
	}
	result, err := h.service.ImportCSV(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to import template csv", "TEMPLATE_OPS_CSV_IMPORT_FAILED", "Check csv headers, JSON columns, and platform logs to identify the import failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) PreviewImportCSV(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.csv.preview")
	defer span.End()
	var req CSVImportPreviewInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid csv preview request")
		return
	}
	result, err := h.service.PreviewImportCSV(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to preview template csv", "TEMPLATE_OPS_CSV_PREVIEW_FAILED", "Check csv headers, JSON columns, and asset references to identify the preview failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ExportCSV(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.csv.export")
	defer span.End()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	content, err := h.service.ExportCSV(c.Request.Context(), ListCatalogInput{
		ProductCode:   c.Query("product_code"),
		Query:         c.Query("query"),
		Locale:        c.Query("locale"),
		Limit:         limit,
		Offset:        offset,
		PublishedOnly: c.Query("published_only") == "true",
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to export template csv", "TEMPLATE_OPS_CSV_EXPORT_FAILED", "Check platform logs and export filters to identify the export failure.")
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="template_ops_catalog.csv"`)
	c.String(200, content)
}

func (h *Handler) ExportCSVTemplate(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="template_ops_import_template.csv"`)
	c.String(200, CSVTemplateExample())
}

func (h *Handler) ExportPreparedRealImportCSV(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.csv.real_sample")
	defer span.End()
	result, err := h.service.LoadPreparedRealImportBundle()
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load prepared real import csv", "TEMPLATE_OPS_REAL_IMPORT_LOAD_FAILED", "Check the generated real import artifacts in testdata/templateops/real-import.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ImportPreparedRealAssets(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.assets.import_prepared")
	defer span.End()
	var req PreparedAssetImportInput
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid prepared asset import request")
		return
	}
	if !req.OnlyMissing {
		req.OnlyMissing = true
	}
	result, err := h.service.ImportPreparedRealAssets(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to import prepared real assets", "TEMPLATE_OPS_PREPARED_ASSET_IMPORT_FAILED", "Check local asset paths, storage bindings, and the prepared manifest to identify the import failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) BatchUploadAssets(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.assets.batch_upload")
	defer span.End()
	var req BatchUploadAssetsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid batch asset upload request")
		return
	}
	result, err := h.service.BatchUploadAssets(c.Request.Context(), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to batch upload template assets", "TEMPLATE_OPS_ASSET_BATCH_UPLOAD_FAILED", "Check source_ref mappings, payload size, and storage configuration to identify the batch upload failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) GetDetail(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.catalog.detail")
	defer span.End()
	result, err := h.service.GetDetail(c.Request.Context(), c.Param("templateRef"), c.Query("locale"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "failed to load template detail", "TEMPLATE_OPS_DETAIL_FAILED", "Check template_ref and upstream product availability, then try again.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ListTemplateAssets(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.assets.list")
	defer span.End()
	result, err := h.service.ListTemplateAssets(c.Request.Context(), c.Param("templateRef"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "failed to load template asset bindings", "TEMPLATE_OPS_ASSET_LIST_FAILED", "Check template_ref and detail payload to identify the asset binding failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) UpsertTemplateAsset(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.assets.upsert")
	defer span.End()
	var req UpsertTemplateAssetInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid template asset request")
		return
	}
	req.AssetRole = c.Param("assetRole")
	result, err := h.service.UpsertTemplateAsset(c.Request.Context(), c.Param("templateRef"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to upsert template asset binding", "TEMPLATE_OPS_ASSET_UPSERT_FAILED", "Check asset payload, template_ref, and storage configuration to identify the binding failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) UnbindTemplateAsset(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/templateops-handler", "templateops.assets.unbind")
	defer span.End()
	result, err := h.service.UnbindTemplateAsset(c.Request.Context(), c.Param("templateRef"), c.Param("assetRole"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to unbind template asset", "TEMPLATE_OPS_ASSET_UNBIND_FAILED", "Check template_ref, asset_role, and template detail payload to identify the unbind failure.")
		return
	}
	response.JSONSuccess(c, result)
}
