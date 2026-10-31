package bootstrap_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	ad "sea-codex-server/internal/auth/dto"
	"sea-codex-server/internal/bootstrap"
	"sea-codex-server/internal/common"
	dd "sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
)

func TestCatalogSnapshotsWarningsAndAssignment(t *testing.T) {
	provider := fakeVision(t)
	defer provider.Close()
	app := newTestApp(t)
	app.Config.Processing.Enabled = true
	app.Config.Processing.BaseURL = provider.URL
	app.Config.Processing.APIKey = "fixture-key"
	app.Config.Processing.Model = "fixture-model"
	router, err := bootstrap.NewRouter(app)
	require.NoError(t, err)
	_, admin := signupAndLogin(t, router, "lifecycle-company")
	member := func(name, role string) (ad.UserResponse, string) {
		u := data[ad.UserResponse](t, request(t, router, "POST", "/api/v1/auth/users", admin, ad.CreateUserRequest{Username: name, Email: name + "@example.test", Password: "a-valid-test-password", FullName: name, Role: role}), 201)
		session := data[ad.LoginResponse](t, request(t, router, "POST", "/api/v1/auth/login", "", ad.LoginRequest{UsernameOrEmail: name, Password: "a-valid-test-password"}), 200)
		return u, session.AccessToken
	}
	legal, legalToken := member("legal-lifecycle", "legal_specialist")
	_, broker := member("broker-lifecycle", "broker")
	document := data[models.DocumentType](t, request(t, router, "POST", "/api/v1/document-types", admin, dd.DocumentTypeRequest{DocumentDefinition: models.DocumentDefinition{Name: "Identity", Fields: []models.FieldSchema{{Label: "ID", Type: "identifier", Required: true, PII: true}}, SpecialRequirements: []models.SpecialRequirement{{Description: "Verify registry separately", Severity: "BLOCKER", ExternalSource: "manual registry check"}}}}), 201)
	typ := data[models.DossierType](t, request(t, router, "POST", "/api/v1/dossier-types", admin, dd.DossierTypeRequest{DossierDefinition: models.DossierDefinition{Name: "Lifecycle", Documents: []models.DocumentRequirement{{DocumentType: document.Code, Required: true}}}}), 201)
	d := data[models.Dossier](t, request(t, router, "POST", "/api/v1/dossiers", legalToken, dd.CreateDossierRequest{Title: "Pinned schema", DossierType: typ.Code}), 201)
	base := "/api/v1/dossiers/" + d.ID.String()
	assert.Equal(t, 404, request(t, router, "GET", base, broker, nil).Code, "unassigned broker in the SAME company must not read the dossier")
	updated := document.Definition
	updated.Name = "Identity schema v2"
	doc2 := data[models.DocumentType](t, request(t, router, "PUT", "/api/v1/document-types/"+document.Code, admin, dd.DocumentTypeRequest{DocumentDefinition: updated, ExpectedVersion: 1}), 200)
	assert.Equal(t, 2, doc2.CurrentVersion)
	assert.Equal(t, document.Code, doc2.Code)
	assert.Equal(t, document.Definition.Fields[0].Key, doc2.Definition.Fields[0].Key)
	assert.Equal(t, 409, request(t, router, "PUT", "/api/v1/document-types/"+document.Code, admin, dd.DocumentTypeRequest{DocumentDefinition: updated, ExpectedVersion: 1}).Code)
	var detail dd.Detail
	get := func() { detail = data[dd.Detail](t, request(t, router, "GET", base, admin, nil), 200) }
	get()
	require.Equal(t, 1, detail.Dossier.Snapshot.Documents[0].Version)
	require.Equal(t, "Identity", detail.Dossier.Snapshot.Documents[0].Definition.Name)
	assert.Equal(t, 409, request(t, router, "POST", base+"/requirements", legalToken, dd.RequirementRequest{ExpectedVersion: detail.Dossier.Version, DocumentType: document.Code, Reason: "Cannot duplicate a mandatory requirement"}).Code)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = app.Workers[0].Run(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("worker shutdown timeout")
		}
	}()
	data[dd.UploadResult](t, upload(t, router, base+"/files", legalToken, document.Code, 60), 200)
	require.Eventually(t, func() bool { get(); return detail.Dossier.Status == models.Valid && len(detail.Issues) > 0 }, 5*time.Second, 20*time.Millisecond)
	require.Len(t, detail.Fields, 1)
	assert.Equal(t, 1, detail.Fields[0].SchemaVersion)
	assert.Equal(t, 409, request(t, router, "POST", base+"/decisions", admin, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE"}).Code, "external evidence is a warning requiring human confirmation")
	assert.Equal(t, 403, request(t, router, "POST", base+"/decisions", legalToken, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE"}).Code, "reviewer must be assigned")
	assert.Equal(t, 200, request(t, router, "PATCH", base+"/assignment", admin, dd.AssignmentRequest{ExpectedVersion: detail.Dossier.Version, AssigneeID: &legal.ID}).Code)
	get()
	assert.Equal(t, 400, request(t, router, "POST", base+"/decisions", legalToken, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "REQUEST_CHANGES"}).Code)
	data[models.Decision](t, request(t, router, "POST", base+"/decisions", legalToken, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "REQUEST_CHANGES", Reason: "Verify original document"}), 200)
	get()
	assert.Equal(t, models.Invalid, detail.Dossier.Status)
	// Changing assignment cannot erase a request for changes.
	assert.Equal(t, 200, request(t, router, "PATCH", base+"/assignment", admin, dd.AssignmentRequest{ExpectedVersion: detail.Dossier.Version, AssigneeID: &legal.ID}).Code)
	get()
	assert.Equal(t, models.Invalid, detail.Dossier.Status)
	assert.Equal(t, 200, request(t, router, "PATCH", base+"/fields/"+detail.Fields[0].ID.String(), legalToken, dd.CorrectionRequest{ExpectedVersion: detail.Dossier.Version, Value: "079000000001", Reason: "OTHER", Note: "Checked original"}).Code)
	get()
	assert.Equal(t, models.Valid, detail.Dossier.Status)
	for _, issue := range detail.Issues {
		if issue.Status == "OPEN" && issue.Severity == "WARNING" {
			assert.Equal(t, 200, request(t, router, "POST", base+"/issues/"+issue.ID.String()+"/acknowledge", legalToken, dd.ReasonRequest{ExpectedVersion: detail.Dossier.Version, Reason: "Registry verified manually in synthetic test"}).Code)
			get()
		}
	}
	data[models.Decision](t, request(t, router, "POST", base+"/decisions", legalToken, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE"}), 200)
	get()
	assert.Equal(t, models.Approved, detail.Dossier.Status)
	// The historical drill-down includes a prior VALID transition even after approval.
	today := time.Now().In(time.FixedZone("ICT", 7*3600)).Format("2006-01-02")
	historical := data[common.List[models.Dossier]](t, request(t, router, "GET", "/api/v1/dossiers?status=VALID_DOCUMENT&from="+today+"&to="+today, admin, nil), 200)
	assert.Equal(t, int64(1), historical.Total)
	audit := request(t, router, "GET", "/api/v1/audit-logs?dossier_id="+d.ID.String(), admin, nil)
	assert.Equal(t, 200, audit.Code)
	assert.Contains(t, audit.Body.String(), "WARNING_ACKNOWLEDGED")
}

func TestProcessingClaimsLeaseRecovery(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	company, dossier := uuid.New(), uuid.New()
	d := models.Dossier{CompanyID: company, CreatorID: uuid.New(), Title: "Claims", TypeCode: "fixture", Status: models.Processing, Version: 1}
	d.ID = dossier
	require.NoError(t, app.DB.Create(&d).Error)
	f := models.File{CompanyID: company, DossierID: dossier, BatchID: uuid.New(), OriginalName: "synthetic.pdf", StorageKey: "unused", MIME: "application/pdf", SHA256: "fixture", Status: "UPLOADED"}
	require.NoError(t, app.DB.Create(&f).Error)
	repo := repositories.NewDossierRepository(app.DB)
	results := make(chan models.File, 2)
	failures := make(chan error, 2)
	var group sync.WaitGroup
	for i := 0; i < 2; i++ {
		group.Go(func() {
			claimed, err := repo.Claim(ctx, time.Minute)
			if err != nil {
				failures <- err
			} else {
				results <- claimed
			}
		})
	}
	group.Wait()
	close(results)
	close(failures)
	require.Len(t, results, 1, "only one worker can claim one queued file")
	claimed := <-results
	for err := range failures {
		assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), err)
	}
	require.NotNil(t, claimed.ClaimID)
	require.NoError(t, repo.Renew(ctx, claimed, time.Minute))
	require.NoError(t, app.DB.Model(&models.File{}).Where("id = ?", f.ID).Update("lease_until", time.Now().UTC().Add(-time.Minute)).Error)
	recovered, err := repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, claimed.Attempt+1, recovered.Attempt)
	assert.NotEqual(t, *claimed.ClaimID, *recovered.ClaimID)
	assert.ErrorIs(t, repo.Renew(ctx, claimed, time.Minute), common.ErrConflict, "expired worker cannot renew a new claim")
	require.NoError(t, app.DB.Model(&models.Dossier{}).Where("id = ?", d.ID).Update("status", models.Approved).Error)
	require.NoError(t, app.DB.Model(&models.File{}).Where("id = ?", f.ID).Updates(map[string]any{"status": "UPLOADED", "claim_id": nil, "lease_until": nil}).Error)
	_, err = repo.Claim(ctx, time.Minute)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "approved dossiers are never processed")
}
