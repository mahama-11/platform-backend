package docs

import (
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

type InternalAccessGuide struct {
	BasePath    string   `json:"base_path"`
	AuthMethod  string   `json:"auth_method"`
	Headers     []string `json:"headers"`
	WriteFlows  []string `json:"write_flows"`
	QueryFlows  []string `json:"query_flows"`
	Idempotency []string `json:"idempotency"`
	RetryRules  []string `json:"retry_rules"`
}

type ErrorCodeEntry struct {
	Code       response.ResponseCode `json:"code"`
	Name       string                `json:"name"`
	Message    string                `json:"message"`
	HTTPStatus int                   `json:"http_status"`
}

type ErrorCodesDoc struct {
	Client   []ErrorCodeEntry `json:"client_errors"`
	Business []ErrorCodeEntry `json:"business_errors"`
	Payment  []ErrorCodeEntry `json:"payment_errors"`
	Server   []ErrorCodeEntry `json:"server_errors"`
}

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) InternalAccessDoc(c *gin.Context) {
	out := InternalAccessGuide{
		BasePath:   "/internal/v1",
		AuthMethod: "HMAC signature or legacy shared secret",
		Headers: []string{
			platformconst.HeaderInternalService,
			platformconst.HeaderInternalTimestamp,
			platformconst.HeaderInternalSignature,
			platformconst.HeaderRequestID,
			platformconst.HeaderTraceID,
		},
		WriteFlows: []string{
			"POST /controls/reservations -> POST /controls/reservations/:reservationID/commit or /release",
			"POST /metering/events -> GET /metering/settlements/:eventID",
			"POST /metering/settlements/:eventID/reverse for compensation",
			"POST /commercial/route/resolve for billing subject/commercial routing decision",
		},
		QueryFlows: []string{
			"GET /metering/settlements",
			"GET /metering/discounts",
			"GET /wallet/accounts",
			"GET /wallet/ledger",
			"GET /incentives/rewards",
			"GET /incentives/commissions",
		},
		Idempotency: []string{
			"`metering/events` must use stable `event_id` per business event",
			"`settlements/:eventID/reverse` must be treated as one reverse intent per settlement",
			"`controls/reservations` caller should persist reservation id and avoid duplicate reserve",
		},
		RetryRules: []string{
			"Retry only on transport failure or 5xx",
			"On 409/404, read settlement or reservation state first before retrying",
			"Use query APIs as source of truth instead of assuming last write result",
		},
	}
	response.JSONSuccess(c, out)
}

func (h *Handler) ErrorCodesDoc(c *gin.Context) {
	var out ErrorCodesDoc
	codes := []response.ResponseCode{
		response.CodeInvalidParameter,
		response.CodeUnauthorized,
		response.CodeBadRequest,
		response.CodeForbidden,
		response.CodeNotFound,
		response.CodeConflict,
		response.CodeMethodNotAllowed,
		response.CodeTooManyRequests,
		response.CodeMissingParameter,
		response.CodeBusinessError,
		response.CodeInsufficientQuota,
		response.CodeInsufficientCredits,
		response.CodeInsufficientWallet,
		response.CodeSettlementStateInvalid,
		response.CodeSettlementReverseInvalid,
		response.CodeRouteNotFound,
		response.CodePaymentRequired,
		response.CodeInternalError,
		response.CodeDatabaseError,
		response.CodeThirdPartyError,
		response.CodeServiceUnavailable,
	}
	for _, code := range codes {
		entry := ErrorCodeEntry{
			Code:       code,
			Name:       response.ResponseCodeName[code],
			Message:    response.ResponseMessage[code],
			HTTPStatus: response.GetHTTPStatusCode(code),
		}
		switch {
		case code >= 1000 && code < 2000:
			out.Client = append(out.Client, entry)
		case code >= 2000 && code < 3000:
			out.Business = append(out.Business, entry)
		case code == response.CodePaymentRequired:
			out.Payment = append(out.Payment, entry)
		case code >= 5000:
			out.Server = append(out.Server, entry)
		}
	}
	response.JSONSuccess(c, out)
}
