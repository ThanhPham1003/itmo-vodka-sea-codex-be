package logic

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"testing"
)

func TestRequiredFieldHumanAcceptance(t *testing.T) {
	id, reviewer := uuid.New(), uuid.New()
	field := models.ExtractedField{Required: true, Validation: "MISSING"}
	field.ID = id
	issue := models.Issue{RuleCode: "field:" + id.String(), Code: "FIELD_NEEDS_VERIFICATION", Severity: "WARNING", Status: "ACKNOWLEDGED", AcknowledgedBy: &reviewer, Acknowledgement: "Checked the original and accepted", Evidence: []models.Evidence{{FieldID: id}}}
	for _, tc := range []struct {
		name   string
		change func(*models.ExtractedField, *models.Issue)
		want   int
	}{
		{"accepted missing field", nil, 0},
		{"accepted invalid format", func(f *models.ExtractedField, _ *models.Issue) {
			f.Value = "not a date"
			f.Validation = "INVALID_FORMAT"
		}, 0},
		{"unconfirmed", func(_ *models.ExtractedField, i *models.Issue) { i.Status = "OPEN" }, 1},
		{"unrelated warning", func(_ *models.ExtractedField, i *models.Issue) { i.RuleCode = "field:" + uuid.NewString() }, 1},
		{"wrong evidence", func(_ *models.ExtractedField, i *models.Issue) { i.Evidence = []models.Evidence{{FieldID: uuid.New()}} }, 1},
		{"not a field warning", func(_ *models.ExtractedField, i *models.Issue) { i.Code = "SPECIAL_CHECK" }, 1},
		{"blocker is not acceptance", func(_ *models.ExtractedField, i *models.Issue) { i.Severity = "BLOCKER" }, 1},
		{"missing reviewer", func(_ *models.ExtractedField, i *models.Issue) { i.AcknowledgedBy = nil }, 1},
		{"missing reason", func(_ *models.ExtractedField, i *models.Issue) { i.Acknowledgement = " " }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, i := field, issue
			if tc.change != nil {
				tc.change(&f, &i)
			}
			remaining, accepted := requiredFieldReview(repositories.Bundle{Fields: []models.ExtractedField{f}, Issues: []models.Issue{i}})
			assert.Equal(t, tc.want, remaining)
			assert.Len(t, accepted, 1-tc.want)
			assert.Equal(t, "MISSING", field.Validation)
		})
	}
}
