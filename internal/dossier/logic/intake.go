package logic

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"sea-codex-server/pkg/storage"
)

func safeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	r := []rune(name)
	if len(r) > 200 {
		name = string(r[:200])
	}
	if name == "" || name == "." {
		name = "document"
	}
	return name
}
func (l *DossierLogic) Upload(ctx context.Context, id uuid.UUID, inputs []dto.UploadInput) (dto.UploadResult, error) {
	a, err := actor(ctx, "dossier:upload")
	out := dto.UploadResult{BatchID: uuid.New(), Files: []dto.UploadedFile{}}
	if err != nil {
		return out, err
	}
	if len(inputs) == 0 || len(inputs) > l.cfg.Uploads.MaxFilesPerRequest {
		return out, common.ErrInvalidArgument
	}
	d, err := l.repo.Scope(a.CompanyID).Find(ctx, id, false)
	if err != nil {
		return out, repositories.Error(err)
	}
	if !canRead(a, d) {
		return out, common.ErrNotFound
	}
	if !editable(d) {
		return out, common.ErrConflict
	}
	for _, in := range inputs {
		item := dto.UploadedFile{Name: safeName(in.Name), Status: "REJECTED"}
		if in.DocumentType != "" {
			if _, ok := pinnedDocument(d, in.DocumentType); !ok {
				item.Error = "DOCUMENT_TYPE_NOT_ALLOWED"
				out.Files = append(out.Files, item)
				continue
			}
		}
		if !validRole(d, in.DocumentType, in.Role) {
			item.Error = "ROLE_NOT_ALLOWED"
			out.Files = append(out.Files, item)
			continue
		}
		reader, err := in.Open()
		if err != nil {
			item.Error = "FILE_UNREADABLE"
			out.Files = append(out.Files, item)
			continue
		}
		object, err := l.storage.Put(ctx, reader, l.cfg.Uploads.MaxFileSizeMB*1024*1024)
		reader.Close()
		if err != nil {
			item.Error = "STORAGE_UNAVAILABLE"
			if errors.Is(err, storage.ErrInvalidFile) {
				item.Error = "UNSUPPORTED_OR_OVERSIZED_FILE"
			}
			out.Files = append(out.Files, item)
			continue
		}
		file := models.File{CompanyID: a.CompanyID, DossierID: id, BatchID: out.BatchID, OriginalName: item.Name, StorageKey: object.Key, SHA256: object.SHA256, MIME: object.MIME, Size: object.Size, DocumentType: in.DocumentType, Role: in.Role, Status: "UPLOADED", Pages: []int{}}
		file.ID = uuid.New()
		var duplicate *uuid.UUID
		err = l.mutate(ctx, id, "dossier:upload", 0, "FILE_UPLOADED", map[string]any{"file_id": file.ID, "batch_id": out.BatchID}, func(r *repositories.DossierRepository, b *repositories.Bundle, a common.Actor) error {
			existing, err := r.Duplicate(ctx, id, object.SHA256)
			if err == nil {
				duplicate = &existing.ID
				return common.ErrConflict
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err = r.Create(ctx, &file); err != nil {
				return err
			}
			if in.ReplacesFileID != nil {
				found := false
				for _, old := range b.Files {
					if old.ID == *in.ReplacesFileID && old.SourceFileID == nil && old.SupersededBy == nil {
						found = true
						old.SupersededBy = &file.ID
						old.ClaimID = nil
						old.LeaseUntil = nil
						if err = r.Save(ctx, &old); err != nil {
							return err
						}
						if err = r.RemoveProcessingData(ctx, id, old.ID); err != nil {
							return err
						}
						break
					}
				}
				if !found {
					return common.ErrNotFound
				}
			}
			return nil
		})
		if err != nil {
			_ = l.storage.Delete(object.Key)
			if duplicate != nil {
				item.ID = duplicate
				item.Status = "DUPLICATE"
				item.Error = "DUPLICATE_FILE"
			} else {
				item.Error = "UPLOAD_CONFLICT"
			}
			out.Files = append(out.Files, item)
			continue
		}
		item.ID = &file.ID
		item.Status = "UPLOADED"
		out.Files = append(out.Files, item)
	}
	return out, nil
}
func (l *DossierLogic) Retry(ctx context.Context, id, fileID uuid.UUID, version uint64) error {
	return l.mutate(ctx, id, "dossier:upload", version, "FILE_RETRIED", map[string]any{"file_id": fileID}, func(r *repositories.DossierRepository, b *repositories.Bundle, a common.Actor) error {
		for _, f := range b.Files {
			if f.ID == fileID {
				if f.SourceFileID != nil || f.SupersededBy != nil || f.Status != "FAILED" {
					return common.ErrConflict
				}
				f.Status = "UPLOADED"
				f.ClaimID = nil
				f.LeaseUntil = nil
				f.ErrorCode = ""
				f.ErrorMessage = ""
				if err := r.RemoveProcessingData(ctx, id, f.ID); err != nil {
					return err
				}
				return r.Save(ctx, &f)
			}
		}
		return common.ErrNotFound
	})
}
func (l *DossierLogic) Classify(ctx context.Context, id, fileID uuid.UUID, in dto.ClassificationRequest) error {
	return l.mutate(ctx, id, "dossier:correct", in.ExpectedVersion, "FILE_CLASSIFIED", map[string]any{"file_id": fileID, "document_type": in.DocumentType, "document_group_id": in.GroupID}, func(r *repositories.DossierRepository, b *repositories.Bundle, a common.Actor) error {
		if _, ok := pinnedDocument(b.Dossier, in.DocumentType); !ok || !validRole(b.Dossier, in.DocumentType, in.Role) {
			return common.ErrInvalidArgument
		}
		var source *models.File
		for _, f := range b.Files {
			if f.ID == fileID {
				copy := f
				source = &copy
				break
			}
		}
		if source == nil {
			return common.ErrNotFound
		}
		if source.SourceFileID != nil {
			for _, f := range b.Files {
				if f.ID == *source.SourceFileID {
					copy := f
					source = &copy
					break
				}
			}
		}
		if source.SupersededBy != nil {
			return common.ErrConflict
		}
		pages := []int{}
		groups := 0
		for _, g := range b.Groups {
			belongs := g.FileID == source.ID
			for _, f := range b.Files {
				if f.ID == g.FileID && f.SourceFileID != nil && *f.SourceFileID == source.ID {
					belongs = true
				}
			}
			if belongs {
				groups++
				if in.GroupID == nil || g.ID == *in.GroupID {
					pages = append(pages, g.Pages...)
				}
			}
		}
		if in.GroupID != nil && len(pages) == 0 {
			return common.ErrNotFound
		}
		if in.GroupID == nil && groups > 1 {
			return common.ErrInvalidArgument
		}
		if len(pages) == 0 {
			pages = []int{1}
		}
		preserved := []models.PreservedCorrection{}
		for _, entry := range source.PreservedCorrections {
			overlap := false
			for _, oldPage := range entry.Pages {
				for _, page := range pages {
					overlap = overlap || oldPage == page
				}
			}
			if !overlap {
				preserved = append(preserved, entry)
			}
		}
		source.PreservedCorrections = preserved
		overrides := []models.ClassificationOverride{}
		for _, old := range source.Overrides {
			kept := []int{}
			for _, p := range old.Pages {
				replace := false
				for _, page := range pages {
					replace = replace || p == page
				}
				if !replace {
					kept = append(kept, p)
				}
			}
			if len(kept) > 0 {
				old.Pages = kept
				overrides = append(overrides, old)
			}
		}
		source.Overrides = append(overrides, models.ClassificationOverride{Pages: pages, DocumentType: in.DocumentType, Role: in.Role})
		if in.GroupID == nil {
			source.DocumentType = in.DocumentType
			source.Role = in.Role
		}
		source.Status = "UPLOADED"
		source.ClaimID = nil
		source.LeaseUntil = nil
		source.ErrorCode = ""
		source.ErrorMessage = ""
		if err := r.RemoveProcessingData(ctx, id, source.ID); err != nil {
			return err
		}
		return r.Save(ctx, source)
	})
}
