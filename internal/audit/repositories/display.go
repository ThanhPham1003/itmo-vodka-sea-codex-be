package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"sea-codex-server/internal/audit/models"
	am "sea-codex-server/internal/auth/models"
	dm "sea-codex-server/internal/dossier/models"
)

// Batch by resource kind within the visible audit page. Every lookup is scoped
// to the event company; missing or foreign references never expose their names.
func (r *AuditRepository) resolveResources(ctx context.Context, company uuid.UUID, events []models.AuditLog) error {
	ids := map[string][]uuid.UUID{}
	for _, e := range events {
		ids[e.Resource] = append(ids[e.Resource], e.ResourceID)
		if e.DossierID != nil {
			ids["dossier"] = append(ids["dossier"], *e.DossierID)
		}
	}
	names := map[string]map[uuid.UUID]string{}
	for kind, keys := range ids {
		names[kind] = map[uuid.UUID]string{}
		q := r.db.WithContext(ctx).Unscoped().Where("company_id = ? AND id IN ?", company, keys)
		switch kind {
		case "dossier":
			var rows []dm.Dossier
			if err := q.Select("id", "title").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				names[kind][row.ID] = row.Title
			}
		case "file":
			var rows []dm.File
			if err := q.Select("id", "original_name").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				names[kind][row.ID] = row.OriginalName
			}
		case "document_type":
			var rows []dm.DocumentType
			if err := q.Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				names[kind][row.ID] = row.Definition.Name
			}
		case "dossier_type":
			var rows []dm.DossierType
			if err := q.Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				names[kind][row.ID] = row.Definition.Name
			}
		case "user":
			var rows []am.User
			if err := q.Select("id", "full_name", "username").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				name := strings.TrimSpace(row.FullName)
				if name == "" {
					name = row.Username
				}
				names[kind][row.ID] = name
			}
		case "company":
			var rows []am.Company
			if err := r.db.WithContext(ctx).Unscoped().Where("id = ? AND id IN ?", company, keys).Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				names[kind][row.ID] = row.Name
			}
		}
	}
	for i := range events {
		e := &events[i]
		e.ResourceName = names[e.Resource][e.ResourceID]
		if e.DossierID != nil {
			e.DossierTitle = names["dossier"][*e.DossierID]
		}
	}
	return nil
}
