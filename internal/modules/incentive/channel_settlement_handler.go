package incentive

import (
	"errors"

	audit "platform-service/internal/modules/audit"
	"platform-service/pkg/metrics"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListChannelClawbacks(c *gin.Context) {
	items, err := h.service.ListChannelClawbacks(c.Query("product_code"), c.Query("channel_partner_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel clawbacks", "CHANNEL_CLAWBACK_LIST_FAILED", "Retry the query and inspect platform logs with request_id and clawback filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) ListChannelSettlementBatches(c *gin.Context) {
	items, err := h.service.ListChannelSettlementBatches(c.Query("product_code"), c.Query("channel_program_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel settlement batches", "CHANNEL_SETTLEMENT_BATCH_LIST_FAILED", "Retry the query and inspect platform logs with request_id and settlement batch filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) GetChannelSettlementBatch(c *gin.Context) {
	item, err := h.service.GetChannelSettlementBatchDetail(c.Param("batchID"))
	if err != nil {
		if errors.Is(err, ErrChannelSettlementBatchProgramMissing) || errors.Is(err, ErrChannelSettlementBatchInvalidState) {
			response.JSONError(c, response.CodeBadRequest, "invalid settlement batch")
			return
		}
		response.JSONError(c, response.CodeNotFound, "channel settlement batch not found")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) ListChannelSettlementItems(c *gin.Context) {
	items, err := h.service.ListChannelSettlementItems(c.Query("batch_id"), c.Query("channel_partner_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel settlement items", "CHANNEL_SETTLEMENT_ITEM_LIST_FAILED", "Retry the query and inspect platform logs with request_id and settlement item filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) GenerateChannelSettlementBatch(c *gin.Context) {
	var req GenerateChannelSettlementBatchInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid generate channel settlement batch request")
		return
	}
	item, err := h.service.GenerateChannelSettlementBatch(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelSettlementBatchEmpty):
			response.JSONErrorSemantic(c, response.CodeConflict, "No eligible ledgers for settlement", "CHANNEL_SETTLEMENT_EMPTY", "Ensure earned commissions or clawbacks exist before generating a batch.")
		case errors.Is(err, ErrChannelSettlementBatchPeriodInvalid):
			response.JSONErrorSemantic(c, response.CodeInvalidParameter, "Invalid settlement period", "CHANNEL_SETTLEMENT_PERIOD_INVALID", "Use a valid RFC3339 period_start and period_end.")
		case errors.Is(err, ErrChannelSettlementBatchProgramMissing):
			response.JSONErrorSemantic(c, response.CodeNotFound, "Channel program not found", "CHANNEL_SETTLEMENT_PROGRAM_NOT_FOUND", "Use an existing channel program.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to generate channel settlement batch", "CHANNEL_SETTLEMENT_BATCH_GENERATE_FAILED", "Check platform logs with request_id, channel_program_id, and settlement period to identify the generation failure.")
		}
		return
	}
	metrics.IncBusinessCounter("channel_settlement_batch_generated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_settlement.generate",
			TargetType:    "channel_settlement_batch",
			TargetID:      item.Batch.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ConfirmChannelSettlementBatch(c *gin.Context) {
	h.updateChannelSettlementBatch(c, "confirm")
}

func (h *Handler) ProcessChannelSettlementBatch(c *gin.Context) {
	h.updateChannelSettlementBatch(c, "process")
}

func (h *Handler) CloseChannelSettlementBatch(c *gin.Context) {
	h.updateChannelSettlementBatch(c, "close")
}

func (h *Handler) CancelChannelSettlementBatch(c *gin.Context) {
	h.updateChannelSettlementBatch(c, "cancel")
}

func (h *Handler) updateChannelSettlementBatch(c *gin.Context, action string) {
	var req UpdateChannelSettlementBatchInput
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		response.JSONBindError(c, err, "invalid update channel settlement batch request")
		return
	}

	var (
		item *ChannelSettlementBatchDetail
		err  error
	)
	switch action {
	case "confirm":
		item, err = h.service.ConfirmChannelSettlementBatch(c.Param("batchID"), req)
	case "process":
		item, err = h.service.ProcessChannelSettlementBatch(c.Param("batchID"), req)
	case "close":
		item, err = h.service.CloseChannelSettlementBatch(c.Param("batchID"), req)
	case "cancel":
		item, err = h.service.CancelChannelSettlementBatch(c.Param("batchID"), req)
	}
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelSettlementBatchInvalidState):
			response.JSONErrorSemantic(c, response.CodeConflict, "Invalid settlement batch state", "CHANNEL_SETTLEMENT_INVALID_STATE", "Transition the batch in the correct order.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update channel settlement batch", "CHANNEL_SETTLEMENT_BATCH_UPDATE_FAILED", "Check platform logs with request_id, batch_id, and action to identify the batch transition failure.")
		}
		return
	}
	metrics.IncBusinessCounter("channel_settlement_batch_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_settlement." + action,
			TargetType:    "channel_settlement_batch",
			TargetID:      item.Batch.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccess(c, item)
}
