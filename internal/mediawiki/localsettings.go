package mediawiki

import (
	"os"
	"strings"
)

/*LocalSettingsIsPresent ...*/
func (m MediaWiki) LocalSettingsIsPresent() bool {
	info, err := os.Stat(m.Path("LocalSettings.php"))
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

/*LocalSettingsContains ...*/
func (m MediaWiki) LocalSettingsContains(text string) bool {
	b, err := os.ReadFile(m.Path("LocalSettings.php"))
	if err != nil {
		panic(err)
	}
	s := string(b)
	return strings.Contains(s, text)
}
