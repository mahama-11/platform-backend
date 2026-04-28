package response

import (
	"fmt"
	"net/http"
	"time"

	"platform-service/pkg/platformconst"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const (
	contextResponseCodeKey         = "responseCode"
	contextResponseErrorCodeKey    = "responseErrorCode"
	contextResponseErrorHintKey    = "responseErrorHint"
	contextResponseErrorMessageKey = "responseErrorMessage"
)

type ResponseCode int

const (
	CodeSuccess ResponseCode = 0

	CodeInvalidParameter ResponseCode = 1000
	CodeUnauthorized     ResponseCode = 1001
	CodeBadRequest       ResponseCode = 1002
	CodeForbidden        ResponseCode = 1003
	CodeNotFound         ResponseCode = 1004
	CodeConflict         ResponseCode = 1005
	CodeMethodNotAllowed ResponseCode = 1006
	CodeTooManyRequests  ResponseCode = 1007
	CodeMissingParameter ResponseCode = 1008

	CodeBusinessError            ResponseCode = 2000
	CodeInsufficientQuota        ResponseCode = 2001
	CodeInsufficientCredits      ResponseCode = 2002
	CodeInsufficientWallet       ResponseCode = 2003
	CodeSettlementStateInvalid   ResponseCode = 2004
	CodeSettlementReverseInvalid ResponseCode = 2005
	CodeRouteNotFound            ResponseCode = 2006

	CodePaymentRequired ResponseCode = 40201

	CodeInternalError      ResponseCode = 5000
	CodeDatabaseError      ResponseCode = 5001
	CodeThirdPartyError    ResponseCode = 5002
	CodeServiceUnavailable ResponseCode = 5003
)

var ResponseMessage = map[ResponseCode]string{
	CodeSuccess:                  "success",
	CodeInvalidParameter:         "Invalid parameter",
	CodeUnauthorized:             "Unauthorized",
	CodeBadRequest:               "Invalid request parameters",
	CodeForbidden:                "Forbidden",
	CodeNotFound:                 "Resource not found",
	CodeConflict:                 "Resource conflict",
	CodeMethodNotAllowed:         "Method not allowed",
	CodeTooManyRequests:          "Too many requests",
	CodeMissingParameter:         "Missing required parameter",
	CodeBusinessError:            "Business operation failed",
	CodeInsufficientQuota:        "Insufficient quota balance",
	CodeInsufficientCredits:      "Insufficient credits balance",
	CodeInsufficientWallet:       "Insufficient wallet balance",
	CodeSettlementStateInvalid:   "Settlement state invalid",
	CodeSettlementReverseInvalid: "Settlement reverse not allowed",
	CodeRouteNotFound:            "Commercial route not found",
	CodePaymentRequired:          "Payment required",
	CodeInternalError:            "Internal server error",
	CodeDatabaseError:            "Database operation failed",
	CodeThirdPartyError:          "Third party service error",
	CodeServiceUnavailable:       "Service temporarily unavailable",
}

var ResponseCodeName = map[ResponseCode]string{
	CodeSuccess:                  "CodeSuccess",
	CodeInvalidParameter:         "CodeInvalidParameter",
	CodeUnauthorized:             "CodeUnauthorized",
	CodeBadRequest:               "CodeBadRequest",
	CodeForbidden:                "CodeForbidden",
	CodeNotFound:                 "CodeNotFound",
	CodeConflict:                 "CodeConflict",
	CodeMethodNotAllowed:         "CodeMethodNotAllowed",
	CodeTooManyRequests:          "CodeTooManyRequests",
	CodeMissingParameter:         "CodeMissingParameter",
	CodeBusinessError:            "CodeBusinessError",
	CodeInsufficientQuota:        "CodeInsufficientQuota",
	CodeInsufficientCredits:      "CodeInsufficientCredits",
	CodeInsufficientWallet:       "CodeInsufficientWallet",
	CodeSettlementStateInvalid:   "CodeSettlementStateInvalid",
	CodeSettlementReverseInvalid: "CodeSettlementReverseInvalid",
	CodeRouteNotFound:            "CodeRouteNotFound",
	CodePaymentRequired:          "CodePaymentRequired",
	CodeInternalError:            "CodeInternalError",
	CodeDatabaseError:            "CodeDatabaseError",
	CodeThirdPartyError:          "CodeThirdPartyError",
	CodeServiceUnavailable:       "CodeServiceUnavailable",
}

type BaseResponse struct {
	Code      ResponseCode `json:"code"`
	Message   string       `json:"message"`
	Timestamp int64        `json:"timestamp"`
	RequestID string       `json:"request_id,omitempty"`
}

type SuccessResponse struct {
	BaseResponse
	Data any `json:"data,omitempty"`
}

type ErrorResponse struct {
	BaseResponse
	Error     string       `json:"error,omitempty"`
	ErrorCode string       `json:"error_code,omitempty"`
	ErrorHint string       `json:"error_hint,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

type Pagination struct {
	Page      int `json:"page"`
	PageSize  int `json:"page_size"`
	Total     int `json:"total"`
	TotalPage int `json:"total_page"`
}

type PaginatedResponse struct {
	BaseResponse
	Data       any        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

func NewSuccessResponse(data any) *SuccessResponse {
	return &SuccessResponse{
		BaseResponse: BaseResponse{
			Code:    CodeSuccess,
			Message: ResponseMessage[CodeSuccess],
		},
		Data: data,
	}
}

func NewErrorResponse(code ResponseCode, err string) *ErrorResponse {
	return &ErrorResponse{
		BaseResponse: BaseResponse{
			Code:    code,
			Message: ResponseMessage[code],
		},
		Error: err,
	}
}

func NewSemanticErrorResponse(code ResponseCode, err, errorCode, errorHint string) *ErrorResponse {
	resp := NewErrorResponse(code, err)
	resp.ErrorCode = errorCode
	resp.ErrorHint = errorHint
	return resp
}

func NewErrorResponseWithFields(code ResponseCode, err string, fieldErrors []FieldError) *ErrorResponse {
	return &ErrorResponse{
		BaseResponse: BaseResponse{
			Code:    code,
			Message: ResponseMessage[code],
		},
		Error:  err,
		Errors: fieldErrors,
	}
}

func NewPaginatedResponse(data any, page, pageSize, total int) *PaginatedResponse {
	totalPage := 0
	if pageSize > 0 {
		totalPage = (total + pageSize - 1) / pageSize
	}
	return &PaginatedResponse{
		BaseResponse: BaseResponse{
			Code:    CodeSuccess,
			Message: ResponseMessage[CodeSuccess],
		},
		Data: data,
		Pagination: Pagination{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	}
}

func (r *BaseResponse) SetRequestInfo(c *gin.Context) {
	r.Timestamp = time.Now().UnixMilli()
	r.RequestID = c.GetString(platformconst.CtxRequestID)
}

func JSONSuccess(c *gin.Context, data any) {
	resp := NewSuccessResponse(data)
	resp.SetRequestInfo(c)
	attachResponseMeta(c, resp.Code, "", "", "")
	c.JSON(http.StatusOK, resp)
}

func JSONSuccessWithStatus(c *gin.Context, status int, data any) {
	resp := NewSuccessResponse(data)
	resp.SetRequestInfo(c)
	attachResponseMeta(c, resp.Code, "", "", "")
	c.JSON(status, resp)
}

func JSONPaginated(c *gin.Context, data any, page, pageSize, total int) {
	resp := NewPaginatedResponse(data, page, pageSize, total)
	resp.SetRequestInfo(c)
	attachResponseMeta(c, resp.Code, "", "", "")
	c.JSON(http.StatusOK, resp)
}

func JSONError(c *gin.Context, code ResponseCode, message string) {
	resp := NewErrorResponse(code, message)
	resp.SetRequestInfo(c)
	attachResponseMeta(c, resp.Code, "", "", message)
	c.JSON(GetHTTPStatusCode(code), resp)
}

func WriteObservedError(c *gin.Context, err error, code ResponseCode, message string) {
	ObserveError(c, err)
	JSONError(c, code, message)
}

func JSONErrorSemantic(c *gin.Context, code ResponseCode, message, errorCode, errorHint string) {
	resp := NewSemanticErrorResponse(code, message, errorCode, errorHint)
	resp.SetRequestInfo(c)
	attachResponseMeta(c, resp.Code, errorCode, errorHint, message)
	c.JSON(GetHTTPStatusCode(code), resp)
}

func WriteObservedSemanticError(c *gin.Context, err error, code ResponseCode, message, errorCode, errorHint string) {
	ObserveError(c, err)
	JSONErrorSemantic(c, code, message, errorCode, errorHint)
}

func JSONErrorWithFields(c *gin.Context, code ResponseCode, message string, fieldErrors []FieldError) {
	resp := NewErrorResponseWithFields(code, message, fieldErrors)
	resp.SetRequestInfo(c)
	attachResponseMeta(c, resp.Code, "", "", message)
	c.JSON(GetHTTPStatusCode(code), resp)
}

func JSONErrorWithStatus(c *gin.Context, code ResponseCode, message string, status int) {
	resp := NewErrorResponse(code, message)
	resp.SetRequestInfo(c)
	attachResponseMeta(c, resp.Code, "", "", message)
	c.JSON(status, resp)
}

func JSONErrorWithStatusSemantic(c *gin.Context, code ResponseCode, message, errorCode, errorHint string, status int) {
	resp := NewSemanticErrorResponse(code, message, errorCode, errorHint)
	resp.SetRequestInfo(c)
	attachResponseMeta(c, resp.Code, errorCode, errorHint, message)
	c.JSON(status, resp)
}

func WriteObservedStatusSemanticError(c *gin.Context, err error, code ResponseCode, message, errorCode, errorHint string, status int) {
	ObserveError(c, err)
	JSONErrorWithStatusSemantic(c, code, message, errorCode, errorHint, status)
}

func JSONBindError(c *gin.Context, err error, fallback string) {
	ObserveError(c, err)
	var validationErrs validator.ValidationErrors
	if ok := errorAs(err, &validationErrs); ok {
		fields := make([]FieldError, 0, len(validationErrs))
		for _, fieldErr := range validationErrs {
			fields = append(fields, FieldError{
				Field:   fieldErr.Field(),
				Message: validationMessage(fieldErr),
				Value:   fmt.Sprintf("%v", fieldErr.Value()),
			})
		}
		JSONErrorWithFields(c, CodeInvalidParameter, fallback, fields)
		return
	}
	JSONError(c, CodeInvalidParameter, fallback)
}

func ObserveError(c *gin.Context, err error) {
	if c == nil || err == nil {
		return
	}
	_ = c.Error(err)
}

func attachResponseMeta(c *gin.Context, code ResponseCode, errorCode, errorHint, errorMessage string) {
	if c == nil {
		return
	}
	c.Set(contextResponseCodeKey, int(code))
	c.Set(contextResponseErrorCodeKey, errorCode)
	c.Set(contextResponseErrorHintKey, errorHint)
	c.Set(contextResponseErrorMessageKey, errorMessage)
}

func ResponseMeta(c *gin.Context) (int, string, string, string) {
	if c == nil {
		return 0, "", "", ""
	}
	code, _ := c.Get(contextResponseCodeKey)
	errorCode, _ := c.Get(contextResponseErrorCodeKey)
	errorHint, _ := c.Get(contextResponseErrorHintKey)
	errorMessage, _ := c.Get(contextResponseErrorMessageKey)
	responseCode, _ := code.(int)
	responseErrorCode, _ := errorCode.(string)
	responseErrorHint, _ := errorHint.(string)
	responseErrorMessage, _ := errorMessage.(string)
	return responseCode, responseErrorCode, responseErrorHint, responseErrorMessage
}

func GetHTTPStatusCode(code ResponseCode) int {
	switch code {
	case CodeSuccess:
		return http.StatusOK
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case CodeTooManyRequests:
		return http.StatusTooManyRequests
	case CodePaymentRequired:
		return http.StatusPaymentRequired
	case CodeInternalError, CodeDatabaseError, CodeThirdPartyError, CodeServiceUnavailable:
		return http.StatusInternalServerError
	case CodeInsufficientQuota, CodeInsufficientCredits, CodeInsufficientWallet, CodeSettlementStateInvalid, CodeSettlementReverseInvalid:
		return http.StatusConflict
	case CodeRouteNotFound:
		return http.StatusNotFound
	default:
		if code >= 1000 && code < 2000 {
			return http.StatusBadRequest
		}
		if code >= 2000 && code < 3000 {
			return http.StatusBadRequest
		}
		if code >= 5000 {
			return http.StatusInternalServerError
		}
		return http.StatusBadRequest
	}
}

func validationMessage(fieldErr validator.FieldError) string {
	switch fieldErr.Tag() {
	case "required":
		return "field is required"
	case "oneof":
		return "field must be one of allowed values"
	case "min":
		return "field value is below minimum"
	case "max":
		return "field value exceeds maximum"
	default:
		return "field validation failed"
	}
}

func errorAs(err error, target any) bool {
	switch t := target.(type) {
	case *validator.ValidationErrors:
		v, ok := err.(validator.ValidationErrors)
		if ok {
			*t = v
			return true
		}
	}
	return false
}
