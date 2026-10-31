package bootstrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ad "sea-codex-server/internal/auth/dto"
	"sea-codex-server/internal/bootstrap"
	dd "sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
)

func request(t *testing.T, r http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func data[T any](t *testing.T, w *httptest.ResponseRecorder, status int) T {
	t.Helper()
	require.Equal(t, status, w.Code, w.Body.String())
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	var out T
	require.NoError(t, json.Unmarshal(envelope.Data, &out))
	return out
}
func signupAndLogin(t *testing.T, r http.Handler, name string) (ad.UserResponse, string) {
	t.Helper()
	in := map[string]any{"company": map[string]any{"name": name}, "username": name, "email": name + "@example.test", "password": "a-valid-test-password", "full_name": "Test Admin"}
	u := data[ad.UserResponse](t, request(t, r, "POST", "/api/v1/auth/signup", "", in), 201)
	login := data[ad.LoginResponse](t, request(t, r, "POST", "/api/v1/auth/login", "", ad.LoginRequest{UsernameOrEmail: name, Password: "a-valid-test-password"}), 200)
	return u, login.AccessToken
}
func fakeVision(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Messages []struct {
				Content []struct{ Type, Text string } `json:"content"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Messages) != 1 {
			w.WriteHeader(400)
			return
		}
		prompt := ""
		for _, part := range in.Messages[0].Content {
			if part.Type == "text" {
				prompt = part.Text
			}
		}
		var answer any
		if strings.Contains(prompt, "Classify the current") {
			index := strings.Index(prompt, "Configuration: ")
			var cfg struct {
				Hint        string                  `json:"hint"`
				CurrentPage int                     `json:"current_page"`
				Allowed     []models.PinnedDocument `json:"allowed_document_types"`
			}
			if index < 0 || json.Unmarshal([]byte(prompt[index+len("Configuration: "):]), &cfg) != nil {
				w.WriteHeader(400)
				return
			}
			typ := cfg.Hint
			if typ == "" && len(cfg.Allowed) > 0 {
				typ = cfg.Allowed[(cfg.CurrentPage-1)%len(cfg.Allowed)].Code
			}
			answer = map[string]any{"document_type": typ, "role": "", "confidence": 0.99, "starts_new_document": true, "boundary_confidence": 1}
		} else if strings.Contains(prompt, "Extract only visibly") {
			index := strings.Index(prompt, "Schema: ")
			var def models.DocumentDefinition
			if index < 0 || json.Unmarshal([]byte(prompt[index+len("Schema: "):]), &def) != nil {
				w.WriteHeader(400)
				return
			}
			value := "079000000001"
			if strings.Contains(def.Name, "Contract") {
				value = "079000000002"
			}
			fields := []map[string]any{}
			for _, field := range def.Fields {
				fields = append(fields, map[string]any{"key": field.Key, "value": value, "raw_text": "visible evidence " + value, "confidence": 0.99, "page": 1, "bounding_box": []float64{0.1, 0.1, 0.8, 0.2}})
			}
			page := 1
			_, _ = fmt.Sscanf(prompt, "Extract only visibly supported values from this single page (page %d).", &page)
			for _, field := range fields {
				field["page"] = page
			}
			answer = map[string]any{"fields": fields}
		} else {
			answer = map[string]any{"checks": []any{}}
		}
		encoded, _ := json.Marshal(answer)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": string(encoded)}}}})
	}))
}
func upload(t *testing.T, r http.Handler, path, token, typ string, shade uint8) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("document_types", typ))
	part, err := writer.CreateFormFile("files", "synthetic.png")
	require.NoError(t, err)
	img := image.NewRGBA(image.Rect(0, 0, 180, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 180; x++ {
			if y%20 >= 8 && y%20 <= 11 && x >= 15 && x <= 160 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.RGBA{shade, uint8(x), uint8(y), 255})
			}
		}
	}
	require.NoError(t, png.Encode(part, img))
	require.NoError(t, writer.Close())
	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestEndToEndTenantIntakeReview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := fakeVision(t)
	defer provider.Close()
	app := newTestApp(t)
	app.Config.Processing.Enabled = true
	app.Config.Processing.BaseURL = provider.URL
	app.Config.Processing.APIKey = "fixture-key"
	app.Config.Processing.Model = "fixture-model"
	r, err := bootstrap.NewRouter(app)
	require.NoError(t, err)
	admin, adminToken := signupAndLogin(t, r, "company-a")
	_, otherToken := signupAndLogin(t, r, "company-b")
	bad := request(t, r, "POST", "/api/v1/auth/signup", "", map[string]any{"company_id": admin.CompanyID, "role": "company_admin"})
	assert.Equal(t, 400, bad.Code)
	member := data[ad.UserResponse](t, request(t, r, "POST", "/api/v1/auth/users", adminToken, ad.CreateUserRequest{Username: "broker-a", Email: "broker-a@example.test", Password: "a-valid-test-password", FullName: "Broker", Role: "broker"}), 201)
	brokerLogin := data[ad.LoginResponse](t, request(t, r, "POST", "/api/v1/auth/login", "", ad.LoginRequest{UsernameOrEmail: member.Email, Password: "a-valid-test-password"}), 200)
	brokerToken := brokerLogin.AccessToken
	me := data[ad.UserResponse](t, request(t, r, "GET", "/api/v1/auth/me", brokerToken, nil), 200)
	assert.Equal(t, admin.CompanyID, me.CompanyID)
	createDoc := func(name string) models.DocumentType {
		return data[models.DocumentType](t, request(t, r, "POST", "/api/v1/document-types", adminToken, dd.DocumentTypeRequest{DocumentDefinition: models.DocumentDefinition{Name: name, Description: "Synthetic test", Fields: []models.FieldSchema{{Label: "Identity number", Type: "identifier", PII: true, Required: true}}}}), 201)
	}
	doc1, doc2 := createDoc("Identity"), createDoc("Contract")
	assert.NotEmpty(t, doc1.Code)
	assert.NotEmpty(t, doc1.Definition.Fields[0].Key)
	assert.Equal(t, 403, request(t, r, "POST", "/api/v1/document-types", brokerToken, dd.DocumentTypeRequest{DocumentDefinition: doc1.Definition}).Code)
	assert.Equal(t, 404, request(t, r, "GET", "/api/v1/document-types/"+doc1.Code, otherToken, nil).Code)
	typ := data[models.DossierType](t, request(t, r, "POST", "/api/v1/dossier-types", adminToken, dd.DossierTypeRequest{DossierDefinition: models.DossierDefinition{Name: "Test dossier", Documents: []models.DocumentRequirement{{DocumentType: doc1.Code, Required: true}, {DocumentType: doc2.Code, Required: true}}, Rules: []models.ComparisonRule{{Name: "IDs agree", Left: models.FieldReference{DocumentType: doc1.Code, FieldKey: doc1.Definition.Fields[0].Key}, Right: models.FieldReference{DocumentType: doc2.Code, FieldKey: doc2.Definition.Fields[0].Key}, Operator: "DIGITS_EQUAL", Severity: "BLOCKER"}}}}), 201)
	d := data[models.Dossier](t, request(t, r, "POST", "/api/v1/dossiers", brokerToken, dd.CreateDossierRequest{Title: "Synthetic dossier", DossierType: typ.Code}), 201)
	assert.Equal(t, models.Missing, d.Status)
	base := "/api/v1/dossiers/" + d.ID.String()
	assert.Equal(t, 404, request(t, r, "GET", base, otherToken, nil).Code)
	_, unrelatedBroker := signupAndLogin(t, r, "unrelated-company")
	assert.Equal(t, 404, request(t, r, "GET", base, unrelatedBroker, nil).Code)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = app.Workers[0].Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("worker shutdown timeout")
		}
	})
	first := data[dd.UploadResult](t, upload(t, r, base+"/files", brokerToken, doc1.Code, 20), 200)
	require.Equal(t, "UPLOADED", first.Files[0].Status)
	var detail dd.Detail
	waitStatus := func(status string) {
		t.Helper()
		require.Eventually(t, func() bool {
			w := request(t, r, "GET", base, adminToken, nil)
			if w.Code != 200 {
				return false
			}
			var envelope struct {
				Data dd.Detail `json:"data"`
			}
			if json.Unmarshal(w.Body.Bytes(), &envelope) != nil {
				return false
			}
			detail = envelope.Data
			return detail.Dossier.Status == status
		}, 5*time.Second, 20*time.Millisecond, "did not reach %s; last status=%s", status, detail.Dossier.Status)
	}
	waitStatus(models.Missing)
	require.Eventually(t, func() bool {
		detail = data[dd.Detail](t, request(t, r, "GET", base, adminToken, nil), 200)
		return len(detail.Fields) == 1
	}, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, 409, request(t, r, "POST", base+"/decisions", adminToken, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE"}).Code)
	duplicate := data[dd.UploadResult](t, upload(t, r, base+"/files", brokerToken, doc1.Code, 20), 200)
	assert.Equal(t, "DUPLICATE", duplicate.Files[0].Status)
	second := data[dd.UploadResult](t, upload(t, r, base+"/files", brokerToken, doc2.Code, 80), 200)
	require.Equal(t, "UPLOADED", second.Files[0].Status)
	waitStatus(models.Invalid)
	redactedBody := request(t, r, "GET", base, brokerToken, nil).Body.String()
	assert.NotContains(t, redactedBody, "079000000001")
	assert.NotContains(t, redactedBody, "079000000002")
	assert.NotContains(t, redactedBody, app.Config.Uploads.Directory)
	assert.Equal(t, 403, request(t, r, "POST", base+"/pii/reveal", brokerToken, dd.RevealRequest{FieldIDs: []uuid.UUID{detail.Fields[0].ID}}).Code)
	assert.Equal(t, 403, request(t, r, "GET", base+"/files/"+first.Files[0].ID.String()+"/content", brokerToken, nil).Code)
	var target models.ExtractedField
	for _, f := range detail.Fields {
		if f.Key == doc2.Definition.Fields[0].Key {
			target = f
		}
	}
	full := data[[]models.ExtractedField](t, request(t, r, "POST", base+"/pii/reveal", adminToken, dd.RevealRequest{FieldIDs: []uuid.UUID{target.ID}}), 200)
	require.Len(t, full, 1)
	assert.Equal(t, "079000000002", full[0].Value)
	stale := detail.Dossier.Version
	assert.Equal(t, 200, request(t, r, "PATCH", base+"/fields/"+target.ID.String(), adminToken, dd.CorrectionRequest{ExpectedVersion: stale, Value: "079000000001", Reason: "OCR_ERROR"}).Code)
	waitStatus(models.Valid)
	assert.Equal(t, 409, request(t, r, "POST", base+"/decisions", adminToken, dd.ReviewRequest{ExpectedVersion: stale, Action: "APPROVE"}).Code)
	decision := data[models.Decision](t, request(t, r, "POST", base+"/decisions", adminToken, dd.ReviewRequest{ExpectedVersion: detail.Dossier.Version, Action: "APPROVE"}), 200)
	assert.Equal(t, "APPROVE", decision.Action)
	waitStatus(models.Approved)
	assert.Equal(t, 409, upload(t, r, base+"/files", brokerToken, doc1.Code, 100).Code)
	assert.Equal(t, 200, request(t, r, "POST", base+"/reopen", adminToken, dd.ReasonRequest{ExpectedVersion: detail.Dossier.Version, Reason: "Synthetic follow-up"}).Code)
	waitStatus(models.Valid)
	assert.Len(t, detail.Decisions, 2)
	dashboard := request(t, r, "GET", "/api/v1/dashboard/statuses?metric=transitions", adminToken, nil)
	assert.Equal(t, 200, dashboard.Code)
	assert.Contains(t, dashboard.Body.String(), "APPROVED_DOCUMENT")
	audit := request(t, r, "GET", "/api/v1/audit-logs?action=VIEW_PII", adminToken, nil)
	assert.Equal(t, 200, audit.Code)
	assert.Contains(t, audit.Body.String(), "VIEW_PII")
	assert.NotContains(t, audit.Body.String(), "079000000002")
	assert.Equal(t, 403, request(t, r, "GET", "/api/v1/dashboard/statuses", brokerToken, nil).Code)
	content := request(t, r, "GET", base+"/files/"+first.Files[0].ID.String()+"/content", adminToken, nil)
	assert.Equal(t, 200, content.Code)
	assert.Equal(t, "image/png", content.Header().Get("Content-Type"))
}
