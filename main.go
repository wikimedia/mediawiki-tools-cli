/*
Copyright © 2020 Kosta Harlan <kosta@kostaharlan.net>

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
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra/doc"
	"gitlab.wikimedia.org/releng/cli/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/util/dirs"
)

// Following variables will be statically linked at the time of compiling

/*GitCommit holds short commit hash of source tree.*/
var GitCommit string

/*GitBranch holds current branch name the code is built off.*/
var GitBranch string

/*GitState shows whether there are uncommitted changes.*/
var GitState string

/*GitSummary holds output of git describe --tags --dirty --always.*/
var GitSummary string

/*BuildDate holds RFC3339 formatted UTC date (build time).*/
var BuildDate string

/*Version holds contents of ./VERSION file, if exists, or the value passed via the -version option.*/
var Version string

func main() {
	// Alow doc generation of the command if an env var is set
	if os.Getenv("MWCLI_DOC_GEN") != "" {
		path := os.Getenv("MWCLI_DOC_GEN")
		dirs.EnsureExists(path)
		cmdForDocs := cmd.NewMwCliCmd()

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
		return
	}

	// Alternatively, execute the command
	cmd.Execute(GitCommit, GitBranch, GitState, GitSummary, BuildDate, Version)
}
