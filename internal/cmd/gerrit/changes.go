package gerrit

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/util/dotgitreview"
	"gitlab.wikimedia.org/releng/cli/internal/util/output"
	stringsutil "gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

var (
	gerritProject string
	outputFormat  string
	outputFilter  []string
)

func NewGerritChangesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Interact with Gerrit changes",
	}
	cmd.AddCommand(NewGerritChangesListCmd())
	return cmd
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
	CreatedOn     int64
	LastUpdated   int64
	Open          bool
	Status        string
	WIP           bool
}

func NewGerritChangesListCmd() *cobra.Command {
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
			out := cmdutil.AttachOutputBuffer(ssh)

			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}
			logrus.Trace(out.String())

			lines := stringsutil.SplitMultiline(out.String())
			lastLine := lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			var objects []interface{}
			for _, line := range lines {
				change := Change{}
				err := json.Unmarshal([]byte(line), &change)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				objects = append(objects, change)
			}

			objects = output.Filter(objects, outputFilter)

			if outputFormat != "" {
				output.NewGoTmpl(objects, outputFormat).Print()
				return
			}

			output.TableFromObjects(
				objects,
				[]string{"ID", "Subject", "Status", "Owner", "Branch", "Updated"},
				func(object interface{}) []string {
					typedObject := object.(Change)
					tLastUpdated := time.Unix(typedObject.LastUpdated, 0)
					return []string{strconv.Itoa(typedObject.Number), typedObject.Subject, typedObject.Status, typedObject.Owner.Username, typedObject.Branch, tLastUpdated.Format("02 01 2006")}
				},
			).Print()

			fmt.Println("----------------")
			fmt.Println(lastLine)
			fmt.Println("If you see moreChanges:true, there is currently no way to see these more changes.")
		},
	}
	cmd.Flags().StringVarP(&gerritProject, "project", "p", "", "Auto detect from .gitreview, or specify")
	cmd.Flags().StringVarP(&outputFormat, "format", "", "", "Pretty print output using a Go template")
	cmd.Flags().StringSliceVarP(&outputFilter, "filter", "f", []string{}, "Filter output based on conditions provided")
	return cmd
}
