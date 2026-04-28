package incentive

import (
	"errors"
	audit "platform-service/internal/modules/audit"
	"platform-service/internal/modules/productscope"
	"platform-service/pkg/metrics"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	service *Service
	audit   *audit.Service
}

func NewHandler(service *Service, auditService *audit.Service) *Handler {
	return &Handler{service: service, audit: auditService}
}

func (h *Handler) CreateReward(c *gin.Context) {
	span := startSpan(c, "incentive.reward.create")
	defer span.End()
	var req CreateRewardInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create reward request")
		return
	}
	item, err := h.service.CreateReward(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create reward", "INCENTIVE_REWARD_CREATE_FAILED", "Check platform logs with request_id, product_code, and beneficiary subject to identify the reward creation failure.")
		return
	}
	metrics.IncBusinessCounter("reward_ledger_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.reward.create",
			TargetType:    "reward_ledger",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

// ListRewards godoc
// @Summary List reward ledgers
// @Description Query reward ledgers by product or beneficiary.
// @Tags Internal Incentives
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param product_code query string false "Product code"
// @Param beneficiary_subject_type query string false "Beneficiary subject type"
// @Param beneficiary_subject_id query string false "Beneficiary subject id"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/incentives/rewards [get]
func (h *Handler) ListRewards(c *gin.Context) {
	scope, ok := productscope.Resolve(c)
	if !ok {
		return
	}
	items, err := h.service.ListRewardLedgers(
		scope.ProductCode,
		firstNonEmpty(c.Query("beneficiary_subject_type"), c.Query("subject_type")),
		firstNonEmpty(c.Query("beneficiary_subject_id"), c.Query("subject_id")),
	)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list rewards", "INCENTIVE_REWARD_LIST_FAILED", "Retry the query and inspect platform logs with request_id and reward filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateReward(c *gin.Context) {
	span := startSpan(c, "incentive.reward.update")
	defer span.End()
	before, err := h.service.GetRewardLedger(c.Param("rewardID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "reward not found", "INCENTIVE_REWARD_NOT_FOUND", "Verify the reward_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateRewardInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.RecordError(bindErr)
		response.JSONBindError(c, bindErr, "invalid update reward request")
		return
	}
	item, err := h.service.UpdateReward(c.Param("rewardID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update reward", "INCENTIVE_REWARD_UPDATE_FAILED", "Check platform logs with request_id and reward_id to identify the reward update failure.")
		return
	}
	metrics.IncBusinessCounter("reward_ledger_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "incentive.reward.update",
			TargetType:     "reward_ledger",
			TargetID:       item.ID,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) CreateCommission(c *gin.Context) {
	span := startSpan(c, "incentive.commission.create")
	defer span.End()
	var req CreateCommissionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create commission request")
		return
	}
	item, err := h.service.CreateCommission(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create commission", "INCENTIVE_COMMISSION_CREATE_FAILED", "Check platform logs with request_id, product_code, and beneficiary subject to identify the commission creation failure.")
		return
	}
	metrics.IncBusinessCounter("commission_ledger_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.commission.create",
			TargetType:    "commission_ledger",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

// ListCommissions godoc
// @Summary List commission ledgers
// @Description Query commission ledgers by product or beneficiary.
// @Tags Internal Incentives
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param product_code query string false "Product code"
// @Param beneficiary_subject_type query string false "Beneficiary subject type"
// @Param beneficiary_subject_id query string false "Beneficiary subject id"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/incentives/commissions [get]
func (h *Handler) ListCommissions(c *gin.Context) {
	scope, ok := productscope.Resolve(c)
	if !ok {
		return
	}
	items, err := h.service.ListCommissionLedgers(
		scope.ProductCode,
		firstNonEmpty(c.Query("beneficiary_subject_type"), c.Query("subject_type")),
		firstNonEmpty(c.Query("beneficiary_subject_id"), c.Query("subject_id")),
		c.Query("status"),
	)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list commissions", "INCENTIVE_COMMISSION_LIST_FAILED", "Retry the query and inspect platform logs with request_id and commission filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) ResolveReferralCode(c *gin.Context) {
	item, err := h.service.ResolveReferralCode(c.Param("code"), c.Query("product_code"))
	if err != nil {
		switch {
		case errors.Is(err, ErrReferralCodeNotFound):
			response.WriteObservedSemanticError(c, err, response.CodeNotFound, "Referral code not found", "REFERRAL_CODE_NOT_FOUND", "Check the code and try again.")
		case errors.Is(err, ErrReferralCodeInactive), errors.Is(err, ErrReferralProgramInactive):
			response.WriteObservedSemanticError(c, err, response.CodeForbidden, "Referral code is inactive", "REFERRAL_CODE_INACTIVE", "Use an active referral code.")
		case errors.Is(err, ErrReferralProductMismatch):
			response.WriteObservedSemanticError(c, err, response.CodeForbidden, "Referral code does not apply to this product", "REFERRAL_PRODUCT_MISMATCH", "Use a referral code for the current product.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to resolve referral code", "REFERRAL_CODE_RESOLVE_FAILED", "Check platform logs with request_id, referral_code, and product_code to identify the resolution failure.")
		}
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) UpdateCommission(c *gin.Context) {
	span := startSpan(c, "incentive.commission.update")
	defer span.End()
	before, err := h.service.GetCommissionLedger(c.Param("commissionID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "commission not found", "INCENTIVE_COMMISSION_NOT_FOUND", "Verify the commission_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateCommissionInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.RecordError(bindErr)
		response.JSONBindError(c, bindErr, "invalid update commission request")
		return
	}
	item, err := h.service.UpdateCommission(c.Param("commissionID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update commission", "INCENTIVE_COMMISSION_UPDATE_FAILED", "Check platform logs with request_id and commission_id to identify the commission update failure.")
		return
	}
	metrics.IncBusinessCounter("commission_ledger_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "incentive.commission.update",
			TargetType:     "commission_ledger",
			TargetID:       item.ID,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) RedeemCommissions(c *gin.Context) {
	span := startSpan(c, "incentive.commission.redeem")
	defer span.End()
	var req RedeemCommissionsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid redeem commissions request")
		return
	}
	item, err := h.service.RedeemCommissions(req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrNoRedeemableCommission):
			response.WriteObservedSemanticError(c, err, response.CodeConflict, "No redeemable commission available", "NO_REDEEMABLE_COMMISSION", "Earned commissions are required before redeeming credits.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to redeem commissions", "INCENTIVE_COMMISSION_REDEEM_FAILED", "Check platform logs with request_id, beneficiary subject, and redeem request details.")
		}
		return
	}
	metrics.IncBusinessCounter("commission_redeemed_total")
	response.JSONSuccess(c, item)
}

func (h *Handler) CreateReferralProgram(c *gin.Context) {
	span := startSpan(c, "incentive.referral_program.create")
	defer span.End()
	var req CreateReferralProgramInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create referral program request")
		return
	}
	item, err := h.service.CreateReferralProgram(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create referral program", "REFERRAL_PROGRAM_CREATE_FAILED", "Check platform logs with request_id, product_code, and trigger configuration to identify the program creation failure.")
		return
	}
	metrics.IncBusinessCounter("referral_program_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.referral_program.create",
			TargetType:    "referral_program",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListReferralPrograms(c *gin.Context) {
	items, err := h.service.ListReferralPrograms(c.Query("product_code"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list referral programs", "REFERRAL_PROGRAM_LIST_FAILED", "Retry the query and inspect platform logs with request_id and referral program filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateReferralProgram(c *gin.Context) {
	span := startSpan(c, "incentive.referral_program.update")
	defer span.End()
	before, err := h.service.GetReferralProgram(c.Param("programID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "referral program not found", "REFERRAL_PROGRAM_NOT_FOUND", "Verify the program_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateReferralProgramInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.RecordError(bindErr)
		response.JSONBindError(c, bindErr, "invalid update referral program request")
		return
	}
	item, err := h.service.UpdateReferralProgram(c.Param("programID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update referral program", "REFERRAL_PROGRAM_UPDATE_FAILED", "Check platform logs with request_id and program_id to identify the referral program update failure.")
		return
	}
	metrics.IncBusinessCounter("referral_program_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "incentive.referral_program.update",
			TargetType:     "referral_program",
			TargetID:       item.ID,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) CreateReferralCode(c *gin.Context) {
	span := startSpan(c, "incentive.referral_code.create")
	defer span.End()
	var req CreateReferralCodeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create referral code request")
		return
	}
	item, err := h.service.CreateReferralCode(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create referral code", "REFERRAL_CODE_CREATE_FAILED", "Check platform logs with request_id, program_id, and promoter subject to identify the referral code creation failure.")
		return
	}
	metrics.IncBusinessCounter("referral_code_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.referral_code.create",
			TargetType:    "referral_code",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListReferralCodes(c *gin.Context) {
	items, err := h.service.ListReferralCodes(c.Query("program_id"), c.Query("promoter_subject_type"), c.Query("promoter_subject_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list referral codes", "REFERRAL_CODE_LIST_FAILED", "Retry the query and inspect platform logs with request_id and referral code filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateReferralCode(c *gin.Context) {
	span := startSpan(c, "incentive.referral_code.update")
	defer span.End()
	before, err := h.service.GetReferralCode(c.Param("code"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "referral code not found", "REFERRAL_CODE_NOT_FOUND", "Verify the referral code before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateReferralCodeInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.RecordError(bindErr)
		response.JSONBindError(c, bindErr, "invalid update referral code request")
		return
	}
	item, err := h.service.UpdateReferralCode(c.Param("code"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update referral code", "REFERRAL_CODE_UPDATE_FAILED", "Check platform logs with request_id and referral code to identify the update failure.")
		return
	}
	metrics.IncBusinessCounter("referral_code_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "incentive.referral_code.update",
			TargetType:     "referral_code",
			TargetID:       item.ID,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) CreateReferralConversion(c *gin.Context) {
	span := startSpan(c, "incentive.referral_conversion.create")
	defer span.End()
	var req CreateReferralConversionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create referral conversion request")
		return
	}
	item, err := h.service.CreateReferralConversion(req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrReferralConversionExists), errors.Is(err, ErrReferralAlreadyClaimed):
			response.WriteObservedSemanticError(c, err, response.CodeConflict, "Referral reward already claimed", "REFERRAL_ALREADY_CLAIMED", "This account has already used the available referral reward.")
		case errors.Is(err, ErrReferralSelfInviteBlocked):
			response.WriteObservedSemanticError(c, err, response.CodeForbidden, "Self referral is not allowed", "REFERRAL_SELF_INVITE_BLOCKED", "Use a referral code from a different promoter.")
		case errors.Is(err, ErrReferralCodeInactive), errors.Is(err, ErrReferralProgramInactive):
			response.WriteObservedSemanticError(c, err, response.CodeForbidden, "Referral code is inactive", "REFERRAL_CODE_INACTIVE", "Use an active referral code.")
		case errors.Is(err, ErrReferralProductMismatch):
			response.WriteObservedSemanticError(c, err, response.CodeForbidden, "Referral code does not apply to this product", "REFERRAL_PRODUCT_MISMATCH", "Use a referral code for the current product.")
		case errors.Is(err, ErrReferralTriggerMismatch):
			response.WriteObservedSemanticError(c, err, response.CodeForbidden, "Referral code does not apply to this trigger", "REFERRAL_TRIGGER_NOT_ELIGIBLE", "Complete the required trigger before expecting referral rewards.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create referral conversion", "REFERRAL_CONVERSION_CREATE_FAILED", "Check platform logs with request_id, referral code, claimant subject, and trigger details.")
		}
		return
	}
	metrics.IncBusinessCounter("referral_conversion_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "incentive.referral_conversion.create",
			TargetType:    "referral_conversion",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListReferralConversions(c *gin.Context) {
	items, err := h.service.ListReferralConversions(c.Query("product_code"), c.Query("promoter_subject_type"), c.Query("promoter_subject_id"), c.Query("status"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list referral conversions", "REFERRAL_CONVERSION_LIST_FAILED", "Retry the query and inspect platform logs with request_id and conversion filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateReferralConversion(c *gin.Context) {
	span := startSpan(c, "incentive.referral_conversion.update")
	defer span.End()
	before, err := h.service.GetReferralConversion(c.Param("conversionID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "referral conversion not found", "REFERRAL_CONVERSION_NOT_FOUND", "Verify the conversion_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateReferralConversionInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.RecordError(bindErr)
		response.JSONBindError(c, bindErr, "invalid update referral conversion request")
		return
	}
	item, err := h.service.UpdateReferralConversion(c.Param("conversionID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update referral conversion", "REFERRAL_CONVERSION_UPDATE_FAILED", "Check platform logs with request_id and conversion_id to identify the update failure.")
		return
	}
	metrics.IncBusinessCounter("referral_conversion_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "incentive.referral_conversion.update",
			TargetType:     "referral_conversion",
			TargetID:       item.ID,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func startSpan(c *gin.Context, name string) trace.Span {
	ctx, span := otel.Tracer("platform-service").Start(c.Request.Context(), name)
	c.Request = c.Request.WithContext(ctx)
	return span
}
