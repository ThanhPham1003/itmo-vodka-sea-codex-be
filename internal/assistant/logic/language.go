package logic

import (
	"regexp"
	"sea-codex-server/internal/common"
	"strings"
)

// Keep enforcement on user-visible prose only. JSON keys and citation IDs remain stable.
var internalTerm = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]*(?:_[A-Za-z0-9]+)+\b|\b[a-z][a-z0-9_]*:[a-z][a-z0-9_]*\b|\b(?:PROCESSING|APPROVED|REJECTED|WARNING|BLOCKER|FAILED|UPLOADED|EXTRACTED|UNKNOWN|RESOLVED|OPEN|ACKNOWLEDGED|broker|manager|notary)\b`)
var internalPath = regexp.MustCompile(`(?i)/api/v[0-9]+(?:/|\b)|\b(?:SQLSTATE|PASETO|GORM)\b`)
var duplicatePermission = regexp.MustCompile(`(?i)quyền\s+quyền\b|permission\s+permission\b`)

var displayTerms = map[string][2]string{
	"VALID_DOCUMENT": {"Chờ kiểm duyệt", "Awaiting review"}, "APPROVED_DOCUMENT": {"Đã phê duyệt", "Approved"},
	"PROCESSING": {"Đang xử lý", "Processing"}, "MISSING_DOCUMENT": {"Thiếu giấy tờ", "Missing documents"}, "INVALID_DOCUMENT": {"Có sai lệch", "Has discrepancies"},
	"APPROVED": {"Đã phê duyệt", "Approved"}, "REJECTED": {"Đã từ chối", "Rejected"},
	"WARNING": {"cảnh báo cần kiểm tra", "warning to check"}, "BLOCKER": {"vấn đề bắt buộc phải khắc phục", "issue that must be resolved"},
	"FAILED": {"Xử lý lỗi", "Processing failed"}, "UPLOADED": {"Đã tiếp nhận", "Received"}, "EXTRACTED": {"Đã đọc thông tin", "Information read"}, "UNKNOWN": {"Chờ phân loại", "Awaiting classification"},
	"RESOLVED": {"Đã xử lý", "Resolved"}, "OPEN": {"Cần xử lý", "Needs attention"}, "ACKNOWLEDGED": {"Đã xác nhận", "Acknowledged"},
	"PENDING_DOCUMENTS": {"Chờ dữ liệu", "Awaiting information"}, "PENDING_CONDITION": {"Chờ xác định", "Awaiting confirmation"}, "NOT_REQUIRED": {"Không áp dụng", "Not applicable"},
	"company_admin": {"Quản trị công ty", "Company administrator"}, "legal_specialist": {"Chuyên viên pháp lý", "Legal specialist"},
	"broker": {"Nhân viên môi giới", "Broker"}, "manager": {"Quản lý", "Manager"}, "notary": {"Công chứng viên", "Notary"},
	"dossier:review": {"quyền kiểm duyệt hồ sơ", "permission to review dossiers"}, "dossier:create": {"quyền tạo hồ sơ", "permission to create dossiers"},
	"dossier:read": {"quyền xem hồ sơ", "permission to view dossiers"}, "dossier:read_all": {"quyền xem hồ sơ của công ty", "permission to view company dossiers"},
	"dossier:upload": {"quyền thêm tài liệu", "permission to add documents"}, "dossier:correct": {"quyền sửa thông tin", "permission to correct information"},
	"dossier:assign": {"quyền phân công hồ sơ", "permission to assign dossiers"}, "dossier:reopen": {"quyền mở lại hồ sơ", "permission to reopen dossiers"},
	"dossier:image_view": {"quyền xem bản gốc", "permission to view originals"}, "pii:view": {"quyền xem thông tin cá nhân", "permission to view personal information"},
	"audit:read": {"quyền xem nhật ký", "permission to view activity logs"}, "user:manage": {"quyền quản lý thành viên", "permission to manage members"},
	"catalog:manage": {"quyền quản lý cấu hình hồ sơ", "permission to manage dossier settings"}, "dashboard:read": {"quyền xem tổng quan", "permission to view the overview"},
	"expected_version": {"phiên bản hồ sơ", "dossier version"},
}

func userFacingText(text, locale string) (string, bool) {
	i := 0
	if locale == "en" {
		i = 1
	}
	ok := !internalPath.MatchString(text)
	text = internalTerm.ReplaceAllStringFunc(text, func(term string) string {
		if value, exists := displayTerms[term]; exists {
			return value[i]
		}
		ok = false
		return term
	})
	text = strings.ReplaceAll(text, "`", "")
	text = duplicatePermission.ReplaceAllStringFunc(text, func(term string) string {
		if strings.HasPrefix(strings.ToLower(term), "quyền") {
			return "quyền"
		}
		return "permission"
	})
	// A model can repeat a code in parentheses after an already translated label.
	for _, value := range displayTerms {
		label := value[i]
		duplicate := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(label) + `\s*\(\s*` + regexp.QuoteMeta(label) + `\s*\)`)
		text = duplicate.ReplaceAllString(text, label)
	}
	return text, ok
}

func audienceAccess(a common.Actor, locale string) map[string]any {
	i := 0
	role := "Thành viên công ty"
	if locale == "en" {
		i = 1
		role = "Company member"
	}
	if value, ok := displayTerms[a.Role]; ok {
		role = value[i]
	}
	actions := []string{}
	for _, permission := range []string{"dossier:create", "dossier:read", "dossier:read_all", "dossier:upload", "dossier:correct", "dossier:assign", "dossier:review", "dossier:reopen", "dossier:image_view", "pii:view", "audit:read", "user:manage", "catalog:manage", "dashboard:read"} {
		if a.HasPermission(permission) {
			actions = append(actions, displayTerms[permission][i])
		}
	}
	return map[string]any{"role_label": role, "allowed_actions": actions}
}
