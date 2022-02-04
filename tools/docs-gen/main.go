package main

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra/doc"
	"gitlab.wikimedia.org/releng/cli/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/util/dirs"
)

func main() {
	// Rendering the markdown before then trying to turn it into markdown does bad things, so skip it
	os.Setenv("MWCLI_SKIP_RENDER_MARKDOWN", "true")

	path := "./_docs"
	dirs.EnsureExists(path)
	cmdForDocs := cmd.NewMwCliCmd()

	// Disable this tag while we push docs to MediaWiki pages, to avoid a new edit even if there are no doc changes.
	// https://phabricator.wikimedia.org/T299976
	cmdForDocs.DisableAutoGenTag = true

	filePrepender := func(filename string) string {
		return ""
	}

	linkHandler := func(name string) string {
		trimmedName := strings.TrimSuffix(name, filepath.Ext(name))
		return "../" + trimmedName
	}

	err := doc.GenMarkdownTreeCustom(cmdForDocs, path, filePrepender, linkHandler)
	if err != nil {
		log.Fatal(err)
	}
}
