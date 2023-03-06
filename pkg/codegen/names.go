package codegen

import (
	"strings"
)

var seperatorsToNormalize = []string{"-", "_", " ", "."}

func ForFunctionNamePart(s string) string {
	return ForFunctionName(s, false)
}

func ForFunctionNameStart(s string) string {
	return ForFunctionName(s, true)
}

func ForFunctionName(s string, isStart bool) string {
	// Shortcut for empty strings
	if s == "" {
		return s
	}
	// Remove separator chars by splitting and joining, uppercasding words
	// e.g. `get-change` -> `get change` -> `getChange`
	for _, separator := range seperatorsToNormalize {
		split := strings.Split(s, separator)
		for i, v := range split {
			split[i] = upperCaseFirstChar(v)
		}
		s = strings.Join(split, "")
	}
	// Uppercase first char if the string is not being used at the start of a function name
	if isStart {
		s = lowerCaseFirstChar(s)
	} else {
		s = upperCaseFirstChar(s)
	}
	return s
}

func upperCaseFirstChar(s string) string {
	return strings.ToUpper(s[:1]) + s[1:]
}

func lowerCaseFirstChar(s string) string {
	return strings.ToLower(s[:1]) + s[1:]
}

func ForFileName(s string) string {
	// Shortcut for empty strings
	if s == "" {
		return s
	}
	// Lowercase everything
	s = strings.ToLower(s)
	// Replace seperators with `_`
	// e.g. `get-change` -> `get_change`
	s = normalizeSeperators(s, "_")
	return s
}

func normalizeSeperators(s string, to string) string {
	for _, separator := range seperatorsToNormalize {
		s = strings.ReplaceAll(s, separator, to)
	}
	return s
}
