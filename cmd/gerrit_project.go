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
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/util/dotgitreview"
	stringsutil "gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

func NewGerritProjectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "project",
		Short: "Interact with Gerrit projects",
	}
}

func NewGerritProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Gerrit projects",
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-projects"}))
			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}
		},
	}
}

func NewGerritProjectSearchCmd() *cobra.Command {
	return &cobra.Command{
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
}

func NewGerritProjectCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Detect current Gerrit project",
		Run: func(cmd *cobra.Command, args []string) {
			gitReview, err := dotgitreview.ForCWD()
			if err != nil {
				fmt.Println("Failed to get .gitreview file, are you in a Gerrit repository?")
				os.Exit(1)
			}

			fmt.Println(gitReview.Project)
		},
	}
}
