package bootstrap_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/internal/bootstrap"
	dd "sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
)

func TestApprovalAcceptsRequiredFieldsAfterHumanAcknowledgement(t *testing.T) {
	app := newTestApp(t)
	var logs bytes.Buffer
	app.Logger = zerolog.New(&logs)
	router, err := bootstrap.NewRouter(app)
	require.NoError(t, err)
	admin, token := signupAndLogin(t, router, "review-readiness")
	document := data[models.DocumentType](t, request(t, router, "POST", "/api/v1/document-types", token, dd.DocumentTypeRequest{DocumentDefinition: models.DocumentDefinition{
		Name: "Contract", Fields: []models.FieldSchema{{Label: "Signing date", Type: "date", Required: true}, {Label: "Identity number", Type: "identifier", Required: true}},
	}}), 201)
	typ := data[models.DossierType](t, request(t, router, "POST", "/api/v1/dossier-types", token, dd.DossierTypeRequest{DossierDefinition: models.DossierDefinition{Name: "Review", Documents: []models.DocumentRequirement{{DocumentType: document.Code, Required: true}}}}), 201)
	dossier := data[models.Dossier](t, request(t, router, "POST", "/api/v1/dossiers", token, dd.CreateDossierRequest{Title: "Required fields", DossierType: typ.Code}), 201)
	base := "/api/v1/dossiers/" + dossier.ID.String()
	file := models.File{CompanyID: admin.CompanyID, DossierID: dossier.ID, BatchID: uuid.New(), OriginalName: "synthetic.pdf", StorageKey: "not-read", SHA256: "synthetic", MIME: "application/pdf", DocumentType: document.Code, Status: "EXTRACTED"}
	require.NoError(t, app.DB.Create(&file).Error)
	group := models.Group{CompanyID: admin.CompanyID, DossierID: dossier.ID, FileID: file.ID, DocumentType: document.Code, Pages: []int{1}, Confidence: 1, Confirmed: true}
	require.NoError(t, app.DB.Create(&group).Error)
	fields := []models.ExtractedField{}
	for i, field := range document.Definition.Fields {
		value, validation := "", "MISSING"
		if i == 1 {
			value, validation = "private-invalid-value", "INVALID_FORMAT"
		}
		fields = append(fields, models.ExtractedField{CompanyID: admin.CompanyID, DossierID: dossier.ID, FileID: file.ID, GroupID: group.ID, Key: field.Key, Label: field.Label, Type: field.Type, Required: true, Value: value, Validation: validation, Confidence: 1, Page: 1, SchemaVersion: 1})
	}
	require.NoError(t, app.DB.Create(&fields).Error)
	require.Equal(t, 200, request(t, router, "PATCH", base+"/assignment", token, dd.AssignmentRequest{ExpectedVersion: dossier.Version, AssigneeID: &admin.ID}).Code)
	get := func() dd.Detail { return data[dd.Detail](t, request(t, router, "GET", base, token, nil), 200) }
	detail := get()
	require.Equal(t, models.Valid, detail.Dossier.Status)
	require.Len(t, detail.Issues, 2)
	logs.Reset()
	remaining := 2
	for _, issue := range detail.Issues {
		rejected := request(t, router, "POST", base+"/decisions", token, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE", Reason: "private-review-reason"})
		require.Equal(t, 409, rejected.Code, "each outstanding field must be reviewed")
		assert.Equal(t, detail.Dossier.Version, get().Dossier.Version)
		require.Equal(t, 200, request(t, router, "POST", base+"/issues/"+issue.ID.String()+"/acknowledge", token, dd.ReasonRequest{ExpectedVersion: detail.Dossier.Version, Reason: "Checked original and accepted"}).Code)
		detail = get()
		remaining--
	}
	require.Zero(t, remaining)
	for _, issue := range detail.Issues {
		require.Equal(t, "ACKNOWLEDGED", issue.Status)
	}

	rejections := 0
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		if entry["message"] != "dossier decision rejected" {
			continue
		}
		rejections++
		assert.Equal(t, "invalid_required_fields", entry["reason"])
		assert.Equal(t, float64(3-rejections), entry["invalid_required_fields"])
		assert.Equal(t, entry["expected_version"], entry["current_version"])
		assert.NotEmpty(t, entry["request_id"])
	}
	assert.Equal(t, 2, rejections)
	assert.NotContains(t, logs.String(), "private-invalid-value")
	assert.NotContains(t, logs.String(), "private-review-reason")
	var decisions int64
	require.NoError(t, app.DB.Model(&models.Decision{}).Where("dossier_id = ?", dossier.ID).Count(&decisions).Error)
	assert.Zero(t, decisions)

	decision := data[models.Decision](t, request(t, router, "POST", base+"/decisions", token, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE"}), 200)
	assert.Equal(t, "APPROVE", decision.Action)
	assert.Equal(t, models.Approved, get().Dossier.Status)
	var stored models.Decision
	require.NoError(t, app.DB.First(&stored, "id = ?", decision.ID).Error)
	var snapshot struct {
		Fields []models.ExtractedField
		Issues []models.Issue
	}
	require.NoError(t, json.Unmarshal([]byte(stored.Snapshot), &snapshot))
	require.Len(t, snapshot.Fields, 2)
	for _, field := range snapshot.Fields {
		assert.NotEqual(t, "VALID", field.Validation)
	}
	for _, issue := range snapshot.Issues {
		assert.Equal(t, "ACKNOWLEDGED", issue.Status)
		require.NotNil(t, issue.AcknowledgedBy)
		assert.Equal(t, admin.ID, *issue.AcknowledgedBy)
		assert.Equal(t, "Checked original and accepted", issue.Acknowledgement)
	}
	var event struct{ Metadata string }
	require.NoError(t, app.DB.Table("audit_logs").Where("action = ? AND resource_id = ?", "DOSSIER_APPROVE", dossier.ID).First(&event).Error)
	assert.Contains(t, event.Metadata, "accepted_required_fields")

	// A new processing round with changed evidence invalidates the old acceptance.
	detail = get()
	require.Equal(t, 200, request(t, router, "POST", base+"/reopen", token, dd.ReasonRequest{ExpectedVersion: detail.Dossier.Version, Reason: "New document scan"}).Code)
	detail = get()
	require.NoError(t, app.DB.Model(&models.ExtractedField{}).Where("id = ?", fields[0].ID).Updates(map[string]any{"value": "2026-99-99", "validation": "INVALID_FORMAT"}).Error)
	require.Equal(t, 200, request(t, router, "PATCH", base+"/assignment", token, dd.AssignmentRequest{ExpectedVersion: detail.Dossier.Version, AssigneeID: &admin.ID}).Code)
	detail = get()
	var changed models.Issue
	for _, issue := range detail.Issues {
		if issue.RuleCode == "field:"+fields[0].ID.String() {
			changed = issue
		}
	}
	require.Equal(t, "OPEN", changed.Status)
	assert.Nil(t, changed.AcknowledgedBy)
	require.Equal(t, 409, request(t, router, "POST", base+"/decisions", token, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE"}).Code)
	require.Equal(t, 200, request(t, router, "POST", base+"/issues/"+changed.ID.String()+"/acknowledge", token, dd.ReasonRequest{ExpectedVersion: detail.Dossier.Version, Reason: "Checked changed document and accepted"}).Code)
	detail = get()
	require.Equal(t, 200, request(t, router, "POST", base+"/decisions", token, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE"}).Code)
}
