package handlers

import (
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	dd "sea-codex-server/internal/dossier/dto"
)

func (h *DossierHandler) Upload(app *common.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := id(c, "id")
		if !ok {
			return
		}
		cfg := app.Config.Uploads
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxFileSizeMB*1024*1024*int64(cfg.MaxFilesPerRequest)+2*1024*1024)
		defer func() {
			if c.Request.MultipartForm != nil {
				_ = c.Request.MultipartForm.RemoveAll()
			}
		}()
		if err := c.Request.ParseMultipartForm(8 * 1024 * 1024); err != nil {
			dto.ErrorResponse(c, 400, "Invalid or oversized multipart upload")
			return
		}
		files := c.Request.MultipartForm.File["files"]
		types := c.Request.MultipartForm.Value["document_types"]
		roles := c.Request.MultipartForm.Value["roles"]
		replacements := c.Request.MultipartForm.Value["replaces_file_ids"]
		if len(files) == 0 || len(files) > cfg.MaxFilesPerRequest {
			dto.ErrorResponse(c, 400, "Invalid file count")
			return
		}
		for _, values := range [][]string{types, roles, replacements} {
			if len(values) != 0 && len(values) != len(files) {
				dto.ErrorResponse(c, 400, "File metadata must match file count")
				return
			}
		}
		inputs := []dd.UploadInput{}
		for index, header := range files {
			input := dd.UploadInput{Name: header.Filename, Open: func() (io.ReadCloser, error) { return header.Open() }}
			if len(types) > 0 {
				input.DocumentType = types[index]
			}
			if len(roles) > 0 {
				input.Role = roles[index]
			}
			if len(replacements) > 0 && replacements[index] != "" {
				replacement, err := uuid.Parse(replacements[index])
				if err != nil {
					dto.ErrorResponse(c, 400, "Invalid replacement file ID")
					return
				}
				input.ReplacesFileID = &replacement
			}
			inputs = append(inputs, input)
		}
		out, err := h.s.Upload(c.Request.Context(), v, inputs)
		result(c, 200, "Per-file upload results", out, err)
	}
}
func (h *DossierHandler) Retry(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	file, ok := id(c, "file_id")
	if !ok {
		return
	}
	var in dd.VersionRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	err := h.s.Retry(c.Request.Context(), v, file, in.ExpectedVersion)
	result(c, 202, "File queued for retry", nil, err)
}
func (h *DossierHandler) Classify(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	file, ok := id(c, "file_id")
	if !ok {
		return
	}
	var in dd.ClassificationRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	err := h.s.Classify(c.Request.Context(), v, file, in)
	result(c, 202, "Classification confirmed and extraction queued", nil, err)
}
func (h *DossierHandler) Content(c *gin.Context) {
	v, ok := id(c, "id")
	if !ok {
		return
	}
	file, ok := id(c, "file_id")
	if !ok {
		return
	}
	out, err := h.s.FileContent(c.Request.Context(), v, file)
	if err != nil {
		result(c, 200, "", nil, err)
		return
	}
	defer out.File.Close()
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": out.Name}))
	c.Header("Content-Type", out.MIME)
	info, err := out.File.Stat()
	if err != nil {
		dto.ErrorResponse(c, 503, "File unavailable")
		return
	}
	http.ServeContent(c.Writer, c.Request, out.Name, info.ModTime(), out.File)
}
