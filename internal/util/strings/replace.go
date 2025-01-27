package strings

import "strings"

func ReplaceLastOccurrence(s, find, replace string) string {
	lastIndex := strings.LastIndex(s, find)
	if lastIndex == -1 {
		return s
	}
	return s[:lastIndex] + replace + s[lastIndex+len(find):]
}
