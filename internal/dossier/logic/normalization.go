package logic

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode"

	"sea-codex-server/internal/dossier/models"
)

var invalidValue = errors.New("value cannot be normalized")

func normalizeText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
func normalizeDigits(s string) (string, error) {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if !unicode.IsSpace(r) && r != '.' && r != '-' {
			return "", invalidValue
		}
	}
	if b.Len() == 0 {
		return "", invalidValue
	}
	return b.String(), nil
}
func normalizeDate(s string) (string, error) {
	for _, layout := range []string{"2006-01-02", "02/01/2006", "2/1/2006", "02-01-2006", "2-1-2006"} {
		if value, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return value.Format("2006-01-02"), nil
		}
	}
	return "", invalidValue
}

var numberPattern = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?$`)

func normalizeNumber(s, kind string) (string, error) {
	s = normalizeText(s)
	multiplier := int64(1)
	if kind == "money" {
		for _, suffix := range []string{"đồng", "vnd", "vnđ", "₫", "đ"} {
			s = strings.TrimSpace(strings.TrimSuffix(s, suffix))
		}
	}
	if kind == "area" {
		s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(s, "m²"), "m2"))
	}
	for _, unit := range []struct {
		suffix string
		value  int64
	}{{"tỷ", 1000000000}, {"triệu", 1000000}, {"nghìn", 1000}, {"ngàn", 1000}} {
		if strings.HasSuffix(s, unit.suffix) {
			multiplier = unit.value
			s = strings.TrimSpace(strings.TrimSuffix(s, unit.suffix))
			break
		}
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\u00a0", "")
	dot, comma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")
	if dot >= 0 && comma >= 0 {
		decimal, group := byte('.'), byte(',')
		if comma > dot {
			decimal, group = ',', '.'
		}
		parts := strings.Split(s, string(decimal))
		if len(parts) != 2 || strings.ContainsRune(parts[1], rune(group)) {
			return "", invalidValue
		}
		groups := strings.Split(strings.TrimLeft(parts[0], "+-"), string(group))
		if len(groups[0]) < 1 || len(groups[0]) > 3 {
			return "", invalidValue
		}
		for _, g := range groups[1:] {
			if len(g) != 3 {
				return "", invalidValue
			}
		}
		if dot > comma {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.ReplaceAll(s, ",", ".")
		}
	} else {
		separator := "."
		count := strings.Count(s, ".")
		if comma >= 0 {
			separator = ","
			count = strings.Count(s, ",")
		}
		if count > 1 {
			parts := strings.Split(s, separator)
			for _, part := range parts[1:] {
				if len(part) != 3 {
					return "", invalidValue
				}
			}
			s = strings.Join(parts, "")
		} else if count == 1 {
			parts := strings.Split(s, separator)
			if kind == "money" && separator == "." && len(parts[1]) == 3 && len(strings.TrimLeft(parts[0], "+-")) <= 3 {
				s = parts[0] + parts[1]
			} else {
				s = strings.Join(parts, ".")
			}
		}
	}
	if !numberPattern.MatchString(s) {
		return "", invalidValue
	}
	value, ok := new(big.Rat).SetString(s)
	if !ok {
		return "", invalidValue
	}
	value.Mul(value, new(big.Rat).SetInt64(multiplier))
	return value.RatString(), nil
}
func normalizeField(value, kind string) (string, error) {
	switch kind {
	case "text":
		if strings.TrimSpace(value) == "" {
			return "", invalidValue
		}
		return normalizeText(value), nil
	case "identifier":
		return normalizeDigits(value)
	case "date":
		return normalizeDate(value)
	case "number", "money", "area":
		return normalizeNumber(value, kind)
	case "boolean":
		switch normalizeText(value) {
		case "true", "yes", "có", "1":
			return "true", nil
		case "false", "no", "không", "0":
			return "false", nil
		}
	}
	return "", invalidValue
}
func validateField(f *models.ExtractedField, s models.FieldSchema) {
	f.NormalizedValue = ""
	f.Validation = "VALID"
	if strings.TrimSpace(f.Value) == "" {
		if s.Required {
			f.Validation = "MISSING"
		} else {
			f.Validation = "EMPTY"
		}
		return
	}
	normal, err := normalizeField(f.Value, s.Type)
	if err != nil {
		f.Validation = "INVALID_FORMAT"
	} else {
		f.NormalizedValue = normal
	}
	if s.Pattern != "" {
		pattern, err := regexp.Compile(s.Pattern)
		if err != nil || !pattern.MatchString(f.Value) {
			f.Validation = "INVALID_FORMAT"
		}
	}
	if f.Validation != "VALID" && f.Confidence > 0.49 {
		f.Confidence = 0.49
	}
}
func trusted(f models.ExtractedField) bool {
	return f.Validation == "VALID" && (f.Corrected || f.Confidence >= 0.8) && f.Value != ""
}
