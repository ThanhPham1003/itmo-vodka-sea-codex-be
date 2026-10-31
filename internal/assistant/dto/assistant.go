package dto

type Page struct {
	Name string `json:"name"`
	Tab  string `json:"tab"`
}
type Message struct {
	Role    string `json:"role" binding:"required,oneof=user assistant"`
	Content string `json:"content" binding:"required,min=1,max=2000"`
}
type ChatRequest struct {
	Message string    `json:"message" binding:"required,min=1,max=2000"`
	Locale  string    `json:"locale" binding:"required,oneof=vi en"`
	Page    Page      `json:"page"`
	History []Message `json:"history" binding:"max=8,dive"`
}
type Block struct {
	Text      string   `json:"text"`
	SourceIDs []string `json:"source_ids"`
}
type Source struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Excerpt     string `json:"excerpt"`
	ArticlePath string `json:"article_path"`
}
type ChatResponse struct {
	Status           string   `json:"status"`
	Blocks           []Block  `json:"blocks"`
	Sources          []Source `json:"sources"`
	Suggestions      []string `json:"suggestions"`
	KnowledgeVersion string   `json:"knowledge_version"`
	RequestID        string   `json:"request_id"`
}
type ArticleResponse struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	KnowledgeVersion string `json:"knowledge_version"`
}
type ContextResponse struct {
	Enabled          bool     `json:"enabled"`
	KnowledgeVersion string   `json:"knowledge_version"`
	SupportedLocales []string `json:"supported_locales"`
	Suggestions      []string `json:"suggestions"`
	Articles         []Source `json:"articles"`
}
