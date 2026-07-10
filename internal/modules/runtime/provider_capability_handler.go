package runtime

import (
	"errors"
	"net/http"

	"platform-service/internal/telemetry"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

// ProviderBalance godoc
// @Summary Query runtime provider balance
// @Description Proxy a balance query without exposing provider credentials.
// @Tags internal-runtime
// @Produce json
// @Param providerCode path string true "Runtime provider code"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/runtime/providers/{providerCode}/balance [get]
func (h *Handler) ProviderBalance(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.provider.balance")
	defer span.End()
	result, err := h.service.ProviderBalance(c.Request.Context(), c.Param("providerCode"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to query runtime provider balance", "RUNTIME_PROVIDER_BALANCE_FAILED", "Check provider configuration and upstream provider status.")
		return
	}
	response.JSONSuccess(c, result)
}

// ProviderTTSVoices godoc
// @Summary List runtime provider TTS voices
// @Tags internal-runtime
// @Produce json
// @Param providerCode path string true "Runtime provider code"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/runtime/providers/{providerCode}/tts-voices [get]
func (h *Handler) ProviderTTSVoices(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.provider.tts_voices")
	defer span.End()
	result, err := h.service.ProviderTTSVoices(c.Request.Context(), c.Param("providerCode"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to query runtime provider voices", "RUNTIME_PROVIDER_VOICES_FAILED", "Check provider configuration and upstream provider status.")
		return
	}
	response.JSONSuccess(c, result)
}

// ProviderUploadImage godoc
// @Summary Upload an image to a runtime provider
// @Tags internal-runtime
// @Accept multipart/form-data
// @Produce json
// @Param providerCode path string true "Runtime provider code"
// @Param image formData file true "Image file"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/runtime/providers/{providerCode}/image-upload [post]
func (h *Handler) ProviderUploadImage(c *gin.Context) { h.providerUploadFile(c, "image") }

// ProviderUploadMedia godoc
// @Summary Upload media to a runtime provider
// @Tags internal-runtime
// @Accept multipart/form-data
// @Produce json
// @Param providerCode path string true "Runtime provider code"
// @Param file formData file true "Video or audio file"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/runtime/providers/{providerCode}/media-upload [post]
func (h *Handler) ProviderUploadMedia(c *gin.Context) { h.providerUploadFile(c, "media") }

func (h *Handler) providerUploadFile(c *gin.Context, kind string) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.provider.upload")
	defer span.End()
	file, err := c.FormFile(kind)
	if err != nil {
		file, err = c.FormFile("file")
	}
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			response.JSONErrorWithStatus(c, response.CodeInvalidParameter, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		response.JSONError(c, response.CodeInvalidParameter, "file is required")
		return
	}
	src, err := file.Open()
	if err != nil {
		response.JSONError(c, response.CodeInvalidParameter, "failed to open upload file")
		return
	}
	defer src.Close()
	result, err := h.service.ProviderUploadFile(c.Request.Context(), c.Param("providerCode"), ProviderUploadFileInput{Kind: kind, Filename: file.Filename, Reader: src})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to upload runtime provider file", "RUNTIME_PROVIDER_UPLOAD_FAILED", "Check provider configuration, file size, and upstream provider status.")
		return
	}
	response.JSONSuccessWithStatus(c, http.StatusCreated, result)
}

// ProviderUploadURL godoc
// @Summary Import a media URL into a runtime provider
// @Tags internal-runtime
// @Accept json
// @Produce json
// @Param providerCode path string true "Runtime provider code"
// @Param request body ProviderUploadURLInput true "Remote media URL"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/runtime/providers/{providerCode}/media-upload-url [post]
func (h *Handler) ProviderUploadURL(c *gin.Context) {
	var req ProviderUploadURLInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid provider upload-url request")
		return
	}
	result, err := h.service.ProviderUploadURL(c.Request.Context(), c.Param("providerCode"), req.FileURL)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to upload runtime provider url", "RUNTIME_PROVIDER_UPLOAD_URL_FAILED", "Check provider configuration, URL accessibility, and upstream provider status.")
		return
	}
	response.JSONSuccessWithStatus(c, http.StatusCreated, result)
}

// ProviderAction godoc
// @Summary Invoke a provider utility action
// @Tags internal-runtime
// @Accept json
// @Produce json
// @Param providerCode path string true "Runtime provider code"
// @Param action path string true "Provider action"
// @Param request body ProviderActionInput false "Action payload"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/runtime/providers/{providerCode}/actions/{action} [post]
func (h *Handler) ProviderAction(c *gin.Context) {
	var req ProviderActionInput
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.JSONBindError(c, err, "invalid provider action request")
			return
		}
	}
	result, err := h.service.ProviderAction(c.Request.Context(), c.Param("providerCode"), c.Param("action"), req.Payload)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to invoke runtime provider action", "RUNTIME_PROVIDER_ACTION_FAILED", "Check provider action, payload, and upstream provider status.")
		return
	}
	response.JSONSuccess(c, result)
}
