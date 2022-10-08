package strings

import "strings"

func ReplaceInAll(list []string, find string, replace string) []string {
	for i, s := range list {
		list[i] = strings.Replace(s, find, replace, -1)
	}
	return list
}

func StringInSlice(find string, list []string) bool {
	for _, b := range list {
		if b == find {
			return true
		}
	}
	return false
}
