package logic

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

func addDecodeDiagnostics(event *zerolog.Event, err error) {
	var mismatch *json.UnmarshalTypeError
	var syntax *json.SyntaxError
	switch {
	case errors.As(err, &mismatch):
		event.Str("decode_error", "type_mismatch").Str("expected_type", jsonKind(mismatch.Type.Kind()))
		// Value may contain the actual number. Retain only the JSON type.
		actual := strings.SplitN(mismatch.Value, " ", 2)[0]
		switch actual {
		case "array", "object", "string", "number", "bool":
			event.Str("actual_type", actual)
		}
		if mismatch.Field == "" {
			event.Str("json_field", "$")
		} else if diagnosticJSONPath(mismatch.Field) {
			event.Str("json_field", mismatch.Field)
		}
	case errors.As(err, &syntax):
		event.Str("decode_error", "syntax_error").Int64("json_offset", syntax.Offset)
	case errors.Is(err, io.EOF):
		event.Str("decode_error", "empty_response")
	case err != nil && strings.HasPrefix(err.Error(), "json: unknown field "):
		event.Str("decode_error", "unknown_field")
		field, unquoteErr := strconv.Unquote(strings.TrimPrefix(err.Error(), "json: unknown field "))
		if unquoteErr == nil && diagnosticJSONPath(field) {
			event.Str("json_field", field)
		}
	default:
		event.Str("decode_error", "invalid_document")
	}
}

func jsonKind(kind reflect.Kind) string {
	switch kind {
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Struct, reflect.Map:
		return "object"
	case reflect.Bool:
		return "boolean"
	case reflect.String:
		return "string"
	default:
		return "number"
	}
}

func diagnosticJSONPath(path string) bool {
	// These are schema names and common erroneous wrappers. Arbitrary keys
	// produced by a provider may themselves contain personal data.
	for _, field := range strings.Split(path, ".") {
		switch field {
		case "name", "description", "fields", "special_requirements", "key", "label", "type", "required", "pii", "pattern",
			"code", "severity", "external_source", "documents", "rules", "position", "document_type", "schema_version",
			"role", "conditional", "min_count", "trigger_reason", "condition", "field_key", "operator", "value",
			"left", "right", "subject_field", "message", "remediation",
			"definition", "data", "schema", "title", "properties", "additionalProperties", "document", "document_configuration":
		default:
			return false
		}
	}
	return true
}
