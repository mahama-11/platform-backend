package incentive

import (
	"errors"

	audit "platform-service/internal/modules/audit"
	"platform-service/pkg/metrics"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateChannelPartner(c *gin.Context) {
	var req CreateChannelPartnerInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create channel partner request")
		return
	}
	item, err := h.service.CreateChannelPartner(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelPartnerExists):
			response.JSONErrorSemantic(c, response.CodeConflict, "Channel partner already exists", "CHANNEL_PARTNER_EXISTS", "Use a unique channel partner code.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create channel partner", "CHANNEL_PARTNER_CREATE_FAILED", "Check platform logs with request_id and channel partner code to identify the creation failure.")
		}
		return
	}
	metrics.IncBusinessCounter("channel_partner_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_partner.create",
			TargetType:    "channel_partner",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListChannelPartners(c *gin.Context) {
	items, err := h.service.ListChannelPartners(c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel partners", "CHANNEL_PARTNER_LIST_FAILED", "Retry the query and inspect platform logs with request_id and channel partner filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateChannelProgram(c *gin.Context) {
	var req CreateChannelProgramInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create channel program request")
		return
	}
	item, err := h.service.CreateChannelProgram(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelProgramExists):
			response.JSONErrorSemantic(c, response.CodeConflict, "Channel program already exists", "CHANNEL_PROGRAM_EXISTS", "Use a unique channel program code.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create channel program", "CHANNEL_PROGRAM_CREATE_FAILED", "Check platform logs with request_id, product_code, and channel program code to identify the creation failure.")
		}
		return
	}
	metrics.IncBusinessCounter("channel_program_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_program.create",
			TargetType:    "channel_program",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListChannelPrograms(c *gin.Context) {
	items, err := h.service.ListChannelPrograms(c.Query("product_code"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel programs", "CHANNEL_PROGRAM_LIST_FAILED", "Retry the query and inspect platform logs with request_id and channel program filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateChannelBinding(c *gin.Context) {
	var req CreateChannelBindingInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create channel binding request")
		return
	}
	item, err := h.service.CreateChannelBinding(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelPartnerInactive):
			response.JSONErrorSemantic(c, response.CodeForbidden, "Channel partner is inactive", "CHANNEL_PARTNER_INACTIVE", "Activate the channel partner before binding.")
		case errors.Is(err, ErrChannelProgramInactive):
			response.JSONErrorSemantic(c, response.CodeForbidden, "Channel program is inactive", "CHANNEL_PROGRAM_INACTIVE", "Activate the channel program before binding.")
		case errors.Is(err, ErrChannelBindingLocked):
			response.JSONErrorSemantic(c, response.CodeConflict, "Channel binding is locked", "CHANNEL_BINDING_LOCKED", "Wait for the lock window to expire or override manually.")
		case errors.Is(err, ErrChannelBindingExists):
			response.JSONErrorSemantic(c, response.CodeConflict, "Active channel binding already exists", "CHANNEL_BINDING_EXISTS", "Supersede or expire the current binding before creating a new one.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create channel binding", "CHANNEL_BINDING_CREATE_FAILED", "Check platform logs with request_id, product_code, org_id, and channel binding scope to identify the creation failure.")
		}
		return
	}
	metrics.IncBusinessCounter("channel_binding_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_binding.create",
			TargetType:    "channel_partner_binding",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListChannelBindings(c *gin.Context) {
	items, err := h.service.ListChannelBindings(c.Query("product_code"), c.Query("org_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel bindings", "CHANNEL_BINDING_LIST_FAILED", "Retry the query and inspect platform logs with request_id and channel binding filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateChannelCommissionPolicy(c *gin.Context) {
	var req CreateChannelCommissionPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create channel commission policy request")
		return
	}
	item, err := h.service.CreateChannelCommissionPolicy(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelPolicyExists):
			response.JSONErrorSemantic(c, response.CodeConflict, "Channel commission policy already exists", "CHANNEL_POLICY_EXISTS", "Use a unique channel policy code.")
		case errors.Is(err, ErrChannelPolicyNotSupported):
			response.JSONErrorSemantic(c, response.CodeInvalidParameter, "Unsupported channel commission policy", "CHANNEL_POLICY_UNSUPPORTED", "Use a supported rate type and commission base.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create channel commission policy", "CHANNEL_POLICY_CREATE_FAILED", "Check platform logs with request_id, channel_program_id, and policy code to identify the creation failure.")
		}
		return
	}
	metrics.IncBusinessCounter("channel_policy_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_policy.create",
			TargetType:    "channel_commission_policy",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListChannelCommissionPolicies(c *gin.Context) {
	items, err := h.service.ListChannelCommissionPolicies(c.Query("channel_program_id"), c.Query("product_code"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel commission policies", "CHANNEL_POLICY_LIST_FAILED", "Retry the query and inspect platform logs with request_id and channel policy filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateChannelCommissionPolicyVersion(c *gin.Context) {
	var req CreateChannelCommissionPolicyVersionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create channel commission policy version request")
		return
	}
	item, err := h.service.CreateChannelCommissionPolicyVersion(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelPolicyVersionExists):
			response.JSONErrorSemantic(c, response.CodeConflict, "Channel commission policy version already exists", "CHANNEL_POLICY_VERSION_EXISTS", "Use a unique channel policy version code.")
		case errors.Is(err, ErrChannelPolicyNotSupported):
			response.JSONErrorSemantic(c, response.CodeInvalidParameter, "Unsupported channel commission policy version", "CHANNEL_POLICY_VERSION_UNSUPPORTED", "Use a supported commission base, rate type, and config.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create channel commission policy version", "CHANNEL_POLICY_VERSION_CREATE_FAILED", "Check platform logs with request_id, policy_id, and version code to identify the creation failure.")
		}
		return
	}
	metrics.IncBusinessCounter("channel_policy_version_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_policy_version.create",
			TargetType:    "channel_commission_policy_version",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListChannelCommissionPolicyVersions(c *gin.Context) {
	items, err := h.service.ListChannelCommissionPolicyVersions(c.Query("policy_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel commission policy versions", "CHANNEL_POLICY_VERSION_LIST_FAILED", "Retry the query and inspect platform logs with request_id and policy version filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateChannelCommissionPolicyAssignment(c *gin.Context) {
	var req CreateChannelCommissionPolicyAssignmentInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create channel commission policy assignment request")
		return
	}
	item, err := h.service.CreateChannelCommissionPolicyAssignment(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelPolicyAssignmentConflict):
			response.JSONErrorSemantic(c, response.CodeConflict, "Channel commission policy assignment overlaps an active assignment", "CHANNEL_POLICY_ASSIGNMENT_CONFLICT", "Expire the current assignment or use a different scope or time window.")
		case errors.Is(err, ErrChannelPolicyNotSupported):
			response.JSONErrorSemantic(c, response.CodeInvalidParameter, "Unsupported channel commission policy assignment", "CHANNEL_POLICY_ASSIGNMENT_UNSUPPORTED", "Use a supported assignment level and scope.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create channel commission policy assignment", "CHANNEL_POLICY_ASSIGNMENT_CREATE_FAILED", "Check platform logs with request_id, policy_version_id, and assignment scope to identify the creation failure.")
		}
		return
	}
	metrics.IncBusinessCounter("channel_policy_assignment_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_policy_assignment.create",
			TargetType:    "channel_commission_policy_assignment",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListChannelCommissionPolicyAssignments(c *gin.Context) {
	items, err := h.service.ListChannelCommissionPolicyAssignments(c.Query("policy_version_id"), c.Query("product_code"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel commission policy assignments", "CHANNEL_POLICY_ASSIGNMENT_LIST_FAILED", "Retry the query and inspect platform logs with request_id and assignment filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) ListChannelProfitSnapshots(c *gin.Context) {
	items, err := h.service.ListChannelProfitSnapshots(c.Query("product_code"), c.Query("org_id"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel profit snapshots", "CHANNEL_PROFIT_SNAPSHOT_LIST_FAILED", "Retry the query and inspect platform logs with request_id and profit snapshot filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateChannelCommissionAdjustment(c *gin.Context) {
	var req CreateChannelCommissionAdjustmentInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create channel commission adjustment request")
		return
	}
	item, err := h.service.CreateChannelCommissionAdjustmentLedger(req)
	if err != nil {
		if errors.Is(err, ErrChannelPolicyNotSupported) {
			response.JSONErrorSemantic(c, response.CodeInvalidParameter, "Unsupported channel commission adjustment", "CHANNEL_ADJUSTMENT_UNSUPPORTED", "Use a supported adjustment type and non-zero amount.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create channel commission adjustment", "CHANNEL_ADJUSTMENT_CREATE_FAILED", "Check platform logs with request_id, channel_partner_id, and adjustment payload to identify the creation failure.")
		return
	}
	metrics.IncBusinessCounter("channel_adjustment_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.channel_adjustment.create",
			TargetType:    "channel_commission_adjustment_ledger",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListChannelCommissionAdjustments(c *gin.Context) {
	items, err := h.service.ListChannelCommissionAdjustmentLedgers(c.Query("product_code"), c.Query("channel_partner_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel commission adjustments", "CHANNEL_ADJUSTMENT_LIST_FAILED", "Retry the query and inspect platform logs with request_id and adjustment filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) PreviewChannelPolicyResolution(c *gin.Context) {
	var req PreviewChannelPolicyResolutionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid channel policy resolution preview request")
		return
	}
	result, err := h.service.PreviewChannelPolicyResolution(req)
	if err != nil {
		if errors.Is(err, ErrChannelPolicyResolutionConflict) {
			response.JSONErrorSemantic(c, response.CodeConflict, "Channel policy resolution conflict", "CHANNEL_POLICY_RESOLUTION_CONFLICT", "Review overlapping assignments or priorities before publishing.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to preview channel policy resolution", "CHANNEL_POLICY_RESOLUTION_PREVIEW_FAILED", "Check platform logs with request_id and preview payload to identify the resolution failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ListChannelCommissions(c *gin.Context) {
	items, err := h.service.ListChannelCommissionLedger(c.Query("product_code"), c.Query("channel_partner_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list channel commissions", "CHANNEL_COMMISSION_LIST_FAILED", "Retry the query and inspect platform logs with request_id and commission filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) RecordChannelCharge(c *gin.Context) {
	var req RecordChannelChargeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid channel charge event request")
		return
	}
	result, err := h.service.RecordChannelCharge(req)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to record channel charge", "CHANNEL_CHARGE_RECORD_FAILED", "Check platform logs with request_id, product_code, and source charge identifiers to identify the charge record failure.")
		return
	}
	if result.Matched {
		metrics.IncBusinessCounter("channel_charge_recorded_total")
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) RecordChannelRefund(c *gin.Context) {
	var req RecordChannelRefundInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid channel refund event request")
		return
	}
	result, err := h.service.RecordChannelRefund(req)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to record channel refund", "CHANNEL_REFUND_RECORD_FAILED", "Check platform logs with request_id, product_code, and source refund identifiers to identify the refund record failure.")
		return
	}
	if result.Matched {
		metrics.IncBusinessCounter("channel_refund_recorded_total")
	}
	response.JSONSuccess(c, result)
}
