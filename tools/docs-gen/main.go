/*
Copyright © 2022 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package main

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra/doc"
	"gitlab.wikimedia.org/releng/cli/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/util/dirs"
)

func main() {
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
