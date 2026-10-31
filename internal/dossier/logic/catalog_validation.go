package logic

import "sea-codex-server/internal/common"

// Only static rule/field names and an array index are retained, never values
// from generated configurations or database errors.
type catalogValidationError struct {
	Rule  string
	Field string
	Index int
}

func (e *catalogValidationError) Error() string { return "invalid catalog definition: " + e.Rule }
func (e *catalogValidationError) Unwrap() error { return common.ErrInvalidArgument }

func invalidCatalogDefinition(rule, field string, index int) error {
	return &catalogValidationError{Rule: rule, Field: field, Index: index}
}
