package driver

import "strings"

func normalizeTableType(t string) string {
	lower := strings.ToLower(t)
	if strings.Contains(lower, "base table") {
		return "table"
	}
	if strings.Contains(lower, "view") {
		return "view"
	}
	return lower
}

func splitByComma(s string) []string {
	var result []string
	var current string
	for _, ch := range s {
		if ch == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func parsePostgresArray(s string) []string {
	if s == "" || s == "{}" {
		return []string{}
	}
	s = s[1 : len(s)-1]
	var result []string
	for _, item := range splitByComma(s) {
		result = append(result, item)
	}
	return result
}

func parseMySQLArray(s string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	for _, item := range splitByComma(s) {
		result = append(result, item)
	}
	return result
}
