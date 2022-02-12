package gerrit

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/util/dotgitreview"
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

			// Filter
			if outputFilter != nil {
				getField := func(v *Change, filter string) string {
					fields := strings.Split(filter, ".")
					val := reflect.ValueOf(v)
					for _, field := range fields {
						val = reflect.Indirect(val).FieldByName(field)
					}
					return string(val.String())
				}
				for _, filter := range outputFilter {
					filterSplit := strings.Split(filter, "=")
					filterKey := filterSplit[0]
					filterValue := filterSplit[1]
					for i := len(changes) - 1; i >= 0; i-- {
						change := changes[i]
						fieldValue := getField(&change, filterKey)
						keep := true
						if filterValue[0:1] == "*" && filterValue[len(filterValue)-1:] == "*" {
							lookFor := filterValue[1 : len(filterValue)-1]
							if !strings.Contains(fieldValue, lookFor) {
								logrus.Tracef("Filtering out as '%s' not in '%s'", lookFor, fieldValue)
								keep = false
							}
						} else if fieldValue != filterValue {
							logrus.Tracef("Filtering out as '%s' doesn't match '%s'", filterValue, fieldValue)
							keep = false
						}

						if !keep {
							changes = append(changes[:i], changes[i+1:]...)
						}
					}
				}
			}

			// Output using format
			if outputFormat != "" {
				tmpl := template.Must(template.
					New("").
					Funcs(map[string]interface{}{
						"json": func(v interface{}) (string, error) {
							b, err := json.MarshalIndent(v, "", "  ")
							if err != nil {
								return "", err
							}
							return string(b), nil
						},
					}).
					Parse(outputFormat))
				for _, change := range changes {
					_ = tmpl.Execute(os.Stdout, change)
					fmt.Println()
				}
				return
			}

			// Default table output below
			headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
			columnFmt := color.New(color.FgYellow).SprintfFunc()

			tbl := table.New("ID", "Subject", "Status", "Owner", "Branch", "Updated")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, change := range changes {
				tLastUpdated := time.Unix(change.LastUpdated, 0)
				tbl.AddRow(change.Number, change.Subject, change.Status, change.Owner.Username, change.Branch, tLastUpdated.Format("02 01 2006"))
			}
			tbl.Print()

			fmt.Println(lastLine)
			fmt.Println("If you see moreChanges:true, there is currently no way to see these more changes.")
		},
	}
	cmd.Flags().StringVarP(&gerritProject, "project", "p", "", "Auto detect from .gitreview, or specify")
	cmd.Flags().StringVarP(&outputFormat, "format", "", "", "Pretty print output using a Go template")
	cmd.Flags().StringSliceVarP(&outputFilter, "filter", "f", []string{}, "Filter output based on conditions provided")
	return cmd
}
