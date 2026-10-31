package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	ad "sea-codex-server/internal/assistant/dto"
	"sea-codex-server/internal/assistant/logic"
	"sea-codex-server/internal/assistant/services"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
)

type AssistantHandler struct{ service services.IAssistantService }

func NewAssistantHandler(s services.IAssistantService) *AssistantHandler { return &AssistantHandler{s} }
func failure(c *gin.Context, err error) {
	var rate *logic.RateLimitError
	if errors.As(err, &rate) {
		c.Header("Retry-After", strconv.Itoa(rate.RetryAfter))
		dto.ErrorResponse(c, 429, "Assistant request limit exceeded")
		return
	}
	status, message := common.HTTPError(err)
	dto.ErrorResponse(c, status, message)
}
func query(c *gin.Context, allowPage bool) (string, string, bool) {
	for key, values := range c.Request.URL.Query() {
		if len(values) != 1 || (key != "locale" && (!allowPage || key != "page")) {
			dto.ErrorResponse(c, 400, "Invalid query parameters")
			return "", "", false
		}
	}
	locale := c.DefaultQuery("locale", "vi")
	page := c.Query("page")
	if !logic.ValidLocale(locale) || !logic.ValidPage(page) {
		dto.ErrorResponse(c, 400, "Invalid locale or page")
		return "", "", false
	}
	return locale, page, true
}
func (h *AssistantHandler) Context(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	locale, page, ok := query(c, true)
	if !ok {
		return
	}
	out, err := h.service.Context(c.Request.Context(), locale, page)
	if err != nil {
		failure(c, err)
		return
	}
	dto.SuccessResponse(c, 200, "Assistant context", out)
}
func (h *AssistantHandler) Article(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	locale, _, ok := query(c, false)
	if !ok {
		return
	}
	out, err := h.service.Article(c.Request.Context(), c.Param("id"), locale)
	if err != nil {
		failure(c, err)
		return
	}
	dto.SuccessResponse(c, 200, "Assistant article", out)
}
func (h *AssistantHandler) Chat(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024)
	var in ad.ChatRequest
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	err := d.Decode(&in)
	if err == nil {
		err = d.Decode(new(any))
		if err == io.EOF {
			err = nil
		} else if err == nil {
			err = common.ErrInvalidArgument
		}
	}
	if err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			dto.ErrorResponse(c, 413, "Assistant request is too large")
		} else {
			dto.ErrorResponse(c, 400, "Invalid JSON request")
		}
		return
	}
	if binding.Validator.ValidateStruct(in) != nil {
		dto.ErrorResponse(c, 400, "Invalid request fields")
		return
	}
	out, err := h.service.Chat(c.Request.Context(), in)
	if err != nil {
		failure(c, err)
		return
	}
	dto.SuccessResponse(c, 200, "Assistant response", out)
}
