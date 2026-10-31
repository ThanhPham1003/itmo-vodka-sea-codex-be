// Package models defines immutable knowledge values, not persisted GORM entities.
package models

const KnowledgeVersion = "2026-10-31.v3"

type Translation struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}
type Article struct {
	ID           string                 `json:"id"`
	Version      string                 `json:"version"`
	Aliases      []string               `json:"aliases"`
	Pages        []string               `json:"pages"`
	Sources      []string               `json:"sources"`
	Revision     string                 `json:"revision"`
	Translations map[string]Translation `json:"translations"`
}
