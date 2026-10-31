package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"time"

	"github.com/google/uuid"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
)

func evidence(f models.ExtractedField) models.Evidence {
	return models.Evidence{FileID: f.FileID, GroupID: f.GroupID, FieldID: f.ID, FieldKey: f.Key, Page: f.Page, Value: f.Value}
}
func fingerprint(issue models.Issue) string {
	raw, _ := json.Marshal(struct {
		Code, Status, Severity string
		Evidence               []models.Evidence
	}{issue.Code, issue.Status, issue.Severity, issue.Evidence})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func groupCandidates(ref models.FieldReference, groups []models.Group) []models.Group {
	out := []models.Group{}
	for _, g := range groups {
		if g.DocumentType == ref.DocumentType && (ref.Role == "" || g.Role == ref.Role) && !g.NeedsConfirmation && (g.Confirmed || g.Confidence >= 0.8) {
			out = append(out, g)
		}
	}
	return out
}
func findField(fields []models.ExtractedField, group uuid.UUID, key string) (models.ExtractedField, bool) {
	for _, f := range fields {
		if f.GroupID == group && f.Key == key {
			return f, true
		}
	}
	return models.ExtractedField{}, false
}
func ruleIssue(rule models.ComparisonRule, b repositories.Bundle) models.Issue {
	i := models.Issue{RuleCode: rule.Code, Code: "MATCH", Severity: rule.Severity, Status: "RESOLVED", Message: rule.Name, Remediation: rule.Remediation, Evidence: []models.Evidence{}}
	left, right := groupCandidates(rule.Left, b.Groups), groupCandidates(rule.Right, b.Groups)
	if len(left) == 0 || len(right) == 0 {
		i.Code = "PENDING_DOCUMENTS"
		i.Status = "PENDING_DOCUMENTS"
		i.Message = "Chưa đủ giấy tờ nguồn: " + rule.Name
		return i
	}
	type pair struct{ left, right models.Group }
	pairs := []pair{}
	if rule.Left.SubjectField == "" {
		if len(left) == 1 && len(right) == 1 {
			pairs = append(pairs, pair{left[0], right[0]})
		}
	} else {
		for _, l := range left {
			lf, lok := findField(b.Fields, l.ID, rule.Left.SubjectField)
			if !lok || !trusted(lf) {
				continue
			}
			for _, r := range right {
				rf, rok := findField(b.Fields, r.ID, rule.Right.SubjectField)
				lv, le := normalizeField(lf.Value, lf.Type)
				rv, re := normalizeField(rf.Value, rf.Type)
				if rok && trusted(rf) && le == nil && re == nil && lv == rv {
					pairs = append(pairs, pair{l, r})
				}
			}
		}
	}
	// A complete one-to-one join is required. Never choose an arbitrary high-confidence pair.
	leftUsed, rightUsed := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	unambiguous := len(pairs) > 0 && len(pairs) == len(left) && len(pairs) == len(right)
	for _, p := range pairs {
		if leftUsed[p.left.ID] || rightUsed[p.right.ID] {
			unambiguous = false
		}
		leftUsed[p.left.ID], rightUsed[p.right.ID] = true, true
	}
	if !unambiguous {
		i.Code, i.Status, i.Severity = "AMBIGUOUS_SUBJECT", "OPEN", "WARNING"
		i.Message = "Cần chuyên viên xác định đúng cặp tài liệu: " + rule.Name
		return i
	}
	mismatch, uncertain := false, false
	for _, p := range pairs {
		lf, lok := findField(b.Fields, p.left.ID, rule.Left.FieldKey)
		rf, rok := findField(b.Fields, p.right.ID, rule.Right.FieldKey)
		if lok {
			i.Evidence = append(i.Evidence, evidence(lf))
		}
		if rok {
			i.Evidence = append(i.Evidence, evidence(rf))
		}
		if !lok || !rok || !trusted(lf) || !trusted(rf) {
			uncertain = true
			continue
		}
		equal, err := compareFields(rule.Operator, lf, rf)
		if err != nil {
			uncertain = true
		} else if !equal {
			mismatch = true
		}
	}
	if uncertain {
		i.Code, i.Status, i.Severity = "NEEDS_VERIFICATION", "OPEN", "WARNING"
		i.Message = "Thiếu dữ liệu hoặc độ tin cậy thấp, cần xác minh: " + rule.Name
	} else if mismatch {
		i.Code, i.Status = "MISMATCH", "OPEN"
		i.Message = rule.Message
		if i.Message == "" {
			i.Message = "Dữ liệu không khớp: " + rule.Name
		}
	}
	return i
}
func compareFields(operator string, left, right models.ExtractedField) (bool, error) {
	var l, r string
	var le, re error
	switch operator {
	case "TEXT_EQUALS":
		l, r = normalizeText(left.Value), normalizeText(right.Value)
	case "DIGITS_EQUAL":
		l, le = normalizeDigits(left.Value)
		r, re = normalizeDigits(right.Value)
	case "DATE_EQUAL":
		l, le = normalizeDate(left.Value)
		r, re = normalizeDate(right.Value)
	case "NUMBER_EQUAL":
		l, le = normalizeNumber(left.Value, left.Type)
		r, re = normalizeNumber(right.Value, right.Type)
	default:
		return false, invalidValue
	}
	if le != nil || re != nil {
		return false, invalidValue
	}
	if operator == "NUMBER_EQUAL" {
		a, aok := new(big.Rat).SetString(l)
		b, bok := new(big.Rat).SetString(r)
		if !aok || !bok {
			return false, invalidValue
		}
		return a.Cmp(b) == 0, nil
	}
	return l == r, nil
}
func requirementState(req models.Requirement, b repositories.Bundle) (string, bool) {
	if !req.Required && !req.Conditional && req.Override == nil {
		return "NOT_REQUIRED", false
	}
	if req.Override != nil && !*req.Override {
		return "NOT_REQUIRED", false
	}
	if req.Conditional && req.Override == nil {
		if req.Condition == nil {
			return "PENDING_CONDITION", true
		}
		c := req.Condition
		groups := groupCandidates(models.FieldReference{DocumentType: c.DocumentType, Role: c.Role}, b.Groups)
		if len(groups) != 1 {
			return "PENDING_CONDITION", true
		}
		f, ok := findField(b.Fields, groups[0].ID, c.FieldKey)
		if !ok || !trusted(f) {
			return "PENDING_CONDITION", true
		}
		applies := c.Operator == "exists"
		if c.Operator == "equals" {
			actual, e1 := normalizeField(f.Value, f.Type)
			expected, e2 := normalizeField(c.Value, f.Type)
			if e1 != nil || e2 != nil {
				return "PENDING_CONDITION", true
			}
			applies = actual == expected
		}
		if !applies {
			return "NOT_REQUIRED", false
		}
	}
	count := 0
	for _, g := range b.Groups {
		if g.DocumentType == req.DocumentType && (req.Role == "" || g.Role == req.Role) && !g.NeedsConfirmation && (g.Confirmed || g.Confidence >= 0.8) {
			for _, f := range b.Files {
				if f.ID == g.FileID && f.Status == "EXTRACTED" {
					count++
					break
				}
			}
		}
	}
	if count >= req.MinCount {
		return "EXTRACTED", false
	}
	for _, f := range b.Files {
		if f.DocumentType == req.DocumentType && (req.Role == "" || f.Role == req.Role) && f.SourceFileID == nil && f.SupersededBy == nil {
			if f.Status == "PROCESSING" {
				return "PROCESSING", true
			}
			if f.Status == "UPLOADED" {
				return "UPLOADED", true
			}
		}
	}
	return models.Missing, true
}
func (l *DossierLogic) reconcile(ctx context.Context, r *repositories.DossierRepository, b *repositories.Bundle, actorID uuid.UUID) error {
	issues := []models.Issue{}
	for _, rule := range b.Dossier.Snapshot.Definition.Rules {
		i := ruleIssue(rule, *b)
		if i.Status == "PENDING_DOCUMENTS" {
			for _, ref := range []models.FieldReference{rule.Left, rule.Right} {
				if len(groupCandidates(ref, b.Groups)) != 0 {
					continue
				}
				found, ignored := false, true
				for _, req := range b.Requirements {
					if req.DocumentType == ref.DocumentType && (ref.Role == "" || req.Role == ref.Role) {
						found = true
						state, _ := requirementState(req, *b)
						ignored = ignored && state == "NOT_REQUIRED"
					}
				}
				if found && ignored {
					i.Status = "RESOLVED"
					i.Code = "NOT_APPLICABLE"
					i.Message = "Không áp dụng do giấy tờ nguồn không bắt buộc: " + rule.Name
					break
				}
			}
		}
		issues = append(issues, i)
	}
	for _, g := range b.Groups {
		if g.NeedsConfirmation {
			issues = append(issues, models.Issue{RuleCode: "classification:" + g.ID.String(), Code: "CLASSIFICATION_UNCERTAIN", Severity: "WARNING", Status: "OPEN", Message: "Cần xác nhận loại giấy tờ và đối tượng của tài liệu.", Evidence: []models.Evidence{{FileID: g.FileID, GroupID: g.ID}}})
		}
	}
	for _, f := range b.Fields {
		if requiredFieldNeedsReview(f) || (f.Value != "" && !trusted(f)) {
			issues = append(issues, models.Issue{RuleCode: "field:" + f.ID.String(), Code: "FIELD_NEEDS_VERIFICATION", Severity: "WARNING", Status: "OPEN", Message: "Cần xác minh trường: " + f.Label, Remediation: "Kiểm tra bản gốc và sửa dữ liệu kèm lý do nếu cần.", Evidence: []models.Evidence{evidence(f)}})
		}
	}
	for _, g := range b.Groups {
		for _, doc := range b.Dossier.Snapshot.Documents {
			if doc.Code != g.DocumentType {
				continue
			}
			for _, requirement := range doc.Definition.SpecialRequirements {
				var check *models.SpecialCheck
				for index := range b.Checks {
					if b.Checks[index].GroupID == g.ID && b.Checks[index].Code == requirement.Code {
						check = &b.Checks[index]
						break
					}
				}
				i := models.Issue{RuleCode: "special:" + g.ID.String() + ":" + requirement.Code, Code: "SPECIAL_CHECK", Status: "OPEN", Severity: "WARNING", Message: requirement.Description, Evidence: []models.Evidence{{FileID: g.FileID, GroupID: g.ID}}}
				if check != nil {
					i.Evidence = []models.Evidence{{FileID: check.FileID, GroupID: g.ID, Page: check.Page, Value: check.Evidence}}
				}
				if check != nil && check.Confidence >= 0.8 && check.Evidence != "" && requirement.ExternalSource == "" {
					if check.Status == "PASS" {
						i.Status = "RESOLVED"
					} else if check.Status == "FAIL" {
						i.Severity = requirement.Severity
					}
				}
				if requirement.ExternalSource != "" {
					i.Message = "Cần xác minh nguồn ngoài: " + requirement.Description
				}
				issues = append(issues, i)
			}
		}
	}
	old := map[string]models.Issue{}
	for _, i := range b.Issues {
		old[i.RuleCode] = i
	}
	for idx := range issues {
		i := &issues[idx]
		i.CompanyID = b.Dossier.CompanyID
		i.DossierID = b.Dossier.ID
		i.Fingerprint = fingerprint(*i)
		if previous, ok := old[i.RuleCode]; ok {
			i.BaseModel = previous.BaseModel
			if previous.Fingerprint == i.Fingerprint && previous.Status == "ACKNOWLEDGED" && i.Severity == "WARNING" {
				i.Status = previous.Status
				i.AcknowledgedBy = previous.AcknowledgedBy
				i.Acknowledgement = previous.Acknowledgement
			}
			if err := r.Save(ctx, i); err != nil {
				return err
			}
			delete(old, i.RuleCode)
		} else {
			if err := r.Create(ctx, i); err != nil {
				return err
			}
		}
	}
	for _, i := range old {
		if err := r.RetireIssue(ctx, i.ID); err != nil {
			return err
		}
	}
	b.Issues = issues
	missing := false
	for idx := range b.Requirements {
		req := &b.Requirements[idx]
		state, unfilled := requirementState(*req, *b)
		req.Status = state
		if unfilled {
			missing = true
		}
		if err := r.Save(ctx, req); err != nil {
			return err
		}
	}
	pending, failed, blocker := false, false, false
	for _, f := range b.Files {
		if f.SourceFileID == nil && f.SupersededBy == nil {
			pending = pending || f.Status == "UPLOADED" || f.Status == "PROCESSING"
			failed = failed || f.Status == "FAILED"
		}
	}
	for _, i := range b.Issues {
		if i.Status == "OPEN" && i.Severity == "BLOCKER" {
			blocker = true
		}
		if i.Status == "PENDING_DOCUMENTS" {
			missing = true
		}
	}
	next := models.Valid
	if pending {
		next = models.Processing
	} else if failed || blocker || b.Dossier.ChangeRequest != "" {
		next = models.Invalid
	} else if missing {
		next = models.Missing
	}
	if b.Dossier.Status != models.Approved && b.Dossier.Status != models.Rejected {
		return l.setStatus(ctx, r, &b.Dossier, next, actorID)
	}
	return nil
}
func (l *DossierLogic) setStatus(ctx context.Context, r *repositories.DossierRepository, d *models.Dossier, status string, actorID uuid.UUID) error {
	old := d.Status
	d.Status = status
	if old == status {
		return nil
	}
	h := models.StatusHistory{ID: uuid.New(), CompanyID: d.CompanyID, DossierID: d.ID, From: old, To: status, ActorID: actorID, CreatedAt: time.Now().UTC()}
	if err := r.Create(ctx, &h); err != nil {
		return err
	}
	return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: d.CompanyID, ActorID: actorID, Action: "DOSSIER_STATUS_CHANGED", Resource: "dossier", ResourceID: d.ID, Metadata: map[string]any{"from": old, "to": status}})
}
