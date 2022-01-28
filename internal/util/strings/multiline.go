package strings

import (
	"bufio"
	"strings"
)

/*FilterMultiline ...*/
func FilterMultiline(s string, requiredMatches []string) string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	out := ""
	for scanner.Scan() {
		okay := true
		for _, arg := range requiredMatches {
			if !strings.Contains(scanner.Text(), arg) {
				okay = false
			}
		}
		if okay {
			out = out + scanner.Text() + "\n"
		}
	}
	return strings.Trim(out, "\n")
}

/*SplitMultiline ...*/
func SplitMultiline(s string) []string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	out := []string{}
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	return out
}
