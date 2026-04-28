package catalog

import (
	audit "platform-service/internal/modules/audit"
	"platform-service/internal/repository"
	"platform-service/internal/telemetry"
	"platform-service/pkg/metrics"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service     *Service
	financeRepo *repository.FinanceRepository
	audit       *audit.Service
}

func NewHandler(service *Service, financeRepo *repository.FinanceRepository, auditService *audit.Service) *Handler {
	return &Handler{service: service, financeRepo: financeRepo, audit: auditService}
}

func (h *Handler) CreateProduct(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.product.create")
	defer span.End()
	var req CreateProductInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create product request")
		return
	}
	item, err := h.service.CreateProduct(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create product", "CATALOG_PRODUCT_CREATE_FAILED", "Check platform logs with request_id and product code to identify the product creation failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_product_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "catalog.product.create",
			TargetType:    "product",
			TargetID:      item.ID,
			Details:       item.Code,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListProducts(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.product.list")
	defer span.End()
	items, err := h.service.ListProducts()
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list products", "CATALOG_PRODUCT_LIST_FAILED", "Retry the query and inspect platform logs with request_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) Offerings(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.offerings.get")
	defer span.End()
	productCode := c.Query("product_code")
	if productCode == "" {
		response.JSONError(c, response.CodeInvalidParameter, "missing product_code")
		return
	}
	item, err := h.service.Offerings(productCode, h.financeRepo)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load offerings", "CATALOG_OFFERINGS_GET_FAILED", "Check platform logs with request_id and product_code to identify the offerings aggregation failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.product.update")
	defer span.End()
	before, err := h.service.GetProduct(c.Param("productID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "product not found", "CATALOG_PRODUCT_NOT_FOUND", "Verify the product_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateProductInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.JSONBindError(c, bindErr, "invalid update product request")
		return
	}
	item, err := h.service.UpdateProduct(c.Param("productID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update product", "CATALOG_PRODUCT_UPDATE_FAILED", "Check platform logs with request_id and product_id to identify the product update failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_product_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.product.update",
			TargetType:     "product",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteProduct(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.product.delete")
	defer span.End()
	item, err := h.service.DeleteProduct(c.Param("productID"))
	if err != nil {
		response.JSONError(c, response.CodeNotFound, "product not found")
		return
	}
	metrics.IncBusinessCounter("catalog_product_deleted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.product.delete",
			TargetType:     "product",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: item,
		})
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": item.ID})
}

func (h *Handler) CreateSKU(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.sku.create")
	defer span.End()
	var req CreateSKUInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create sku request")
		return
	}
	item, err := h.service.CreateSKU(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create sku", "CATALOG_SKU_CREATE_FAILED", "Check platform logs with request_id, product_id, and sku code to identify the SKU creation failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_sku_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "catalog.sku.create",
			TargetType:    "sku",
			TargetID:      item.ID,
			Details:       item.Code,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListSKUs(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.sku.list")
	defer span.End()
	items, err := h.service.ListSKUs(c.Query("product_id"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list skus", "CATALOG_SKU_LIST_FAILED", "Retry the query and inspect platform logs with request_id and product_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateSKU(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.sku.update")
	defer span.End()
	before, err := h.service.GetSKU(c.Param("skuID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "sku not found", "CATALOG_SKU_NOT_FOUND", "Verify the sku_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateSKUInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.JSONBindError(c, bindErr, "invalid update sku request")
		return
	}
	item, err := h.service.UpdateSKU(c.Param("skuID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update sku", "CATALOG_SKU_UPDATE_FAILED", "Check platform logs with request_id and sku_id to identify the SKU update failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_sku_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.sku.update",
			TargetType:     "sku",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteSKU(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.sku.delete")
	defer span.End()
	item, err := h.service.DeleteSKU(c.Param("skuID"))
	if err != nil {
		response.JSONError(c, response.CodeNotFound, "sku not found")
		return
	}
	metrics.IncBusinessCounter("catalog_sku_deleted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.sku.delete",
			TargetType:     "sku",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: item,
		})
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": item.ID})
}

func (h *Handler) CreateBillableItem(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.billable_item.create")
	defer span.End()
	var req CreateBillableItemInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create billable item request")
		return
	}
	item, err := h.service.CreateBillableItem(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create billable item", "CATALOG_BILLABLE_ITEM_CREATE_FAILED", "Check platform logs with request_id, product_id, and billable item code to identify the creation failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_billable_item_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "catalog.billable_item.create",
			TargetType:    "billable_item",
			TargetID:      item.ID,
			Details:       item.Code,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListBillableItems(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.billable_item.list")
	defer span.End()
	items, err := h.service.ListBillableItems(c.Query("product_id"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list billable items", "CATALOG_BILLABLE_ITEM_LIST_FAILED", "Retry the query and inspect platform logs with request_id and product_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateBillableItem(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.billable_item.update")
	defer span.End()
	before, err := h.service.GetBillableItem(c.Param("billableItemID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "billable item not found", "CATALOG_BILLABLE_ITEM_NOT_FOUND", "Verify the billable_item_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateBillableItemInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.JSONBindError(c, bindErr, "invalid update billable item request")
		return
	}
	item, err := h.service.UpdateBillableItem(c.Param("billableItemID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update billable item", "CATALOG_BILLABLE_ITEM_UPDATE_FAILED", "Check platform logs with request_id and billable_item_id to identify the update failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_billable_item_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.billable_item.update",
			TargetType:     "billable_item",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteBillableItem(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.billable_item.delete")
	defer span.End()
	item, err := h.service.DeleteBillableItem(c.Param("billableItemID"))
	if err != nil {
		response.JSONError(c, response.CodeNotFound, "billable item not found")
		return
	}
	metrics.IncBusinessCounter("catalog_billable_item_deleted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.billable_item.delete",
			TargetType:     "billable_item",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: item,
		})
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": item.ID})
}

func (h *Handler) CreatePackage(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.package.create")
	defer span.End()
	var req CreatePackageInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create package request")
		return
	}
	item, err := h.service.CreatePackage(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create package", "CATALOG_PACKAGE_CREATE_FAILED", "Check platform logs with request_id, product_id, and package code to identify the package creation failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_package_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "catalog.package.create",
			TargetType:    "package",
			TargetID:      item.ID,
			Details:       item.Code,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListPackages(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.package.list")
	defer span.End()
	items, err := h.service.ListPackages(c.Query("product_id"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list packages", "CATALOG_PACKAGE_LIST_FAILED", "Retry the query and inspect platform logs with request_id and product_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdatePackage(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.package.update")
	defer span.End()
	before, err := h.service.GetPackage(c.Param("packageID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "package not found", "CATALOG_PACKAGE_NOT_FOUND", "Verify the package_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdatePackageInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.JSONBindError(c, bindErr, "invalid update package request")
		return
	}
	item, err := h.service.UpdatePackage(c.Param("packageID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update package", "CATALOG_PACKAGE_UPDATE_FAILED", "Check platform logs with request_id and package_id to identify the package update failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_package_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.package.update",
			TargetType:     "package",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeletePackage(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.package.delete")
	defer span.End()
	item, err := h.service.DeletePackage(c.Param("packageID"))
	if err != nil {
		response.JSONError(c, response.CodeNotFound, "package not found")
		return
	}
	metrics.IncBusinessCounter("catalog_package_deleted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.package.delete",
			TargetType:     "package",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: item,
		})
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": item.ID})
}

func (h *Handler) CreateRateCard(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.rate_card.create")
	defer span.End()
	var req CreateRateCardInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create rate card request")
		return
	}
	item, err := h.service.CreateRateCard(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create rate card", "CATALOG_RATE_CARD_CREATE_FAILED", "Check platform logs with request_id, product_id, and rate card code to identify the rate card creation failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_rate_card_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "catalog.rate_card.create",
			TargetType:    "rate_card",
			TargetID:      item.ID,
			Details:       item.Code,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListRateCards(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.rate_card.list")
	defer span.End()
	items, err := h.service.ListRateCards(c.Query("product_id"), c.Query("target_type"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list rate cards", "CATALOG_RATE_CARD_LIST_FAILED", "Retry the query and inspect platform logs with request_id, product_id, and target_type.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateRateCard(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.rate_card.update")
	defer span.End()
	before, err := h.service.GetRateCard(c.Param("rateCardID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "rate card not found", "CATALOG_RATE_CARD_NOT_FOUND", "Verify the rate_card_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateRateCardInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.JSONBindError(c, bindErr, "invalid update rate card request")
		return
	}
	item, err := h.service.UpdateRateCard(c.Param("rateCardID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update rate card", "CATALOG_RATE_CARD_UPDATE_FAILED", "Check platform logs with request_id and rate_card_id to identify the rate card update failure.")
		return
	}
	metrics.IncBusinessCounter("catalog_rate_card_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.rate_card.update",
			TargetType:     "rate_card",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteRateCard(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/catalog-handler", "catalog.rate_card.delete")
	defer span.End()
	item, err := h.service.DeleteRateCard(c.Param("rateCardID"))
	if err != nil {
		response.JSONError(c, response.CodeNotFound, "rate card not found")
		return
	}
	metrics.IncBusinessCounter("catalog_rate_card_deleted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "catalog.rate_card.delete",
			TargetType:     "rate_card",
			TargetID:       item.ID,
			Details:        item.Code,
			BeforeSnapshot: item,
		})
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": item.ID})
}
