package sal

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

const baseURL = "https://sal.toolforge.org"

var projectName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// NewCmd exposes the public Server Admin Log, without requiring credentials.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "sal", Short: "Read and search public Wikimedia server admin logs"}
	cmd.AddCommand(newReadCmd(false), newReadCmd(true))
	return cmd
}

func newReadCmd(search bool) *cobra.Command {
	var project, before string
	var all bool
	var limit, page int
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "recent", Short: "Read the latest public server admin log entries",
		Long:    "Read up to 50 latest entries from SAL's public Atom feed. No authentication is required. Times are UTC.",
		Example: "recent\nrecent --project deployment-prep --limit 10\nrecent --all --output table", Args: cobra.NoArgs,
	}
	if search {
		cmd.Use = "search <query>"
		cmd.Short = "Search the public server admin log archive"
		cmd.Long = "Search SAL's archive across messages and user names using Elasticsearch simple query string syntax (for example, quote a phrase or use + to require terms). Results are newest first. SAL exposes search as HTML, which this command reads; its Atom feed only supports recent entries. No authentication is required. Times are UTC."
		cmd.Example = "search CentralAuth\nsearch 'T123456' --all\nsearch 'scap sync-world' --before 2026-01-31 --page 2"
		cmd.Args = cobra.MinimumNArgs(1)
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if all && cmd.Flags().Changed("project") {
			return fmt.Errorf("use either --all or --project")
		}
		if all {
			project = "__all__"
		}
		if !projectName.MatchString(project) || project == "." || project == ".." {
			return fmt.Errorf("invalid SAL project %q", project)
		}
		maxLimit := 50
		if search {
			maxLimit = 200
		}
		if limit < 1 || limit > maxLimit {
			return fmt.Errorf("limit must be between 1 and %d", maxLimit)
		}
		u, _ := url.Parse(baseURL)
		if search {
			if page < 1 {
				return fmt.Errorf("page must be at least 1")
			}
			query := strings.Join(args, " ")
			if strings.TrimSpace(query) == "" {
				return fmt.Errorf("search query must not be empty")
			}
			u.Path = "/" + project
			q := url.Values{"q": {query}, "i": {strconv.Itoa(limit)}, "p": {strconv.Itoa(page - 1)}}
			if before != "" {
				date, err := time.Parse("2006-01-02", before)
				if err != nil || date.After(time.Now().UTC()) {
					return fmt.Errorf("before must be a valid date on or before today (YYYY-MM-DD)")
				}
				q.Set("d", before)
			}
			u.RawQuery = q.Encode()
		} else {
			u.Path = "/atom/" + project
		}
		entries, err := readEntries(cmd.Context(), u, project, search)
		if err != nil {
			return err
		}
		if len(entries) > limit {
			entries = entries[:limit]
		}
		out.Print(cmd, entries)
		return nil
	}
	cmd.Flags().StringVar(&project, "project", "production", "SAL project (for example production, releng, or deployment-prep)")
	cmd.Flags().BoolVar(&all, "all", false, "Read entries from all projects")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum entries to return")
	if search {
		cmd.Flags().StringVar(&before, "before", "", "Entries on or before this UTC date (YYYY-MM-DD)")
		cmd.Flags().IntVar(&page, "page", 1, "Result page, starting at 1")
	}
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithTableBinding(&output.TableBinding{
		Headings: []string{"Time (UTC)", "User", "Project", "Message"},
		ProcessObjects: func(objects interface{}, table *output.Table) {
			for _, entry := range objects.([]Entry) {
				table.AddRow(entry.Timestamp, entry.User, entry.Project, entry.Message)
			}
		},
	}))
	accesspolicy.Support(cmd)
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPublic},
	})
}
