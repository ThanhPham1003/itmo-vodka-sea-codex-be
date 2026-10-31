package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/pkg/ai"
	"sea-codex-server/pkg/config"
)

func TestDocumentSuggestionStructuredOutput(t *testing.T) {
	definition := `{"name":"Draft","description":"Example","fields":[{"key":"","label":"Document number","type":"identifier","required":true,"pii":true,"pattern":""}],"special_requirements":[{"code":"","description":"Review","severity":"WARNING","external_source":""}]}`
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ResponseFormat struct {
				Type       string `json:"type"`
				JSONSchema struct {
					Name   string          `json:"name"`
					Strict bool            `json:"strict"`
					Schema json.RawMessage `json:"schema"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if calls.Add(1) == 1 {
			assert.Equal(t, "json_schema", payload.ResponseFormat.Type)
			assert.True(t, payload.ResponseFormat.JSONSchema.Strict)
			assert.JSONEq(t, documentSuggestionSchema, string(payload.ResponseFormat.JSONSchema.Schema))
		} else {
			assert.Equal(t, "json_object", payload.ResponseFormat.Type, "ordinary completions keep their existing format")
			assert.Empty(t, payload.ResponseFormat.JSONSchema.Schema)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": definition}}}})
	}))
	defer server.Close()
	client := ai.NewClient(config.ProcessingConfig{Provider: "openai-compatible", BaseURL: server.URL, APIKey: "test", Model: "test", Timeout: time.Second})
	l := NewCatalogLogic(nil, suggestionAudit{}, client, zerolog.Nop())
	ctx := common.WithActor(context.Background(), common.Actor{UserID: uuid.New(), CompanyID: uuid.New(), Role: common.RoleCompanyAdmin})
	raw, err := l.SuggestDocument(ctx, "An example document")
	require.NoError(t, err)
	var draft struct {
		Draft      bool                      `json:"draft"`
		Definition models.DocumentDefinition `json:"definition"`
	}
	require.NoError(t, json.Unmarshal(raw, &draft))
	assert.True(t, draft.Draft)
	require.Len(t, draft.Definition.Fields, 1)
	assert.Empty(t, draft.Definition.Fields[0].Key)
	require.Len(t, draft.Definition.SpecialRequirements, 1)
	assert.Empty(t, draft.Definition.SpecialRequirements[0].Code)
	_, err = client.Complete(ctx, "Return JSON", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 2, calls.Load())
}

func TestDocumentSuggestionSchemaMatchesDefinition(t *testing.T) {
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(documentSuggestionSchema), &schema))
	var check func(map[string]any, reflect.Type)
	check = func(node map[string]any, typ reflect.Type) {
		switch typ.Kind() {
		case reflect.Struct:
			assert.Equal(t, "object", node["type"])
			assert.Equal(t, false, node["additionalProperties"])
			properties, ok := node["properties"].(map[string]any)
			require.True(t, ok)
			required, ok := node["required"].([]any)
			require.True(t, ok)
			require.Len(t, properties, typ.NumField())
			require.Len(t, required, typ.NumField())
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
				assert.Contains(t, required, name)
				child, ok := properties[name].(map[string]any)
				require.True(t, ok, name)
				check(child, field.Type)
				if name == "key" || name == "code" {
					assert.Equal(t, []any{""}, child["enum"], "draft identifiers must be empty")
				}
			}
		case reflect.Slice:
			assert.Equal(t, "array", node["type"])
			items, ok := node["items"].(map[string]any)
			require.True(t, ok)
			check(items, typ.Elem())
		case reflect.String:
			assert.Equal(t, "string", node["type"])
		case reflect.Bool:
			assert.Equal(t, "boolean", node["type"])
		default:
			t.Fatalf("unhandled schema type %s", typ)
		}
	}
	check(schema, reflect.TypeFor[models.DocumentDefinition]())
}

func TestDecodeFailureDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, raw, kind, field, expected, actual string
	}{
		{"wrapper", `{"definition":{"name":"private-content"}}`, "unknown_field", "definition", "", ""},
		{"unknown private key", `{"private-content":"private-key"}`, "unknown_field", "", "", ""},
		{"boolean string", `{"fields":[{"required":"private-content"}]}`, "type_mismatch", "fields.required", "boolean", "string"},
		{"requirements strings", `{"special_requirements":["private-content"]}`, "type_mismatch", "special_requirements", "object", "string"},
		{"root array", `[]`, "type_mismatch", "$", "object", "array"},
		{"number value omitted", `{"fields":[{"label":123456789}]}`, "type_mismatch", "fields.label", "string", "number"},
		{"invalid syntax", `{"fields":`, "invalid_document", "", "", ""},
		{"empty", ``, "empty_response", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var def models.DocumentDefinition
			err := strictJSON([]byte(tc.raw), &def)
			require.Error(t, err)
			var buf bytes.Buffer
			l := NewCatalogLogic(nil, nil, nil, zerolog.New(&buf))
			l.logSuggestionFailure(context.Background(), "document_type", "decode", err)
			var entry map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
			assert.Equal(t, tc.kind, entry["decode_error"])
			for key, value := range map[string]string{"json_field": tc.field, "expected_type": tc.expected, "actual_type": tc.actual} {
				if value == "" {
					assert.NotContains(t, entry, key)
				} else {
					assert.Equal(t, value, entry[key])
				}
			}
			for _, private := range []string{"private-content", "private-key", "123456789"} {
				assert.NotContains(t, buf.String(), private)
			}
		})
	}
}
