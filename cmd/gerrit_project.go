/*Package cmd is used for command line.

Copyright © 2020 Addshore

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
package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
	stringsutil "gitlab.wikimedia.org/releng/cli/internal/util/strings"
	"gopkg.in/ini.v1"
)

var mwddGerritProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Interact with Gerrit projects",
}

var mwddGerritProjectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Gerrit projects",
	Run: func(cmd *cobra.Command, args []string) {
		ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-projects"}))
		if err := ssh.Run(); err != nil {
			os.Exit(1)
		}
	},
}

var mwddGerritProjectSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search Gerrit projects",
	Example: `  search mediawiki/extensions
  search Wikibase Lexeme`,
	Run: func(cmd *cobra.Command, args []string) {
		ssh := cmdutil.AttachInErrIO(sshGerritCommand([]string{"ls-projects"}))
		out := cmdutil.AttachOutputBuffer(ssh)

		if err := ssh.Run(); err != nil {
			os.Exit(1)
		}

		fmt.Println(stringsutil.FilterMultiline(out.String(), args))
	},
}

var mwddGerritProjectCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Detect current Gerrit project",
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := os.Getwd()

		for {
			if _, err := os.Stat(dir + "/.gitreview"); os.IsNotExist(err) {
				dir = filepath.Dir(dir)
			} else {
				break
			}
			if dir == "/" {
				fmt.Println("Not in a Wikimedia Gerrit repository")
				os.Exit(1)
			}
		}

		gitReview, err := ini.Load(dir + "/.gitreview")
		if err != nil {
			log.Fatal(err)
		}

		project := gitReview.Section("gerrit").Key("project").String()
		project = strings.TrimSuffix(project, ".git")

		fmt.Println(project)
	},
}

func init() {
	mwddGerritCmd.AddCommand(mwddGerritProjectCmd)
	mwddGerritProjectCmd.AddCommand(mwddGerritProjectListCmd)
	mwddGerritProjectCmd.AddCommand(mwddGerritProjectSearchCmd)
	mwddGerritProjectCmd.AddCommand(mwddGerritProjectCurrentCmd)
}
