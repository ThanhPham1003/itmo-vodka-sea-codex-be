package bootstrap_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	am "sea-codex-server/internal/audit/models"
	"sea-codex-server/internal/bootstrap"
	"sea-codex-server/internal/common"
	dd "sea-codex-server/internal/dossier/dto"
	dm "sea-codex-server/internal/dossier/models"
)

func TestGETDisplayLabelsPreserveIDsPinnedVersionsAndTenantScope(t *testing.T) {
	app := newTestApp(t)
	router, err := bootstrap.NewRouter(app)
	require.NoError(t, err)
	admin, token := signupAndLogin(t, router, "display-labels")
	foreign, _ := signupAndLogin(t, router, "display-foreign")
	doc := data[dm.DocumentType](t, request(t, router, "POST", "/api/v1/document-types", token, dd.DocumentTypeRequest{DocumentDefinition: dm.DocumentDefinition{
		Name: "Căn cước bản đầu", Fields: []dm.FieldSchema{{Label: "Họ tên bản đầu", Type: "text"}},
	}}), 201)
	typ := data[dm.DossierType](t, request(t, router, "POST", "/api/v1/dossier-types", token, dd.DossierTypeRequest{DossierDefinition: dm.DossierDefinition{
		Name: "Hồ sơ mua nhà", Documents: []dm.DocumentRequirement{{DocumentType: doc.Code, Required: true}},
	}}), 201)
	dossier := data[dm.Dossier](t, request(t, router, "POST", "/api/v1/dossiers", token, dd.CreateDossierRequest{Title: "Nhà ở đường 18", DossierType: typ.Code}), 201)
	require.NoError(t, app.DB.Model(&dossier).Updates(map[string]any{"assignee_id": admin.ID, "status": dm.Valid}).Error)
	decision := dm.Decision{CompanyID: admin.CompanyID, DossierID: dossier.ID, ActorID: admin.ID, Action: "REQUEST_CHANGES", Reason: "Kiểm tra bản gốc", Version: 1}
	require.NoError(t, app.DB.Create(&decision).Error)
	def := doc.Definition
	def.Name = "Căn cước đổi tên"
	def.Fields[0].Label = "Họ tên đổi nhãn"
	data[dm.DocumentType](t, request(t, router, "PUT", "/api/v1/document-types/"+doc.Code, token, dd.DocumentTypeRequest{DocumentDefinition: def, ExpectedVersion: 1}), 200)
	require.Equal(t, 200, request(t, router, "DELETE", "/api/v1/document-types/"+doc.Code, token, nil).Code)

	for _, path := range []string{"/api/v1/dossiers", "/api/v1/dossiers/review-queue"} {
		rows := data[common.List[dm.Dossier]](t, request(t, router, "GET", path, token, nil), 200)
		require.Len(t, rows.Items, 1)
		assert.Equal(t, dossier.ID, rows.Items[0].ID)
		assert.Equal(t, admin.FullName, rows.Items[0].CreatorName)
		assert.Equal(t, admin.FullName, rows.Items[0].AssigneeName)
	}
	base := "/api/v1/dossiers/" + dossier.ID.String()
	detail := data[dd.Detail](t, request(t, router, "GET", base, token, nil), 200)
	assert.Equal(t, admin.FullName, detail.Dossier.AssigneeName)
	require.Len(t, detail.Decisions, 1)
	assert.Equal(t, admin.ID, detail.Decisions[0].ActorID)
	assert.Equal(t, admin.FullName, detail.Decisions[0].ActorName)
	assert.Equal(t, "Căn cước bản đầu", detail.Dossier.Snapshot.Documents[0].Definition.Name)
	types := data[common.List[dm.DossierType]](t, request(t, router, "GET", "/api/v1/dossier-types", token, nil), 200)
	single := data[dm.DossierType](t, request(t, router, "GET", "/api/v1/dossier-types/"+typ.Code, token, nil), 200)
	for _, item := range []dm.DossierType{types.Items[0], single} {
		require.Len(t, item.DocumentSchemas, 1)
		assert.Equal(t, doc.Code, item.DocumentSchemas[0].Code)
		assert.Equal(t, 1, item.DocumentSchemas[0].Version)
		assert.Equal(t, "Căn cước bản đầu", item.DocumentSchemas[0].Definition.Name)
		assert.Equal(t, "Họ tên bản đầu", item.DocumentSchemas[0].Definition.Fields[0].Label)
	}
	for _, path := range []string{"/api/v1/document-types", "/api/v1/document-types/" + doc.Code} {
		res := request(t, router, "GET", path, token, nil)
		require.Equal(t, 200, res.Code)
		assert.Contains(t, res.Body.String(), "Căn cước đổi tên")
	}
	logs := data[common.List[am.AuditLog]](t, request(t, router, "GET", "/api/v1/audit-logs?limit=100", token, nil), 200)
	for _, row := range logs.Items {
		switch row.Resource {
		case "dossier":
			assert.Equal(t, dossier.Title, row.ResourceName)
			assert.Equal(t, dossier.Title, row.DossierTitle)
		case "document_type":
			assert.Equal(t, "Căn cước đổi tên", row.ResourceName)
		case "dossier_type":
			assert.Equal(t, typ.Definition.Name, row.ResourceName)
		case "user":
			assert.Equal(t, admin.FullName, row.ResourceName)
		case "company":
			assert.NotEmpty(t, row.ResourceName)
		}
	}
	// Corrupt foreign references must retain their IDs without leaking names.
	require.NoError(t, app.DB.Model(&dossier).Updates(map[string]any{"assignee_id": foreign.ID}).Error)
	require.NoError(t, app.DB.Model(&decision).Update("actor_id", foreign.ID).Error)
	detail = data[dd.Detail](t, request(t, router, "GET", base, token, nil), 200)
	assert.Empty(t, detail.Dossier.AssigneeName)
	assert.Empty(t, detail.Decisions[0].ActorName)
	foreignDoc := dm.Dossier{CompanyID: foreign.CompanyID, CreatorID: foreign.ID, Title: "private-foreign-title"}
	require.NoError(t, app.DB.Create(&foreignDoc).Error)
	event := am.AuditLog{CompanyID: admin.CompanyID, ActorID: foreign.ID, Resource: "dossier", ResourceID: foreignDoc.ID, DossierID: &foreignDoc.ID, Action: "FOREIGN_REFERENCE", CreatedAt: time.Now()}
	require.NoError(t, app.DB.Create(&event).Error)
	logs = data[common.List[am.AuditLog]](t, request(t, router, "GET", "/api/v1/audit-logs?action=FOREIGN_REFERENCE", token, nil), 200)
	require.Len(t, logs.Items, 1)
	assert.Empty(t, logs.Items[0].ResourceName)
	assert.Empty(t, logs.Items[0].DossierTitle)
	assert.Empty(t, logs.Items[0].ActorName)
	assert.Equal(t, foreignDoc.ID, logs.Items[0].ResourceID)
	assert.Equal(t, 404, request(t, router, "GET", "/api/v1/dossiers/"+foreignDoc.ID.String(), token, nil).Code)
	for _, entry := range []struct {
		model any
		field string
	}{{&dm.Dossier{}, "assignee_name"}, {&dm.Decision{}, "actor_name"}, {&dm.DossierType{}, "document_schemas"}, {&am.AuditLog{}, "resource_name"}} {
		assert.False(t, app.DB.Migrator().HasColumn(entry.model, entry.field))
	}
}
