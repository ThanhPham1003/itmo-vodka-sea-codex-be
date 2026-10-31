package repositories_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"regexp"
	"sea-codex-server/internal/assistant/repositories"
	"sea-codex-server/internal/common"
	"testing"
)

func TestKnowledgeRetrieval(t *testing.T) {
	r, err := repositories.NewKnowledgeRepository()
	require.NoError(t, err)
	for _, tc := range []struct{ query, want string }{
		{"tai sao cho kiem duyet chua duoc duyet", "review-readiness"},
		{"Why can't I approve a dossier?", "review-readiness"},
		{"Xác nhận để bỏ qua BLOCKER ở đâu?", "reconciliation-issues"},
		{"Đổi cấu hình có áp dụng hồ sơ cũ không?", "catalog-versions"},
		{"Đăng nhập lại khi tải lại trang", "login-session"},
		{"Giấy kết hôn có bắt buộc không?", "checklist-conditions"},
		{"Excel có upload được không?", "upload-limits"},
		{"Dashboard unique transitions", "dashboard-metrics"},
		{"Broker xem CCCD", "privacy-evidence"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			found, err := r.Search(context.Background(), tc.query, "detail")
			require.NoError(t, err)
			ids := []string{}
			for _, a := range found {
				ids = append(ids, a.ID)
			}
			assert.Contains(t, ids, tc.want)
			assert.LessOrEqual(t, len(found), 5)
		})
	}
	list, err := r.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 18)
	internal := regexp.MustCompile(`\b[A-Za-z]+_[A-Za-z_]+\b|\b[a-z]+:[a-z_]+\b|\b(?:WARNING|BLOCKER|FAILED|PII|snapshot|schema|worker)\b`)
	for _, article := range list {
		for locale, translation := range article.Translations {
			assert.NotRegexp(t, internal, translation.Content, article.ID+" "+locale)
			assert.NotRegexp(t, internal, translation.Title, article.ID+" title")
		}
	}
	list[0].Translations["vi"] = list[0].Translations["en"]
	other, err := r.Get(context.Background(), list[0].ID)
	require.NoError(t, err)
	assert.NotEqual(t, other.Translations["vi"], other.Translations["en"])
	_, err = r.Get(context.Background(), "../../.env")
	assert.ErrorIs(t, err, common.ErrNotFound)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = r.Search(ctx, "review", "")
	assert.ErrorIs(t, err, context.Canceled)
	empty, err := r.Search(context.Background(), "zxqv987", "")
	require.NoError(t, err)
	assert.Empty(t, empty)
}
