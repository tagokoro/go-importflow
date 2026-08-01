package checker

import (
	"path"
	"strings"
)

func matchPattern(pattern string, value string) bool {
	pattern = strings.Trim(pattern, "/")
	value = strings.Trim(value, "/")

	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		if value == base || strings.HasPrefix(value, base+"/") {
			return true
		}
	}

	patternParts := splitPath(pattern)
	valueParts := splitPath(value)
	return matchParts(patternParts, valueParts)
}

func splitPath(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}

func matchParts(pattern []string, value []string) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}
	if pattern[0] == "**" {
		return matchParts(pattern[1:], value) || (len(value) > 0 && matchParts(pattern, value[1:]))
	}
	if len(value) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], value[0])
	if err != nil || !ok {
		return false
	}
	return matchParts(pattern[1:], value[1:])
}
