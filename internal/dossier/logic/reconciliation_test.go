package logic

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"testing"
)

func TestNormalizeValues(t *testing.T) {
	for _, tc := range []struct {
		input, kind, want string
		bad               bool
	}{
		{"079-000-000-001", "identifier", "079000000001", false},
		{"079000000001", "identifier", "079000000001", false},
		{"ABC123", "identifier", "", true},
		{"31/02/2025", "date", "", true}, {"31/10/2026", "date", "2026-10-31", false},
		{"1.200.000 đồng", "money", "1200000", false}, {"1,5 tỷ", "money", "1500000000", false},
		{"1,234.50", "number", "2469/2", false}, {"1.234,50", "number", "2469/2", false},
		{"100,25 m²", "area", "401/4", false}, {"không rõ", "number", "", true},
		{"  NGUYỄN   VĂN A ", "text", "nguyễn văn a", false},
	} {
		t.Run(tc.kind+tc.input, func(t *testing.T) {
			got, err := normalizeField(tc.input, tc.kind)
			if tc.bad {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
func TestRulesDoNotBlockUncertainEvidence(t *testing.T) {
	lID, rID := uuid.New(), uuid.New()
	left := models.Group{DocumentType: "left", Confidence: 1}
	left.ID = lID
	right := models.Group{DocumentType: "right", Confidence: 1}
	right.ID = rID
	lf := models.ExtractedField{GroupID: lID, Key: "id", Type: "identifier", Value: "079000000001", Confidence: 0.99, Validation: "VALID"}
	lf.ID = uuid.New()
	rf := models.ExtractedField{GroupID: rID, Key: "id", Type: "identifier", Value: "079000000002", Confidence: 0.99, Validation: "VALID"}
	rf.ID = uuid.New()
	rule := models.ComparisonRule{Code: "check", Name: "ID match", Left: models.FieldReference{DocumentType: "left", FieldKey: "id"}, Right: models.FieldReference{DocumentType: "right", FieldKey: "id"}, Operator: "DIGITS_EQUAL", Severity: "BLOCKER"}
	for _, tc := range []struct {
		name                   string
		mutate                 func(*repositories.Bundle, *models.ComparisonRule)
		status, severity, code string
	}{
		{"mismatch", nil, "OPEN", "BLOCKER", "MISMATCH"},
		{"missing document", func(b *repositories.Bundle, _ *models.ComparisonRule) { b.Groups = b.Groups[:1] }, "PENDING_DOCUMENTS", "BLOCKER", "PENDING_DOCUMENTS"},
		{"low confidence", func(b *repositories.Bundle, _ *models.ComparisonRule) { b.Fields[1].Confidence = 0.4 }, "OPEN", "WARNING", "NEEDS_VERIFICATION"},
		{"invalid dates never equal", func(b *repositories.Bundle, r *models.ComparisonRule) {
			r.Operator = "DATE_EQUAL"
			b.Fields[0].Value = "not-a-date"
			b.Fields[1].Value = "not-a-date"
		}, "OPEN", "WARNING", "NEEDS_VERIFICATION"},
		{"multiple people without join", func(b *repositories.Bundle, _ *models.ComparisonRule) {
			extra := left
			extra.ID = uuid.New()
			b.Groups = append(b.Groups, extra)
		}, "OPEN", "WARNING", "AMBIGUOUS_SUBJECT"},
		{"match", func(b *repositories.Bundle, _ *models.ComparisonRule) { b.Fields[1].Value = b.Fields[0].Value }, "RESOLVED", "BLOCKER", "MATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := repositories.Bundle{Groups: []models.Group{left, right}, Fields: []models.ExtractedField{lf, rf}}
			r := rule
			if tc.mutate != nil {
				tc.mutate(&b, &r)
			}
			out := ruleIssue(r, b)
			assert.Equal(t, tc.status, out.Status)
			assert.Equal(t, tc.severity, out.Severity)
			assert.Equal(t, tc.code, out.Code)
		})
	}
}

func TestRulesJoinMultipleSubjectsOneToOne(t *testing.T) {
	b := repositories.Bundle{}
	for _, kind := range []string{"left", "right"} {
		for _, id := range []string{"001", "002"} {
			g := models.Group{DocumentType: kind, Confidence: 1}
			g.ID = uuid.New()
			b.Groups = append(b.Groups, g)
			b.Fields = append(b.Fields, models.ExtractedField{GroupID: g.ID, Key: "subject", Type: "identifier", Value: id, Validation: "VALID", Confidence: 1}, models.ExtractedField{GroupID: g.ID, Key: "name", Type: "text", Value: "Synthetic " + id, Validation: "VALID", Confidence: 1})
		}
	}
	rule := models.ComparisonRule{Name: "All matching people", Operator: "TEXT_EQUALS", Severity: "BLOCKER", Left: models.FieldReference{DocumentType: "left", FieldKey: "name", SubjectField: "subject"}, Right: models.FieldReference{DocumentType: "right", FieldKey: "name", SubjectField: "subject"}}
	out := ruleIssue(rule, b)
	assert.Equal(t, "MATCH", out.Code)
	assert.Len(t, out.Evidence, 4)
	b.Fields[len(b.Fields)-1].Value = "Different synthetic name"
	assert.Equal(t, "MISMATCH", ruleIssue(rule, b).Code)
	b.Fields[len(b.Fields)-2].Value = "001" // duplicate subject; never choose one matching pair arbitrarily
	out = ruleIssue(rule, b)
	assert.Equal(t, "AMBIGUOUS_SUBJECT", out.Code)
	assert.Equal(t, "WARNING", out.Severity)
}
