package phabricator

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func newTaskEditCmd(site *string) *cobra.Command {
	var title, description, descriptionFile, status, priority, assignee, comment string
	var projects []string
	var yes, dryRun bool
	cmd := &cobra.Command{Use: "edit <T12345>", Short: "Preview and update a task", Long: "Update selected task fields in one transaction request. Projects add tags without replacing existing tags. Preview changes with --dry-run; otherwise confirm interactively or pass --yes. Dry runs do not access credentials or resolve user/project names.", Example: "edit T12345 --title 'Improve CLI documentation' --dry-run\nedit T12345 --status resolved --comment 'Fixed in the latest release'\nedit T12345 --assigned-to me --project a_wikimedia_cli --yes", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := requireTaskArg(args)
		if err != nil {
			return err
		}
		// Validate the numeric part before prompting or retrieving credentials.
		if !isTaskRef(ref) {
			return fmt.Errorf("invalid task reference %q", ref)
		}
		transactions := []map[string]interface{}{}
		add := func(kind string, value interface{}) {
			transactions = append(transactions, map[string]interface{}{"type": kind, "value": value})
		}
		if cmd.Flags().Changed("title") {
			if strings.TrimSpace(title) == "" {
				return fmt.Errorf("--title must not be empty")
			}
			add("title", title)
		}
		if cmd.Flags().Changed("description-file") {
			body, e := readTaskFile(cmd, descriptionFile)
			if e != nil {
				return e
			}
			description = string(body)
		}
		if cmd.Flags().Changed("description") || cmd.Flags().Changed("description-file") {
			add("description", description)
		}
		if cmd.Flags().Changed("status") {
			value, ok := statusMap[strings.ToLower(status)]
			if !ok {
				return fmt.Errorf("unknown status %q", status)
			}
			add("status", value)
		}
		if cmd.Flags().Changed("priority") {
			value, ok := priorityMap[strings.ToLower(priority)]
			if !ok {
				return fmt.Errorf("unknown priority %q", priority)
			}
			add("priority", value)
		}
		if cmd.Flags().Changed("comment") {
			if strings.TrimSpace(comment) == "" {
				return fmt.Errorf("--comment must not be empty")
			}
			add("comment", comment)
		}
		if len(transactions) == 0 && !cmd.Flags().Changed("assigned-to") && len(projects) == 0 {
			return fmt.Errorf("specify at least one field to change")
		}
		if cmd.Flags().Changed("assigned-to") && strings.TrimSpace(assignee) == "" {
			return fmt.Errorf("--assigned-to must not be empty")
		}
		for _, p := range projects {
			if strings.TrimSpace(p) == "" {
				return fmt.Errorf("--project must not be empty")
			}
		}
		cmd.Printf("Update %s:\n", ref)
		for _, tx := range transactions {
			cmd.Printf("  %s: %v\n", tx["type"], tx["value"])
		}
		if assignee != "" {
			cmd.Printf("  assignee: %s\n", assignee)
		}
		if len(projects) > 0 {
			cmd.Printf("  add projects: %s\n", strings.Join(projects, ", "))
		}
		if dryRun {
			cmd.Println("Preview only; no task changed.")
			return nil
		}
		if !yes {
			if !cli.CanAskForInput() {
				return fmt.Errorf("task was not changed; use --yes to update without an interactive confirmation")
			}
			approved := false
			if err := survey.AskOne(&survey.Confirm{Message: "Apply these task changes?", Default: false}, &approved); err != nil {
				return err
			}
			if !approved {
				cmd.Println("Cancelled; no task changed.")
				return nil
			}
		}
		client, _, err := clientFromCommand(cmd, *site)
		if err != nil {
			return err
		}
		defer client.cache.flush()
		task, err := client.getTaskContext(cmd.Context(), ref)
		if err != nil {
			return err
		}
		if assignee != "" {
			user, e := client.resolveUserContext(cmd.Context(), assignee)
			if e != nil {
				return e
			}
			add("owner", user.PHID)
		}
		if len(projects) > 0 {
			phids := []string{}
			for _, p := range projects {
				project, e := client.exactProject(cmd.Context(), p)
				if e != nil {
					return e
				}
				phids = append(phids, project.PHID)
			}
			add("projects.add", phids)
		}
		if err = client.editTaskContext(cmd.Context(), task.PHID, transactions); err != nil {
			return err
		}
		cmd.Printf("Updated %s: %s/%s\n", ref, strings.TrimRight(client.baseURL, "/"), ref)
		return nil
	}}
	cmd.Flags().StringVar(&title, "title", "", "New task title")
	cmd.Flags().StringVar(&description, "description", "", "New description (Phabricator Remarkup)")
	cmd.Flags().StringVar(&descriptionFile, "description-file", "", "Read the description from a file, or - for stdin")
	cmd.MarkFlagsMutuallyExclusive("description", "description-file")
	cmd.Flags().StringVar(&status, "status", "", "Task status (open, resolved, progress, stalled, declined, duplicate or invalid)")
	cmd.Flags().StringVar(&priority, "priority", "", "Task priority (unbreak, triage, high, normal, low or lowest)")
	cmd.Flags().StringVar(&assignee, "assigned-to", "", "Assignee username or me")
	cmd.Flags().StringSliceVar(&projects, "project", nil, "Project hashtag, exact name, ID or PHID to add (repeatable)")
	cmd.Flags().StringVar(&comment, "comment", "", "Add a comment (Phabricator Remarkup)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Apply changes without confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without accessing credentials or changing a task")
	return cmd
}
