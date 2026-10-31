package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"sea-codex-server/internal/assistant/knowledge"
	"sea-codex-server/internal/assistant/models"
	"sea-codex-server/internal/common"
)

type IKnowledgeRepository interface {
	List(context.Context) ([]models.Article, error)
	Get(context.Context, string) (models.Article, error)
	Search(context.Context, string, string) ([]models.Article, error)
}

type KnowledgeRepository struct{ articles []models.Article }

func NewKnowledgeRepository() (*KnowledgeRepository, error) {
	entries, err := knowledge.Files.ReadDir("articles")
	if err != nil {
		return nil, fmt.Errorf("read embedded knowledge: %w", err)
	}
	r := &KnowledgeRepository{}
	seen := map[string]bool{}
	for _, entry := range entries {
		raw, err := knowledge.Files.ReadFile("articles/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read knowledge article: %w", err)
		}
		// JSON metadata first, followed by the actual localized Markdown bodies.
		var a models.Article
		parts := strings.SplitN(string(raw), "\n", 2)
		if len(parts) != 2 || json.Unmarshal([]byte(parts[0]), &a) != nil || a.ID == "" || seen[a.ID] || a.Version != models.KnowledgeVersion || a.Revision == "" || len(a.Sources) == 0 {
			return nil, fmt.Errorf("invalid embedded knowledge article: %s", entry.Name())
		}
		vi, en, ok := strings.Cut(parts[1], "\n\n## en\n\n")
		if !ok || !strings.HasPrefix(vi, "\n## vi\n\n") {
			return nil, fmt.Errorf("invalid knowledge sections: %s", entry.Name())
		}
		for locale, content := range map[string]string{"vi": strings.TrimPrefix(vi, "\n## vi\n\n"), "en": en} {
			t := a.Translations[locale]
			t.Content = strings.TrimSpace(content)
			a.Translations[locale] = t
		}
		for _, locale := range []string{"vi", "en"} {
			if a.Translations[locale].Title == "" || a.Translations[locale].Content == "" {
				return nil, fmt.Errorf("missing knowledge translation: %s", entry.Name())
			}
		}
		seen[a.ID] = true
		r.articles = append(r.articles, a)
	}
	if len(r.articles) == 0 {
		return nil, fmt.Errorf("empty embedded knowledge")
	}
	return r, nil
}

// Deep copies prevent callers from changing the process-wide embedded corpus.
func copyArticle(a models.Article) models.Article {
	a.Aliases = append([]string(nil), a.Aliases...)
	a.Pages = append([]string(nil), a.Pages...)
	a.Sources = append([]string(nil), a.Sources...)
	translations := make(map[string]models.Translation, len(a.Translations))
	for k, v := range a.Translations {
		translations[k] = v
	}
	a.Translations = translations
	return a
}
func (r *KnowledgeRepository) List(ctx context.Context) ([]models.Article, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	list := make([]models.Article, len(r.articles))
	for i, a := range r.articles {
		list[i] = copyArticle(a)
	}
	return list, nil
}
func (r *KnowledgeRepository) Get(ctx context.Context, id string) (models.Article, error) {
	if err := ctx.Err(); err != nil {
		return models.Article{}, err
	}
	for _, a := range r.articles {
		if a.ID == id {
			return copyArticle(a), nil
		}
	}
	return models.Article{}, common.ErrNotFound
}

func Normalize(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "đ", "d")
	return strings.Join(strings.FieldsFunc(norm.NFD.String(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.Is(unicode.Mn, r) }), " ")
}

func words(s string) map[string]bool {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, Normalize(s))
	stop := " a an the i me my is are do does to of for in on it that this how why can cannot with and or what where when you toi cua la co khong tai sao nhu nao gi o dau thi va voi duoc cho mot cac nhung nay do he thong ho so "
	result := map[string]bool{}
	for _, w := range strings.Fields(s) {
		if len(w) > 1 && !strings.Contains(stop, " "+w+" ") {
			result[w] = true
		}
	}
	return result
}

// Fold is also used by deterministic scope checks, without language-specific rules.
func Fold(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, Normalize(s))
}

func (r *KnowledgeRepository) Search(ctx context.Context, query, page string) ([]models.Article, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q := words(query)
	if len(q) == 0 {
		return []models.Article{}, nil
	}
	type candidate struct {
		article models.Article
		score   float64
	}
	var candidates []candidate
	documents := make([]map[string]bool, len(r.articles))
	df := map[string]int{}
	for i, a := range r.articles {
		text := a.Translations["vi"].Title + " " + a.Translations["en"].Title + " " + strings.Join(a.Aliases, " ") + " " + a.Translations["vi"].Content + " " + a.Translations["en"].Content
		documents[i] = words(text)
		for w := range documents[i] {
			df[w]++
		}
	}
	for i, a := range r.articles {
		score := 0.0
		for w := range q {
			if documents[i][w] {
				score += math.Log(1 + float64(len(r.articles))/float64(df[w]))
			}
		}
		for _, alias := range a.Aliases {
			if len(Fold(alias)) >= 4 && strings.Contains(" "+Fold(query)+" ", " "+Fold(alias)+" ") {
				score += 8
			}
		}
		if score == 0 {
			continue
		}
		for _, p := range a.Pages {
			if p == page {
				score += 0.5
				break
			}
		}
		candidates = append(candidates, candidate{a, score})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	result := []models.Article{}
	budget := 0
	for _, c := range candidates {
		if c.score < 1.5 || (len(candidates) > 0 && c.score < candidates[0].score*0.35) {
			continue
		}
		n := utf8.RuneCountInString(c.article.Translations["vi"].Content) + utf8.RuneCountInString(c.article.Translations["en"].Content)
		if budget+n > 16000 {
			continue
		}
		result = append(result, copyArticle(c.article))
		budget += n
		if len(result) == 5 {
			break
		}
	}
	return result, nil
}
