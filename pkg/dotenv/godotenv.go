package dotenv

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Copy of godotenv.Write with modifications to never quote
// https://github.com/joho/godotenv/issues/50#issuecomment-364873528
// https://github.com/moby/moby/issues/12997
func writeOverride(envMap map[string]string, filename string) error {
	content, err := marshelOverride(envMap)
	if err != nil {
		return err
	}
	file, err := os.Create(filepath.Clean(filename))
	if err != nil {
		return err
	}
	_, err = file.WriteString(content)
	return err
}

// Copy of godotenv.Marshel with modifications to never quote
// https://github.com/joho/godotenv/issues/50#issuecomment-364873528
// https://github.com/moby/moby/issues/12997
func marshelOverride(envMap map[string]string) (string, error) {
	lines := make([]string, 0, len(envMap))
	for k, v := range envMap {
		lines = append(lines, fmt.Sprintf(`%s=%s`, k, doubleQuoteEscape(v)))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

const doubleQuoteSpecialChars = "\\\n\r\"!$`"

func doubleQuoteEscape(line string) string {
	for _, c := range doubleQuoteSpecialChars {
		toReplace := "\\" + string(c)
		if c == '\n' {
			toReplace = `\n`
		}
		if c == '\r' {
			toReplace = `\r`
		}
		line = strings.Replace(line, string(c), toReplace, -1)
	}
	return line
}
