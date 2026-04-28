package wallet

import (
	"errors"
	"time"

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

func (h *Handler) CreateAccount(c *gin.Context) {
	span := startSpan(c, "wallet.account.create")
	defer span.End()
	var req CreateWalletAccountInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create wallet account request")
		return
	}
	item, err := h.service.CreateWalletAccount(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create wallet account", "WALLET_ACCOUNT_CREATE_FAILED", "Check platform logs with request_id, billing subject, and product_code to identify the wallet account creation failure.")
		return
	}
	metrics.IncBusinessCounter("wallet_account_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "wallet.account.create",
			TargetType:    "wallet_account",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) CreateAssetDefinition(c *gin.Context) {
	span := startSpan(c, "wallet.asset_definition.upsert")
	defer span.End()
	var req CreateAssetDefinitionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create asset definition request")
		return
	}
	item, err := h.service.CreateAssetDefinition(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create asset definition", "WALLET_ASSET_DEFINITION_CREATE_FAILED", "Check platform logs with request_id, asset_code, and product_code to identify the asset definition failure.")
		return
	}
	metrics.IncBusinessCounter("wallet_asset_definition_upserted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "wallet.asset_definition.upsert",
			TargetType:    "asset_definition",
			TargetID:      item.AssetCode,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListAssetDefinitions(c *gin.Context) {
	span := startSpan(c, "wallet.asset_definition.list")
	defer span.End()
	items, err := h.service.ListAssetDefinitions(c.Query("product_code"), c.Query("lifecycle_type"), c.Query("status"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list asset definitions", "WALLET_ASSET_DEFINITION_LIST_FAILED", "Retry the query and inspect platform logs with request_id and product_code.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateAssetDefinition(c *gin.Context) {
	span := startSpan(c, "wallet.asset_definition.update")
	defer span.End()
	before, err := h.service.GetAssetDefinition(c.Param("assetCode"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "asset definition not found", "WALLET_ASSET_DEFINITION_NOT_FOUND", "Verify the asset_code before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateAssetDefinitionInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.JSONBindError(c, bindErr, "invalid update asset definition request")
		return
	}
	item, err := h.service.UpdateAssetDefinition(c.Param("assetCode"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update asset definition", "WALLET_ASSET_DEFINITION_UPDATE_FAILED", "Check platform logs with request_id and asset_code to identify the asset definition update failure.")
		return
	}
	metrics.IncBusinessCounter("wallet_asset_definition_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "wallet.asset_definition.update",
			TargetType:     "asset_definition",
			TargetID:       item.AssetCode,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteAssetDefinition(c *gin.Context) {
	span := startSpan(c, "wallet.asset_definition.delete")
	defer span.End()
	item, err := h.service.DeleteAssetDefinition(c.Param("assetCode"))
	if err != nil {
		response.JSONError(c, response.CodeNotFound, "asset definition not found")
		return
	}
	metrics.IncBusinessCounter("wallet_asset_definition_deleted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "wallet.asset_definition.delete",
			TargetType:     "asset_definition",
			TargetID:       item.AssetCode,
			BeforeSnapshot: item,
		})
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": item.AssetCode})
}

func (h *Handler) CreateAllowancePolicy(c *gin.Context) {
	span := startSpan(c, "wallet.allowance_policy.upsert")
	defer span.End()
	var req CreateAllowancePolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create allowance policy request")
		return
	}
	item, err := h.service.CreateAllowancePolicy(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create allowance policy", "WALLET_ALLOWANCE_POLICY_CREATE_FAILED", "Check platform logs with request_id, asset_code, and product_code to identify the allowance policy failure.")
		return
	}
	metrics.IncBusinessCounter("wallet_allowance_policy_upserted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "wallet.allowance_policy.upsert",
			TargetType:    "allowance_policy",
			TargetID:      item.ID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListAllowancePolicies(c *gin.Context) {
	span := startSpan(c, "wallet.allowance_policy.list")
	defer span.End()
	items, err := h.service.ListAllowancePolicies(c.Query("product_code"), c.Query("asset_code"), c.Query("status"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list allowance policies", "WALLET_ALLOWANCE_POLICY_LIST_FAILED", "Retry the query and inspect platform logs with request_id, product_code, and asset_code.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateAllowancePolicy(c *gin.Context) {
	span := startSpan(c, "wallet.allowance_policy.update")
	defer span.End()
	before, err := h.service.GetAllowancePolicy(c.Param("policyID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "allowance policy not found", "WALLET_ALLOWANCE_POLICY_NOT_FOUND", "Verify the policy_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateAllowancePolicyInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.JSONBindError(c, bindErr, "invalid update allowance policy request")
		return
	}
	item, err := h.service.UpdateAllowancePolicy(c.Param("policyID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update allowance policy", "WALLET_ALLOWANCE_POLICY_UPDATE_FAILED", "Check platform logs with request_id and policy_id to identify the allowance policy update failure.")
		return
	}
	metrics.IncBusinessCounter("wallet_allowance_policy_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "wallet.allowance_policy.update",
			TargetType:     "allowance_policy",
			TargetID:       item.ID,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteAllowancePolicy(c *gin.Context) {
	span := startSpan(c, "wallet.allowance_policy.delete")
	defer span.End()
	item, err := h.service.DeleteAllowancePolicy(c.Param("policyID"))
	if err != nil {
		response.JSONError(c, response.CodeNotFound, "allowance policy not found")
		return
	}
	metrics.IncBusinessCounter("wallet_allowance_policy_deleted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "wallet.allowance_policy.delete",
			TargetType:     "allowance_policy",
			TargetID:       item.ID,
			BeforeSnapshot: item,
		})
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": item.ID})
}

// ListAccounts godoc
// @Summary List wallet accounts
// @Description Query wallet accounts by billing subject.
// @Tags Internal Wallet
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param billing_subject_type query string false "Billing subject type"
// @Param billing_subject_id query string false "Billing subject id"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/wallet/accounts [get]
func (h *Handler) ListAccounts(c *gin.Context) {
	span := startSpan(c, "wallet.account.list")
	defer span.End()
	scope, ok := productscope.Resolve(c)
	if !ok {
		return
	}
	items, err := h.service.ListScopedWalletAccounts(c.Query("billing_subject_type"), c.Query("billing_subject_id"), scope.ProductCode, scope.IncludeAll)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list wallet accounts", "WALLET_ACCOUNT_LIST_FAILED", "Retry the query and inspect platform logs with request_id and billing subject filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) ListBuckets(c *gin.Context) {
	span := startSpan(c, "wallet.bucket.list")
	defer span.End()
	items, err := h.service.ListWalletBuckets(c.Query("wallet_account_id"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list wallet buckets", "WALLET_BUCKET_LIST_FAILED", "Retry the query and inspect platform logs with request_id and wallet_account_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) GetSummary(c *gin.Context) {
	span := startSpan(c, "wallet.summary.get")
	defer span.End()
	scope, ok := productscope.Resolve(c)
	if !ok {
		return
	}
	item, err := h.service.GetWalletSummary(
		c.Query("billing_subject_type"),
		c.Query("billing_subject_id"),
		scope.ProductCode,
		time.Now(),
	)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to get wallet summary", "WALLET_SUMMARY_GET_FAILED", "Retry the query and inspect platform logs with request_id, billing subject, and product_code.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) PostLedger(c *gin.Context) {
	span := startSpan(c, "wallet.ledger.post")
	defer span.End()
	var req PostWalletLedgerInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid wallet ledger request")
		return
	}
	item, account, err := h.service.PostLedger(req)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, ErrInsufficientWalletBalance) {
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientWallet, err.Error(), "WALLET_LEDGER_INSUFFICIENT_BALANCE", "Recharge the wallet account or reduce the debit amount before retrying.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to post wallet ledger", "WALLET_LEDGER_POST_FAILED", "Check platform logs with request_id, billing subject, wallet account, and asset_code to identify the ledger failure.")
		return
	}
	metrics.IncBusinessCounter("wallet_ledger_posted_total")
	if h.audit != nil && item != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "wallet.ledger.post",
			TargetType:         "wallet_ledger",
			TargetID:           item.ID,
			BillingSubjectType: item.BillingSubjectType,
			BillingSubjectID:   item.BillingSubjectID,
			AfterSnapshot:      item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, gin.H{"ledger": item, "account": account})
}

func (h *Handler) GrantCycleAllowance(c *gin.Context) {
	span := startSpan(c, "wallet.allowance.grant")
	defer span.End()
	var req GrantCycleAllowanceInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid cycle allowance request")
		return
	}
	bucket, account, err := h.service.GrantCycleAllowance(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to grant cycle allowance", "WALLET_CYCLE_ALLOWANCE_GRANT_FAILED", "Check platform logs with request_id, asset_code, billing subject, and policy configuration.")
		return
	}
	metrics.IncBusinessCounter("wallet_cycle_allowance_granted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "wallet.allowance.grant",
			TargetType:    "wallet_bucket",
			TargetID:      bucket.ID,
			AfterSnapshot: gin.H{"account": account, "bucket": bucket},
		})
	}
	response.JSONSuccessWithStatus(c, 201, gin.H{"bucket": bucket, "account": account})
}

func (h *Handler) ExpireBuckets(c *gin.Context) {
	span := startSpan(c, "wallet.bucket.expire")
	defer span.End()
	items, err := h.service.ExpireWalletBuckets(c.Query("asset_code"), time.Now())
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to expire wallet buckets", "WALLET_BUCKET_EXPIRE_FAILED", "Check platform logs with request_id and asset_code to identify the expiration run failure.")
		return
	}
	metrics.IncBusinessCounter("wallet_buckets_expire_runs_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "wallet.bucket.expire",
			TargetType:    "wallet_bucket_batch",
			TargetID:      defaultString(c.Query("asset_code"), "all"),
			AfterSnapshot: gin.H{"count": len(items)},
		})
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) RunLifecycle(c *gin.Context) {
	span := startSpan(c, "wallet.lifecycle.run")
	defer span.End()
	item, err := h.service.RunLifecycleOnce(time.Now())
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to run wallet lifecycle", "WALLET_LIFECYCLE_RUN_FAILED", "Check platform logs with request_id to identify the lifecycle execution failure.")
		return
	}
	metrics.IncBusinessCounter("wallet_lifecycle_runs_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "wallet.lifecycle.run",
			TargetType:    "wallet_lifecycle",
			TargetID:      "manual",
			AfterSnapshot: item,
		})
	}
	response.JSONSuccess(c, item)
}

// ListLedger godoc
// @Summary List wallet ledger
// @Description Query wallet ledger by wallet account id.
// @Tags Internal Wallet
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param wallet_account_id query string false "Wallet account id"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/wallet/ledger [get]
func (h *Handler) ListLedger(c *gin.Context) {
	span := startSpan(c, "wallet.ledger.list")
	defer span.End()
	scope, ok := productscope.Resolve(c)
	if !ok {
		return
	}
	items, err := h.service.ListScopedWalletLedger(c.Query("wallet_account_id"), scope.ProductCode, scope.IncludeAll)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list wallet ledger", "WALLET_LEDGER_LIST_FAILED", "Retry the query and inspect platform logs with request_id and wallet_account_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func startSpan(c *gin.Context, name string) trace.Span {
	ctx, span := otel.Tracer("platform-service").Start(c.Request.Context(), name)
	c.Request = c.Request.WithContext(ctx)
	return span
}
