package bootstrap_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/internal/bootstrap"
	dd "sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
)

func TestMixedPDFProcessingRetainsPageSources(t *testing.T) {
	for _, name := range []string{"pdfinfo", "pdftoppm", "pdfseparate", "pdfunite"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip("full Poppler required; Docker and CI install poppler-utils")
		}
	}
	provider := fakeVision(t)
	defer provider.Close()
	app := newTestApp(t)
	app.Config.Processing.Enabled = true
	app.Config.Processing.BaseURL = provider.URL
	app.Config.Processing.APIKey = "fixture"
	app.Config.Processing.Model = "fixture"
	router, err := bootstrap.NewRouter(app)
	require.NoError(t, err)
	_, token := signupAndLogin(t, router, "pdf-company")
	documents := []models.DocumentType{}
	requirements := []models.DocumentRequirement{}
	for _, name := range []string{"Identity", "Contract"} {
		doc := data[models.DocumentType](t, request(t, router, "POST", "/api/v1/document-types", token, dd.DocumentTypeRequest{DocumentDefinition: models.DocumentDefinition{Name: name, Fields: []models.FieldSchema{{Label: "ID", Type: "identifier", Required: true, PII: true}}}}), 201)
		documents = append(documents, doc)
		requirements = append(requirements, models.DocumentRequirement{DocumentType: doc.Code, Required: true})
	}
	typ := data[models.DossierType](t, request(t, router, "POST", "/api/v1/dossier-types", token, dd.DossierTypeRequest{DossierDefinition: models.DossierDefinition{Name: "Mixed PDF", Documents: requirements}}), 201)
	d := data[models.Dossier](t, request(t, router, "POST", "/api/v1/dossiers", token, dd.CreateDossierRequest{Title: "Synthetic mixed PDF", DossierType: typ.Code}), 201)
	base := "/api/v1/dossiers/" + d.ID.String()
	contents, err := os.ReadFile("testdata/synthetic-two-documents.pdf")
	require.NoError(t, err)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "synthetic.pdf")
	require.NoError(t, err)
	_, err = part.Write(contents)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest("POST", base+"/files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	uploaded := data[dd.UploadResult](t, response, 200)
	require.Equal(t, "UPLOADED", uploaded.Files[0].Status)
	source := *uploaded.Files[0].ID
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = app.Workers[0].Run(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("shutdown timeout")
		}
	}()
	var detail dd.Detail
	require.Eventually(t, func() bool {
		detail = data[dd.Detail](t, request(t, router, "GET", base, token, nil), 200)
		return detail.Dossier.Status == models.Valid
	}, 10*time.Second, 20*time.Millisecond)
	require.Len(t, detail.Files, 3)
	require.Len(t, detail.Groups, 2)
	require.Len(t, detail.Fields, 2)
	derived := map[int]models.File{}
	for _, f := range detail.Files {
		if f.SourceFileID != nil {
			assert.Equal(t, source, *f.SourceFileID)
			require.Len(t, f.Pages, 1)
			derived[f.Pages[0]] = f
		}
	}
	require.Len(t, derived, 2)
	for _, field := range detail.Fields {
		assert.Equal(t, source, field.FileID, "field offsets refer to original PDF")
		assert.Contains(t, []int{1, 2}, field.Page)
		for _, group := range detail.Groups {
			if group.ID == field.GroupID {
				assert.Equal(t, derived[field.Page].ID, group.FileID)
			}
		}
	}
}
