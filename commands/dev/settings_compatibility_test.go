package dev

import (
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
)

func TestSettingsIncludeSurvivesDevelopmentRename(t *testing.T) {
	for _, include := range []string{"", "/mwdd/MwddSettings.php", "/mwdd/DevSettings.php", "/wm-dev/DevSettings.php"} {
		t.Run(include, func(t *testing.T) {
			wiki := mediawiki.MediaWiki(t.TempDir())
			initial := "<?php\nrequire_once '" + include + "';\n"
			if include != "" {
				if err := os.WriteFile(wiki.LocalSettingsPath(), []byte(initial), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := ensureLocalSettingsBase(wiki); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(wiki.LocalSettingsPath())
			if err != nil {
				t.Fatal(err)
			}
			if include != "" && string(content) != initial {
				t.Fatal("an existing settings include was changed or duplicated")
			}
			if include == "" && !strings.Contains(string(content), "require_once '/mwdd/MwddSettings.php'") {
				t.Fatal("new settings must use the entry point restored by older releases")
			}
		})
	}
}
