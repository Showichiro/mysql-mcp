package sqlguard

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	lineComment  = regexp.MustCompile(`(?m)--.*$|#.*$`)
	blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	limitRe      = regexp.MustCompile(`(?i)\blimit\s+\d+(\s*,\s*\d+)?\b`)
	dangerRe     = regexp.MustCompile(`(?i)\b(insert|update|delete|replace|create|alter|drop|truncate|grant|revoke|call|load\s+data|set|start\s+transaction|begin|commit|rollback)\b`)
)

type Kind string

const (
	KindSelect   Kind = "select"
	KindShow     Kind = "show"
	KindDescribe Kind = "describe"
	KindExplain  Kind = "explain"
)

type Checked struct {
	SQL  string
	Kind Kind
}

func CheckReadOnly(sql string, maxRows int) (Checked, error) {
	normalized := normalize(sql)
	if normalized == "" {
		return Checked{}, fmt.Errorf("sql is required")
	}
	if hasMultipleStatements(normalized) {
		return Checked{}, fmt.Errorf("multiple SQL statements are not allowed")
	}
	if dangerRe.MatchString(normalized) {
		return Checked{}, fmt.Errorf("write or administrative SQL is not allowed")
	}

	kind, err := classify(normalized)
	if err != nil {
		return Checked{}, err
	}
	if kind == KindSelect && !limitRe.MatchString(normalized) {
		normalized = fmt.Sprintf("%s LIMIT %d", strings.TrimRight(normalized, "; "), maxRows)
	}
	return Checked{SQL: normalized, Kind: kind}, nil
}

func CheckExplainable(sql string) (string, error) {
	checked, err := CheckReadOnly(sql, 1)
	if err != nil {
		return "", err
	}
	if checked.Kind != KindSelect {
		return "", fmt.Errorf("mysql_explain only accepts SELECT statements")
	}
	return checked.SQL, nil
}

func normalize(sql string) string {
	sql = blockComment.ReplaceAllString(sql, " ")
	sql = lineComment.ReplaceAllString(sql, " ")
	sql = strings.TrimSpace(sql)
	sql = strings.TrimRight(sql, "; ")
	return strings.Join(strings.Fields(sql), " ")
}

func hasMultipleStatements(sql string) bool {
	return strings.Contains(sql, ";")
}

func classify(sql string) (Kind, error) {
	lower := strings.ToLower(strings.TrimSpace(sql))
	switch {
	case strings.HasPrefix(lower, "select "):
		return KindSelect, nil
	case lower == "select":
		return KindSelect, nil
	case strings.HasPrefix(lower, "show "):
		return KindShow, nil
	case strings.HasPrefix(lower, "describe "), strings.HasPrefix(lower, "desc "):
		return KindDescribe, nil
	case strings.HasPrefix(lower, "explain "):
		return KindExplain, nil
	default:
		return "", fmt.Errorf("only SELECT, SHOW, DESCRIBE, and EXPLAIN are allowed")
	}
}
