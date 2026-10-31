package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ad "sea-codex-server/internal/audit/dto"
	"sea-codex-server/internal/audit/services"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	"time"
)

func List(service services.IAuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := common.ParsePage(c.Query("limit"), c.Query("offset"))
		if err != nil {
			dto.ErrorResponse(c, 400, "Invalid pagination")
			return
		}
		f := ad.Filter{Page: p, Action: c.Query("action")}
		for key, target := range map[string]**uuid.UUID{"dossier_id": &f.DossierID, "resource_id": &f.ResourceID, "actor_id": &f.ActorID} {
			if raw := c.Query(key); raw != "" {
				id, err := uuid.Parse(raw)
				if err != nil {
					dto.ErrorResponse(c, 400, "Invalid UUID filter")
					return
				}
				*target = &id
			}
		}
		for key, target := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
			if raw := c.Query(key); raw != "" {
				value, err := time.Parse(time.RFC3339, raw)
				if err != nil {
					dto.ErrorResponse(c, 400, "Invalid timestamp filter")
					return
				}
				*target = &value
			}
		}
		if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
			dto.ErrorResponse(c, 400, "Invalid date range")
			return
		}
		out, err := service.List(c.Request.Context(), f)
		if err != nil {
			status, msg := common.HTTPError(err)
			dto.ErrorResponse(c, status, msg)
			return
		}
		c.Header("Cache-Control", "no-store")
		dto.SuccessResponse(c, 200, "Audit events", out)
	}
}
