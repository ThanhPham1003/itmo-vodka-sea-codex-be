package logic

import (
	"github.com/google/uuid"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"strings"
)

func requiredFieldNeedsReview(field models.ExtractedField) bool {
	return field.Required && (strings.TrimSpace(field.Value) == "" || field.Validation != "VALID")
}

// Called after reconcile, which preserves acknowledgement only for unchanged
// evidence. Human acceptance does not alter OCR values or validation results.
func requiredFieldReview(b repositories.Bundle) (unaccepted int, accepted []uuid.UUID) {
	accepted = []uuid.UUID{}
	for _, field := range b.Fields {
		if !requiredFieldNeedsReview(field) {
			continue
		}
		confirmed := false
		for _, issue := range b.Issues {
			if issue.RuleCode != "field:"+field.ID.String() || issue.Code != "FIELD_NEEDS_VERIFICATION" || issue.Severity != "WARNING" || issue.Status != "ACKNOWLEDGED" || issue.AcknowledgedBy == nil || *issue.AcknowledgedBy == uuid.Nil || strings.TrimSpace(issue.Acknowledgement) == "" {
				continue
			}
			for _, source := range issue.Evidence {
				if source.FieldID == field.ID {
					confirmed = true
					break
				}
			}
			if confirmed {
				break
			}
		}
		if confirmed {
			accepted = append(accepted, field.ID)
		} else {
			unaccepted++
		}
	}
	return unaccepted, accepted
}
