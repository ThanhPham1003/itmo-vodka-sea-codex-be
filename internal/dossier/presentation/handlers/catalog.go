package handlers

import (
	"github.com/gin-gonic/gin"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	dd "sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/services"
)

func result(c *gin.Context, status int, message string, value any, err error) {
	c.Header("Cache-Control", "no-store")
	if err != nil {
		code, msg := common.HTTPError(err)
		dto.ErrorResponse(c, code, msg)
		return
	}
	dto.SuccessResponse(c, status, message, value)
}
func page(c *gin.Context) (common.Page, bool) {
	p, err := common.ParsePage(c.Query("limit"), c.Query("offset"))
	if err != nil {
		result(c, 400, "", nil, err)
		return p, false
	}
	return p, true
}

type CatalogHandler struct{ s services.ICatalogService }

func NewCatalogHandler(s services.ICatalogService) *CatalogHandler { return &CatalogHandler{s: s} }
func (h *CatalogHandler) ListDocuments(c *gin.Context) {
	p, ok := page(c)
	if !ok {
		return
	}
	out, err := h.s.ListDocumentTypes(c.Request.Context(), p)
	result(c, 200, "Document types", out, err)
}
func (h *CatalogHandler) GetDocument(c *gin.Context) {
	out, err := h.s.GetDocumentType(c.Request.Context(), c.Param("code"))
	result(c, 200, "Document type", out, err)
}
func (h *CatalogHandler) PutDocument(c *gin.Context) {
	var in dd.DocumentTypeRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.PutDocumentType(c.Request.Context(), c.Param("code"), in)
	status := 200
	if c.Param("code") == "" {
		status = 201
	}
	result(c, status, "Document type saved", out, err)
}
func (h *CatalogHandler) ArchiveDocument(c *gin.Context) {
	err := h.s.ArchiveDocumentType(c.Request.Context(), c.Param("code"))
	result(c, 200, "Document type archived", nil, err)
}
func (h *CatalogHandler) ListDossiers(c *gin.Context) {
	p, ok := page(c)
	if !ok {
		return
	}
	out, err := h.s.ListDossierTypes(c.Request.Context(), p)
	result(c, 200, "Dossier types", out, err)
}
func (h *CatalogHandler) GetDossier(c *gin.Context) {
	out, err := h.s.GetDossierType(c.Request.Context(), c.Param("code"))
	result(c, 200, "Dossier type", out, err)
}
func (h *CatalogHandler) PutDossier(c *gin.Context) {
	var in dd.DossierTypeRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.PutDossierType(c.Request.Context(), c.Param("code"), in)
	status := 200
	if c.Param("code") == "" {
		status = 201
	}
	result(c, status, "Dossier type saved", out, err)
}
func (h *CatalogHandler) ArchiveDossier(c *gin.Context) {
	err := h.s.ArchiveDossierType(c.Request.Context(), c.Param("code"))
	result(c, 200, "Dossier type archived", nil, err)
}
func (h *CatalogHandler) SuggestDocument(c *gin.Context) {
	var in dd.SuggestionRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.SuggestDocument(c.Request.Context(), in.Description)
	result(c, 200, "Draft suggestion", out, err)
}
func (h *CatalogHandler) SuggestRules(c *gin.Context) {
	var in dd.SuggestionRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.SuggestRules(c.Request.Context(), c.Param("code"), in.Description)
	result(c, 200, "Draft rules", out, err)
}
func (h *CatalogHandler) SuggestDraftRules(c *gin.Context) {
	var in dd.DraftRuleSuggestionRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.SuggestDraftRules(c.Request.Context(), in)
	result(c, 200, "Draft rules", out, err)
}
func (h *CatalogHandler) SuggestDossier(c *gin.Context) {
	var in struct {
		Description   string   `json:"description" binding:"required,max=4000"`
		DocumentTypes []string `json:"document_types" binding:"max=50"`
	}
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.SuggestDossier(c.Request.Context(), in.Description, in.DocumentTypes)
	result(c, 200, "Draft dossier configuration", out, err)
}
