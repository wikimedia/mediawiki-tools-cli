// Package phabricator provides the "phabricator" (alias "phab") command.
// It is a pure-Go implementation of the phab CLI tool using the Phabricator
// Conduit API directly, requiring no external dependencies.
package phabricator

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

// NewPhabricatorCmd returns the "phabricator" cobra command.
func NewPhabricatorCmd() *cobra.Command {
	var site string

	cmd := &cobra.Command{
		Use:     "phabricator",
		Aliases: []string{"phab"},
		GroupID: "service",
		Short:   "Interact with Wikimedia Phabricator",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cobrautil.CallAllPersistentPreRun(cmd, args)
			return checkPhabricatorAgentPolicy(cmd)
		},
		Long: `Interact with Wikimedia Phabricator.

Without a selected or default profile or a legacy CLI token, task list, task
view, read, comments, and project view use public pages on phabricator.wikimedia.org.
These requests are anonymous. Writes, paste reads, and raw API calls require
a saved account or legacy CLI token.

Run "mw phabricator auth login --profile personal" to sign in through your browser
and save credentials in your chosen password manager. Use --method token for an
existing token. Use --profile on any command to choose a saved account.
Use login --url to explicitly choose the endpoint.
Use auth login --method phab-cfg to validate and copy credentials from phab.cfg.
Commands use saved CLI profiles; external credential files are never used directly.

Use --public-only (or set phabricator.public_only to true) to mask non-public tasks.
--agent and detected agent environments require a human rerun outside agent mode
for non-public task access. These are local guardrails, not credential isolation.
Only explicitly public tasks in Wikimedia's known public S1 Space pass the
initial filter; other Spaces and missing policies are treated as unknown.
The low-level api command and non-task objects are not covered by the task filter.
Agent approvals follow the CLI-wide agent.policy setting, initially enforced
only for Phabricator: permissive requires approval for high-risk operations,
balanced (default) for writes and high-risk operations, and cautious for all calls.
Supported offline --dry-run previews never require approval. When approval is
required, explain the operation and ask the user before rerunning with --yes.
Raw API calls are high-risk and unfiltered, even with task access flags.
Targeted task/paste writes do not prompt humans
for confirmation. Profile removal still requires confirmation or --yes.
--yes is a cooperative assertion of user approval, not a bypass of task visibility checks.
For task access controls, see "mw help guides phabricator access".

For storage options and password managers, see "mw help guides authentication".`,
	}

	cmd.PersistentFlags().StringVar(&site, "site", "", "Saved site/profile to use (compatible alias for --profile)")
	phabProfiles.AddFlag(cmd, &site)
	cmd.MarkFlagsMutuallyExclusive("site", "profile")
	cmd.PersistentFlags().Bool("public-only", false, "Restrict task commands to public tasks in Wikimedia's known public Space (also phabricator.public_only)")
	cmd.PersistentFlags().Bool("agent", false, "Enforce agent restrictions on task access, mutations, login, raw API and shell")
	cmd.PersistentFlags().Bool("yes", false, "Assert user approval when agent policy requires it; does not override task visibility checks")
	cmd.PersistentFlags().Bool("allow-non-public", false, "Approve non-public task access for this invocation outside agent mode")
	// TODO(early 2027): remove the legacy --site flag after the profile migration window.
	_ = cmd.PersistentFlags().MarkHidden("site")

	cmd.AddCommand(newPhabricatorAuthCmd(&site))
	cmd.AddCommand(newPhabricatorAPICmd(&site))
	cmd.AddCommand(newPhabricatorMCPCmd(&site))
	cmd.AddCommand(newPhabricatorCreateCmd(&site))
	cmd.AddCommand(newTaskCmd(&site), newProjectCmd(&site), newBoardCmd(&site), newPhabricatorBrowseCmd(&site))
	cmd.AddCommand(newPhabricatorPasteCmd(&site))
	cmd.AddCommand(newPhabricatorObjectViewCmd(&site, ""))
	addCommentsCmd(cmd, &site)
	addReadCmd(cmd, &site)
	addAssignCmd(cmd, &site)
	// TODO(early 2027): remove phab shell in favour of browse and the task commands.
	// Once it reaches some kind of feature parity with shell.
	addShellCmd(cmd, &site)
	addMVCmd(cmd, &site)
	addRMTagCmd(cmd, &site)
	addSetPrioCmd(cmd, &site)
	addSetStatusCmd(cmd, &site)

	return cmd
}

// requireTaskArg validates the task ref argument.
func requireTaskArg(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("task number required (e.g. T12345)")
	}
	ref := args[0]
	upper := strings.ToUpper(ref)
	if !objectReferencePattern.MatchString(upper) || !strings.HasPrefix(upper, "T") {
		return "", fmt.Errorf("invalid task reference %q (expected T12345 format)", ref)
	}
	return upper, nil
}

func addCommentsCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata},
		}),
		Use:     "comments <T12345>",
		Aliases: []string{"c"},
		Short:   "Show comments on a Phabricator task",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if useAnonymousPhabricator(cmd, *site) {
				return readAnonymousTask(cmd, args, true)
			}
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			taskRef, err := requireTaskArg(args)
			if err != nil {
				return err
			}
			task, err := client.getTask(taskRef)
			if err != nil {
				return err
			}
			txs, err := client.getTransactions(task.PHID)
			if err != nil {
				return err
			}
			renderComments(cmd.OutOrStdout(), task, txs, client)
			client.cache.flush()
			return nil
		},
	})
}

func addReadCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata},
		}),
		Use:     "read <T12345>",
		Aliases: []string{"r"},
		Short:   "View a task and all its comments",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if useAnonymousPhabricator(cmd, *site) {
				return readAnonymousTask(cmd, args, false)
			}
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			taskRef, err := requireTaskArg(args)
			if err != nil {
				return err
			}
			task, err := client.getTask(taskRef)
			if err != nil {
				return err
			}
			renderTask(cmd.OutOrStdout(), task, client)
			txs, err := client.getTransactions(task.PHID)
			if err != nil {
				return err
			}
			renderComments(cmd.OutOrStdout(), task, txs, client)
			client.cache.flush()
			return nil
		},
	})
}

func addAssignCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(taskMutationCommand(&cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata},
		}),
		Use:     "assign <T12345> <username|me>",
		Short:   "Assign a task to a Phabricator user",
		Example: "assign T12345 me --dry-run\nassign T12345 me",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			taskRef, err := requireTaskArg(args[:1])
			if err != nil {
				return err
			}
			user, err := client.resolveUser(args[1])
			if err != nil {
				return err
			}
			taskPHID, err := client.lookupTaskPHID(taskRef)
			if err != nil {
				return err
			}
			if err := client.assignTask(taskPHID, user.PHID); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Assigned %s to %s\n", taskRef, user.Fields.Username)
			client.cache.flush()
			return nil
		},
	}, nil))
}

func addShellCmd(parent *cobra.Command, site *string) {
	var project string
	var task string

	c := &cobra.Command{
		Use: "shell",
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			HighRisk:        true,
			ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata},
		}),
		Aliases: []string{"sh"},
		Short:   "Start an interactive Phabricator shell",
		Long:    "Browse projects, tasks and comments, and update task assignment, columns, tags, priority or status. Changes run immediately without a separate confirmation. Existing task access guardrails also apply inside the shell. All agent policies require user approval (--yes) for this high-risk session, covering its subsequent actions. An interactive terminal is required; --no-interaction refuses the shell. Prefer one-shot task commands for narrowly scoped approval.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandFlagEnabled(cmd, "no-interaction") || !cli.CanAskForInput() {
				return fmt.Errorf("the mutation shell requires an interactive terminal; use browse or one-shot task commands")
			}
			client, cfg, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			startTask := task
			if task != "" {
				validated, err := requireTaskArg([]string{task})
				if err != nil {
					return err
				}
				startTask = validated
			}
			err = runShell(client, cfg, project, startTask)
			client.cache.flush()
			return err
		},
	}

	c.Flags().StringVarP(&project, "project", "p", "", "Project to start shell in (overrides default_project)")
	c.Flags().StringVarP(&task, "task", "t", "", "Task to preselect (e.g. T12345)")
	parent.AddCommand(c)
}

func addMVCmd(parent *cobra.Command, site *string) {
	var project string
	c := &cobra.Command{
		Use: "mv <T12345> <column>",
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata},
		}),
		Short:   "Move a task to a workboard column",
		Example: "mv T12345 Backlog --project a_wikimedia_cli --dry-run\nmv T12345 Backlog --project a_wikimedia_cli",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cfg, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			taskRef, err := requireTaskArg(args[:1])
			if err != nil {
				return err
			}
			proj := project
			if proj == "" {
				proj = cfg.DefaultProject
			}
			if proj == "" {
				return fmt.Errorf("--project or default_project in config required")
			}
			projectPHID, err := client.lookupProjectPHID(proj)
			if err != nil {
				return err
			}
			columns, err := client.getColumns(projectPHID)
			if err != nil {
				return err
			}
			colName := normaliseColumnKey(args[1])
			colPHID, ok := columns[colName]
			if !ok {
				return fmt.Errorf("column %q not found in project %s", args[1], proj)
			}
			taskPHID, err := client.lookupTaskPHID(taskRef)
			if err != nil {
				return err
			}
			if err := client.moveTaskToColumn(taskPHID, colPHID); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Moved %s to %s\n", taskRef, colName)
			client.cache.flush()
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", "", "Project name (overrides default_project)")
	parent.AddCommand(taskMutationCommand(c, nil))
}

func addRMTagCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(taskMutationCommand(&cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata},
		}),
		Use:     "rmtag <T12345> <project>",
		Aliases: []string{"rm"},
		Short:   "Remove a project tag from a task",
		Example: "rmtag T12345 a_wikimedia_cli --dry-run\nrmtag T12345 a_wikimedia_cli",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			taskRef, err := requireTaskArg(args[:1])
			if err != nil {
				return err
			}
			projects, err := client.findProjectsByName(args[1])
			if err != nil {
				return err
			}
			if len(projects) == 0 {
				return fmt.Errorf("project %q not found", args[1])
			}
			if len(projects) > 1 {
				fmt.Fprintln(os.Stderr, "Multiple projects matched:")
				for name, phid := range projects {
					fmt.Fprintf(os.Stderr, "  %s (%s)\n", name, phid)
				}
				return fmt.Errorf("specify a more precise project name")
			}
			var projectPHID string
			for _, phid := range projects {
				projectPHID = phid
			}
			taskPHID, err := client.lookupTaskPHID(taskRef)
			if err != nil {
				return err
			}
			if err := client.removeProjectFromTask(taskPHID, projectPHID); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Removed tag from %s\n", taskRef)
			client.cache.flush()
			return nil
		},
	}, nil))
}

func addSetPrioCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(taskMutationCommand(&cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata},
		}),
		Use:     "setprio <T12345> <priority>",
		Aliases: []string{"sp", "prio"},
		Short:   "Set priority: u=unbreak t=triage h=high n=normal l=low ll=lowest",
		Example: "setprio T12345 high --dry-run\nsetprio T12345 high",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			taskRef, err := requireTaskArg(args[:1])
			if err != nil {
				return err
			}
			prioVal, ok := priorityMap[strings.ToLower(args[1])]
			if !ok {
				return fmt.Errorf("unknown priority %q; valid: u=unbreak t=triage h=high n=normal l=low ll=lowest", args[1])
			}
			taskPHID, err := client.lookupTaskPHID(taskRef)
			if err != nil {
				return err
			}
			if err := client.setTaskPriority(taskPHID, prioVal); err != nil {
				return err
			}
			task, err := client.getTaskByPHID(taskPHID)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "%s priority set to: %s\n", taskRef, task.Fields.Priority.Name)
			client.cache.flush()
			return nil
		},
	}, func(value string) error {
		if _, ok := priorityMap[strings.ToLower(value)]; !ok {
			return fmt.Errorf("unknown priority %q; valid: u=unbreak t=triage h=high n=normal l=low ll=lowest", value)
		}
		return nil
	}))
}

func addSetStatusCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(taskMutationCommand(&cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata},
		}),
		Use:     "setstatus <T12345> <status>",
		Aliases: []string{"st"},
		Short:   "Set status: o=open r=resolved p=progress s=stalled d=declined dup=duplicate i=invalid",
		Example: "setstatus T12345 resolved --dry-run\nsetstatus T12345 resolved",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			taskRef, err := requireTaskArg(args[:1])
			if err != nil {
				return err
			}
			statusVal, ok := statusMap[strings.ToLower(args[1])]
			if !ok {
				return fmt.Errorf("unknown status %q; valid: o=open r=resolved p=progress s=stalled d=declined dup=duplicate i=invalid", args[1])
			}
			taskPHID, err := client.lookupTaskPHID(taskRef)
			if err != nil {
				return err
			}
			if err := client.setTaskStatus(taskPHID, statusVal); err != nil {
				return err
			}
			task, err := client.getTaskByPHID(taskPHID)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "%s status set to: %s\n", taskRef, task.Fields.Status.Name)
			client.cache.flush()
			return nil
		},
	}, func(value string) error {
		if _, ok := statusMap[strings.ToLower(value)]; !ok {
			return fmt.Errorf("unknown status %q; valid: o=open r=resolved p=progress s=stalled d=declined dup=duplicate i=invalid", value)
		}
		return nil
	}))
}
