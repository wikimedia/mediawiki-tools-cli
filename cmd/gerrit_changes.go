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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/util/dotgitreview"
	stringsutil "gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

var mwddGerritChangesCmd = &cobra.Command{
	Use:   "changes",
	Short: "Interact with Gerrit changes",
}

type Change struct {
	Project string
	Branch  string
	Topic   string
	ID      string
	Number  int
	Subject string
	Owner   struct {
		Name     string
		Email    string
		Username string
	}
	URL           string
	CommitMessage string
	CreatedOn     int
	LastUpdated   int
	Open          bool
	Status        string
	WIP           bool
}

var mwddGerritChangesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Gerrit changes",
	Run: func(cmd *cobra.Command, args []string) {
		gitReview, err := dotgitreview.ForCWD()
		if err != nil {
			fmt.Println("Failed to get .gitreview file, are you in a Gerrit repository?")
			os.Exit(1)
		}

		ssh := cmdutil.AttachInErrIO(sshGerritCommand([]string{"query", "project:" + gitReview.Project + " status:open", "--format", "JSON"}))
		out := cmdutil.AttachOutputBuffer(ssh)

		if err := ssh.Run(); err != nil {
			os.Exit(1)
		}

		lines := stringsutil.SplitMultiline(out.String())
		lastLine := lines[len(lines)-1]
		lines = lines[:len(lines)-1]

		var changes []Change
		for _, line := range lines {
			change := Change{}
			err := json.Unmarshal([]byte(line), &change)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			changes = append(changes, change)
		}

		for _, change := range changes {
			fmt.Printf("%s %s %s\n", change.Branch, change.Owner.Username, change.Subject)
		}
		fmt.Println(lastLine)
	},
}

func init() {
	mwddGerritCmd.AddCommand(mwddGerritChangesCmd)
	mwddGerritChangesCmd.AddCommand(mwddGerritChangesListCmd)
}
