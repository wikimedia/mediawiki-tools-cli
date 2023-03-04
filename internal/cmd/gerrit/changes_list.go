package gerrit

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotgitreview"
)

var gerritProject string

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
	CreatedOn     int64
	LastUpdated   int64
	Open          bool
	Status        string
	WIP           bool
}

func NewGerritChangesListCmd() *cobra.Command {
	out := output.Output{
		TableBinding: &output.TableBinding{
			Headings: []string{"ID", "Subject", "Status", "Owner", "Branch", "Updated"},
			ProcessObjects: func(objects map[interface{}]interface{}, table *output.Table) {
				for _, object := range objects {
					typedObject := object.(Change)
					tLastUpdated := time.Unix(typedObject.LastUpdated, 0)
					table.AddRowS(strconv.Itoa(typedObject.Number), typedObject.Subject, typedObject.Status, typedObject.Owner.Username, typedObject.Branch, tLastUpdated.Format("02 01 2006"))
				}
			},
		},
		AckBinding: func(objects map[interface{}]interface{}, ack *output.Ack) {
			for _, object := range objects {
				typedObject := object.(Change)
				ack.AddItem(typedObject.Project, strconv.Itoa(typedObject.Number)+" "+typedObject.Subject+" "+typedObject.URL)
			}
		},
	}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Gerrit changes",
		Run: func(cmd *cobra.Command, args []string) {
			if gerritProject == "" {
				gitReview, err := dotgitreview.ForCWD()
				if err != nil {
					fmt.Println("Failed to get .gitreview file, are you in a Gerrit repository?")
					os.Exit(1)
				}
				gerritProject = gitReview.Project
			}

			ssh := cmdutil.AttachInErrIO(sshGerritCommand([]string{"query", "project:" + gerritProject + " status:open", "--format", "JSON"}))
			outBuff := cmdutil.AttachOutputBuffer(ssh)

			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}
			logrus.Trace(outBuff.String())

			lines := stringsutil.SplitMultiline(outBuff.String())
			// lastLine := lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			objects := make(map[interface{}]interface{}, len(lines))
			for key, line := range lines {
				change := Change{}
				err := json.Unmarshal([]byte(line), &change)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				objects[key] = change
			}

			out.Print(objects)
			// fmt.Println("----------------")
			// fmt.Println(lastLine)
			// fmt.Println("If you see moreChanges:true, there is currently no way to see these more changes.")
		},
	}
	out.AddFlags(cmd, "table")
	cmd.Flags().StringVarP(&gerritProject, "project", "p", "", "Auto detect from .gitreview, or specify")
	return cmd
}
