package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	dd "sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/services"
)

type DossierHandler struct{ s services.IDossierService }

func NewDossierHandler(s services.IDossierService) *DossierHandler { return &DossierHandler{s: s} }
func id(c *gin.Context, name string) (uuid.UUID, bool) {
	value, err := uuid.Parse(c.Param(name))
	if err != nil || value == uuid.Nil {
		dto.ErrorResponse(c, 400, "Invalid resource ID")
		return uuid.Nil, false
	}
	return value, true
}
func (h *DossierHandler) Create(c *gin.Context) {
	var in dd.CreateDossierRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.Create(c.Request.Context(), in)
	result(c, 201, "Dossier created", out, err)
}
func (h *DossierHandler) List(review bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		f := dd.DossierFilter{Page: p, Status: c.Query("status"), Type: c.Query("dossier_type"), Search: c.Query("search")}
		for key, target := range map[string]**uuid.UUID{"creator_id": &f.CreatorID, "assignee_id": &f.AssigneeID} {
			if raw := c.Query(key); raw != "" {
				v, err := uuid.Parse(raw)
				if err != nil {
					dto.ErrorResponse(c, 400, "Invalid user filter")
					return
				}
				*target = &v
			}
		}
		if c.Query("from") != "" || c.Query("to") != "" {
			from, to, _, err := common.DateRange(c.Query("from"), c.Query("to"), c.Query("timezone"))
			if err != nil {
				dto.ErrorResponse(c, 400, "Invalid date range")
				return
			}
			f.HistoryFrom = &from
			f.HistoryTo = &to
		}
		out, err := h.s.List(c.Request.Context(), f, review)
		result(c, 200, "Dossiers", out, err)
	}
}
func (h *DossierHandler) Detail(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	out, err := h.s.Detail(c.Request.Context(), v)
	result(c, 200, "Dossier detail", out, err)
}
func (h *DossierHandler) Assign(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	var in dd.AssignmentRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	err := h.s.Assign(c.Request.Context(), v, in)
	result(c, 200, "Assignment updated", nil, err)
}
func (h *DossierHandler) AddRequirement(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	var in dd.RequirementRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	err := h.s.AddRequirement(c.Request.Context(), v, in)
	result(c, 201, "Requirement added", nil, err)
}
func (h *DossierHandler) WaiveRequirement(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	req, ok := id(c, "requirement_id")
	if !ok {
		return
	}
	var in dd.ReasonRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	err := h.s.WaiveRequirement(c.Request.Context(), v, req, in)
	result(c, 200, "Conditional requirement marked not required", nil, err)
}
func (h *DossierHandler) Correct(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	field, ok := id(c, "field_id")
	if !ok {
		return
	}
	var in dd.CorrectionRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	err := h.s.Correct(c.Request.Context(), v, field, in)
	result(c, 200, "Field corrected and reconciliation refreshed", nil, err)
}
func (h *DossierHandler) Acknowledge(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	issue, ok := id(c, "issue_id")
	if !ok {
		return
	}
	var in dd.ReasonRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	err := h.s.Acknowledge(c.Request.Context(), v, issue, in)
	result(c, 200, "Warning acknowledged", nil, err)
}
func (h *DossierHandler) Review(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	var in dd.ReviewRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.Review(c.Request.Context(), v, in)
	result(c, 200, "Review decision recorded", out, err)
}
func (h *DossierHandler) Reopen(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	var in dd.ReasonRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	err := h.s.Reopen(c.Request.Context(), v, in)
	result(c, 200, "Dossier reopened", nil, err)
}
func (h *DossierHandler) Reveal(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	var in dd.RevealRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.Reveal(c.Request.Context(), v, in)
	result(c, 200, "Sensitive fields revealed and audited", out, err)
}
func (h *DossierHandler) Dashboard(c *gin.Context) {
	out, err := h.s.Dashboard(c.Request.Context(), c.Query("from"), c.Query("to"), c.Query("timezone"), c.Query("metric"), c.Query("dossier_type"))
	result(c, 200, "Dossier status metrics", out, err)
}
