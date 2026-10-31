package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	am "sea-codex-server/internal/auth/models"
	"sea-codex-server/internal/dossier/models"
)

// Read-time labels only. IDs remain stable and no display fields are persisted.
func (r *DossierRepository) ResolvePeople(ctx context.Context, dossiers []models.Dossier, decisions []models.Decision) error {
	ids := []uuid.UUID{}
	for _, d := range dossiers {
		ids = append(ids, d.CreatorID)
		if d.AssigneeID != nil {
			ids = append(ids, *d.AssigneeID)
		}
	}
	for _, d := range decisions {
		ids = append(ids, d.ActorID)
	}
	if len(ids) == 0 {
		return nil
	}
	var users []am.User
	if err := r.query(ctx, &am.User{}).Unscoped().Select("id", "full_name", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return err
	}
	names := map[uuid.UUID]string{}
	for _, u := range users {
		name := strings.TrimSpace(u.FullName)
		if name == "" {
			name = u.Username
		}
		names[u.ID] = name
	}
	for i := range dossiers {
		dossiers[i].CreatorName = names[dossiers[i].CreatorID]
		if dossiers[i].AssigneeID != nil {
			dossiers[i].AssigneeName = names[*dossiers[i].AssigneeID]
		}
	}
	for i := range decisions {
		decisions[i].ActorName = names[decisions[i].ActorID]
	}
	return nil
}

// Resolve one page in a single query. Names and field labels come from each
// requirement's pinned version, including archived document types.
func (r *DossierRepository) ResolveCatalogSchemas(ctx context.Context, items []models.DossierType) error {
	type pair struct {
		code    string
		version int
	}
	wanted := map[pair]bool{}
	codes, versions := []string{}, []int{}
	for _, item := range items {
		for _, req := range item.Definition.Documents {
			p := pair{req.DocumentType, req.SchemaVersion}
			if !wanted[p] {
				wanted[p] = true
				codes = append(codes, p.code)
				versions = append(versions, p.version)
			}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	type row struct {
		models.DocumentTypeVersion
		Code string
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("company_document_type_versions AS v").
		Select("v.*, d.code").
		Joins("JOIN company_document_types AS d ON d.id = v.document_type_id AND d.company_id = v.company_id").
		Where("v.company_id = ? AND d.code IN ? AND v.version IN ?", r.company, codes, versions).Scan(&rows).Error
	if err != nil {
		return err
	}
	resolved := map[pair]models.PinnedDocument{}
	for _, v := range rows {
		p := pair{v.Code, v.Version}
		if wanted[p] {
			resolved[p] = models.PinnedDocument{Code: v.Code, Version: v.Version, Definition: v.Definition}
		}
	}
	for i := range items {
		items[i].DocumentSchemas = []models.PinnedDocument{}
		seen := map[pair]bool{}
		for _, req := range items[i].Definition.Documents {
			p := pair{req.DocumentType, req.SchemaVersion}
			if doc, ok := resolved[p]; ok && !seen[p] {
				items[i].DocumentSchemas = append(items[i].DocumentSchemas, doc)
				seen[p] = true
			}
		}
	}
	return nil
}
