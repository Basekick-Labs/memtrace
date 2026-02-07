package sanitize

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Validation patterns for IDs and safe strings
var (
	// IDs must be alphanumeric with underscores and hyphens, prefixed
	idPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,127}$`)

	// Tags: alphanumeric, underscores, hyphens, dots
	tagPattern = regexp.MustCompile(`^[a-zA-Z0-9_.\-]{1,64}$`)

	// Memory types and event types: alphanumeric with underscores
	typePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)
)

// ValidateID checks that an ID matches the safe pattern
func ValidateID(id string) error {
	if id == "" {
		return nil // Empty is allowed (optional field)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid ID format: must be alphanumeric with underscores/hyphens, max 128 chars")
	}
	return nil
}

// ValidateType checks that a type string matches the safe pattern
func ValidateType(t string) error {
	if t == "" {
		return nil
	}
	if !typePattern.MatchString(t) {
		return fmt.Errorf("invalid type format: must be alphanumeric with underscores, max 64 chars")
	}
	return nil
}

// ValidateTag checks that a tag matches the safe pattern
func ValidateTag(tag string) error {
	if tag == "" {
		return nil
	}
	if !tagPattern.MatchString(tag) {
		return fmt.Errorf("invalid tag format: must be alphanumeric with underscores/hyphens/dots, max 64 chars")
	}
	return nil
}

// EscapeSQL escapes a string for safe use in SQL string literals.
// This doubles single quotes and removes null bytes.
func EscapeSQL(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	result := make([]byte, 0, len(s)+8)
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'', '\'')
		} else if s[i] == '\\' {
			result = append(result, '\\', '\\')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

// EscapeLike escapes LIKE pattern special characters (% and _)
func EscapeLike(s string) string {
	s = EscapeSQL(s)
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// ParseRelativeTime converts "2h", "24h", "7d" to DuckDB interval strings.
// Returns empty string if invalid.
func ParseRelativeTime(s string) string {
	if len(s) < 2 {
		return ""
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 || num > 8760 { // max ~1 year in hours
		return ""
	}

	switch unit {
	case 'h':
		return fmt.Sprintf("%d hours", num)
	case 'd':
		if num > 365 {
			return ""
		}
		return fmt.Sprintf("%d days", num)
	case 'm':
		if num > 525600 { // max ~1 year in minutes
			return ""
		}
		return fmt.Sprintf("%d minutes", num)
	default:
		return ""
	}
}

// SQLCondition safely builds a SQL equality condition after validation
func SQLCondition(column, value string) string {
	return fmt.Sprintf("%s = '%s'", column, EscapeSQL(value))
}

// SQLLikeCondition safely builds a SQL LIKE condition
func SQLLikeCondition(column, value string) string {
	return fmt.Sprintf("%s LIKE '%%%s%%' ESCAPE '\\'", column, EscapeLike(value))
}

// SQLInCondition safely builds a SQL IN condition
func SQLInCondition(column string, values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = fmt.Sprintf("'%s'", EscapeSQL(v))
	}
	return fmt.Sprintf("%s IN (%s)", column, strings.Join(quoted, ","))
}
