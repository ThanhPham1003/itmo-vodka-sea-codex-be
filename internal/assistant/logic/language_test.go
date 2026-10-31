package logic

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAnswerUsesUserLanguage(t *testing.T) {
	for _, tc := range []struct {
		name, locale, text, want string
		ok                       bool
	}{
		{"Vietnamese states", "vi", "Chờ kiểm duyệt (VALID_DOCUMENT) chưa phải APPROVED_DOCUMENT.", "Chờ kiểm duyệt chưa phải Đã phê duyệt.", true},
		{"English states", "en", "VALID_DOCUMENT is not APPROVED_DOCUMENT.", "Awaiting review is not Approved.", true},
		{"permissions and roles", "vi", "Cần dossier:review, trừ company_admin.", "Cần quyền kiểm duyệt hồ sơ, trừ Quản trị công ty.", true},
		{"permission label already present", "vi", "Cần quyền dossier:review.", "Cần quyền kiểm duyệt hồ sơ.", true},
		{"overlapping codes", "vi", "INVALID_DOCUMENT; VALID_DOCUMENT; dossier:read_all.", "Có sai lệch; Chờ kiểm duyệt; quyền xem hồ sơ của công ty.", true},
		{"unresolved issues", "vi", "Không còn WARNING/BLOCKER.", "Không còn cảnh báo cần kiểm tra/vấn đề bắt buộc phải khắc phục.", true},
		{"unknown internal field", "vi", "Hãy sửa internal_field_name.", "Hãy sửa internal_field_name.", false},
		{"unknown permission", "en", "Use dossier:delete.", "Use dossier:delete.", false},
		{"API instruction", "vi", "Gọi /api/v1/dossiers.", "Gọi /api/v1/dossiers.", false},
		{"plain text and formats", "vi", "Chọn Thêm tài liệu để tải PDF, JPEG hoặc PNG.", "Chọn Thêm tài liệu để tải PDF, JPEG hoặc PNG.", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := userFacingText(tc.text, tc.locale)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, out)
		})
	}
}

func TestChatConvertsLegacyModelVocabulary(t *testing.T) {
	l, c, _, ctx := fixture(t)
	c.body = `{"status":"answered","blocks":[{"text":"Chờ kiểm duyệt (VALID_DOCUMENT) chưa phải APPROVED_DOCUMENT. Cần dossier:review, đúng người phụ trách trừ company_admin và không còn WARNING/BLOCKER.","source_ids":["review-readiness"]}]}`
	in := req()
	in.Locale = "vi"
	in.Message = "Chờ kiểm duyệt có phải đã duyệt?"
	out, err := l.Chat(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, "answered", out.Status)
	require.Len(t, out.Sources, 1)
	for _, b := range out.Blocks {
		assert.NotRegexp(t, internalTerm, b.Text)
	}
	assert.Contains(t, out.Blocks[0].Text, "Chờ kiểm duyệt chưa phải Đã phê duyệt")
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(c.prompt), &payload))
	access := string(payload["user_access"])
	assert.NotContains(t, access, "dossier:read")
	assert.NotContains(t, access, `"broker"`)
	assert.Contains(t, access, "Nhân viên môi giới")
	assert.NotContains(t, string(payload["articles"]), "VALID_DOCUMENT")
	assert.NotContains(t, string(payload["articles"]), "dossier:review")
}

func TestUnrecognizedTechnicalAnswerIsNotPublished(t *testing.T) {
	l, c, a, ctx := fixture(t)
	c.body = `{"status":"answered","blocks":[{"text":"Edit hidden_internal_column to approve.","source_ids":["review-readiness"]}]}`
	out, err := l.Chat(ctx, req())
	require.NoError(t, err)
	assert.Equal(t, "insufficient_information", out.Status)
	assert.Empty(t, out.Sources)
	assert.NotContains(t, out.Blocks[0].Text, "hidden_internal_column")
	assert.Equal(t, "ASSISTANT_REFUSED", a.event.Action)
}
