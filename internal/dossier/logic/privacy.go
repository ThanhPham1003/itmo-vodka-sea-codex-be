package logic

import (
	"context"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"sea-codex-server/internal/dossier/services"
)

func mask(value string) string {
	r := []rune(value)
	if len(r) < 8 {
		return strings.Repeat("*", len(r))
	}
	return string(r[:3]) + strings.Repeat("*", len(r)-7) + string(r[len(r)-4:])
}
func maskSource(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return '*'
		}
		return r
	}, value)
}
func redacted(b repositories.Bundle) dto.Detail {
	for i := range b.Fields {
		f := &b.Fields[i]
		if f.PII {
			f.Value = mask(f.Value)
			f.NormalizedValue = mask(f.NormalizedValue)
		}
		f.RawText = maskSource(f.RawText)
	}
	for i := range b.Issues {
		for j := range b.Issues[i].Evidence {
			b.Issues[i].Evidence[j].Value = maskSource(b.Issues[i].Evidence[j].Value)
		}
	}
	return dto.Detail{Dossier: b.Dossier, Requirements: b.Requirements, Files: b.Files, Groups: b.Groups, Fields: b.Fields, Issues: b.Issues, Decisions: b.Decisions}
}
func (l *DossierLogic) Reveal(ctx context.Context, id uuid.UUID, in dto.RevealRequest) ([]models.ExtractedField, error) {
	a, err := actor(ctx, "pii:view")
	if err != nil {
		return nil, err
	}
	if a.Role == "broker" || len(in.FieldIDs) == 0 || len(in.FieldIDs) > 100 {
		return nil, common.ErrForbidden
	}
	b, err := l.repo.Scope(a.CompanyID).Bundle(ctx, id, false)
	if err != nil {
		return nil, repositories.Error(err)
	}
	if !canRead(a, b.Dossier) {
		return nil, common.ErrNotFound
	}
	wanted := map[uuid.UUID]bool{}
	for _, v := range in.FieldIDs {
		wanted[v] = true
	}
	out := []models.ExtractedField{}
	for _, f := range b.Fields {
		if wanted[f.ID] {
			out = append(out, f)
			delete(wanted, f.ID)
		}
	}
	if len(wanted) != 0 {
		return nil, common.ErrNotFound
	}
	err = l.audit.Write(ctx, nil, common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "VIEW_PII", Resource: "dossier", ResourceID: id, Metadata: map[string]any{"field_ids": in.FieldIDs}})
	if err != nil {
		return nil, repositories.Error(err)
	}
	return out, nil
}
func (l *DossierLogic) FileContent(ctx context.Context, id, fileID uuid.UUID) (services.Content, error) {
	a, err := actor(ctx, "dossier:image_view")
	if err != nil {
		return services.Content{}, err
	}
	if a.Role == "broker" {
		return services.Content{}, common.ErrForbidden
	}
	b, err := l.repo.Scope(a.CompanyID).Bundle(ctx, id, false)
	if err != nil {
		return services.Content{}, repositories.Error(err)
	}
	if !canRead(a, b.Dossier) {
		return services.Content{}, common.ErrNotFound
	}
	for _, f := range b.Files {
		if f.ID == fileID {
			file, err := l.storage.Open(ctx, f.StorageKey)
			if err != nil {
				return services.Content{}, common.ErrUnavailable
			}
			if err = l.audit.Write(ctx, nil, common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "VIEW_FILE", Resource: "file", ResourceID: f.ID, Metadata: map[string]any{"dossier_id": id}}); err != nil {
				file.Close()
				return services.Content{}, repositories.Error(err)
			}
			return services.Content{File: file, Name: f.OriginalName, MIME: f.MIME, Size: f.Size}, nil
		}
	}
	return services.Content{}, common.ErrNotFound
}
