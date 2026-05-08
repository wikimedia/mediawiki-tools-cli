// Package phabricator provides the "phabricator" (alias "phab") command.
// It is a pure-Go implementation of the phab CLI tool using the Phabricator
// Conduit API directly, requiring no external dependencies.
package phabricator

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// NewPhabricatorCmd returns the "phabricator" cobra command.
func NewPhabricatorCmd() *cobra.Command {
	var site string

	cmd := &cobra.Command{
		Use:     "phabricator",
		Aliases: []string{"phab"},
		GroupID: "service",
		Short:   "Interact with Wikimedia Phabricator",
		Long: `Interact with Wikimedia Phabricator via the Conduit API.

Create an API token first:
	1. Log in at https://phabricator.wikimedia.org/auth/start/
	2. Open https://phabricator.wikimedia.org/settings/user/<username>/page/apitokens/
	3. Create a token and copy the value (for example: cli-XXXXXXXX)

Configure access in either phab.cfg or mwcli config.json.

Legacy phab.cfg is searched in:
	./phab.cfg
	$XDG_CONFIG_HOME/phab/phab.cfg (or ~/.config/phab/phab.cfg)
	~/.phab/phab.cfg
	/etc/phab/phab.cfg

Example phab.cfg:
  [main]
  default = wikimedia

  [wikimedia]
  url = https://phabricator.wikimedia.org
  key = cli-XXXXXXXXXXXXXXXXXXXXXXXXXXXX
  username = your-username
	default_project = #your-project

Alternative mwcli config.json keys:
	{
		"phabricator": {
			"default_site": "wikimedia",
			"sites": {
				"wikimedia": {
					"url": "https://phabricator.wikimedia.org",
					"key": "cli-XXXXXXXXXXXXXXXXXXXXXXXXXXXX",
					"username": "your-username",
					"default_project": "#your-project"
				}
			}
		}
	}

Tip: run "mw config where" to see your mwcli config.json path.`,
	}

	cmd.PersistentFlags().StringVar(&site, "site", "", "Config site section to use (overrides [main] default)")

	addAuthCmd(cmd, &site)
	addViewCmd(cmd, &site)
	addCommentsCmd(cmd, &site)
	addReadCmd(cmd, &site)
	addShellCmd(cmd, &site)
	addMVCmd(cmd, &site)
	addRMTagCmd(cmd, &site)
	addSetPrioCmd(cmd, &site)
	addSetStatusCmd(cmd, &site)

	return cmd
}

// clientFromSite loads config and creates a client. Returns client, cfg, and error.
func clientFromSite(site string) (*conduitClient, *PhabConfig, error) {
	cfg, err := loadConfig(site)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	client := newConduitClient(cfg)
	return client, cfg, nil
}

// requireTaskArg validates the task ref argument.
func requireTaskArg(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("task number required (e.g. T12345)")
	}
	ref := args[0]
	upper := strings.ToUpper(ref)
	if !strings.HasPrefix(upper, "T") {
		return "", fmt.Errorf("invalid task reference %q (expected T12345 format)", ref)
	}
	return upper, nil
}

func addViewCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Use:     "view <T12345>",
		Aliases: []string{"v"},
		Short:   "View a Phabricator task",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromSite(*site)
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
			renderTask(task, client)
			client.cache.flush()
			return nil
		},
	})
}

func addCommentsCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Use:     "comments <T12345>",
		Aliases: []string{"c"},
		Short:   "Show comments on a Phabricator task",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromSite(*site)
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
			renderComments(task, txs, client)
			client.cache.flush()
			return nil
		},
	})
}

func addReadCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Use:   "read <T12345>",
		Aliases: []string{"r"},
		Short: "View a task and all its comments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromSite(*site)
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
			renderTask(task, client)
			txs, err := client.getTransactions(task.PHID)
			if err != nil {
				return err
			}
			renderComments(task, txs, client)
			client.cache.flush()
			return nil
		},
	})
}

func addShellCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Use:     "shell",
		Aliases: []string{"sh"},
		Short:   "Start an interactive Phabricator shell",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cfg, err := clientFromSite(*site)
			if err != nil {
				return err
			}
			err = runShell(client, cfg)
			client.cache.flush()
			return err
		},
	})
}

func addMVCmd(parent *cobra.Command, site *string) {
	var project string
	c := &cobra.Command{
		Use:   "mv <T12345> <column>",
		Short: "Move a task to a workboard column",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cfg, err := clientFromSite(*site)
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
	parent.AddCommand(c)
}

func addRMTagCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Use:     "rmtag <T12345> <project>",
		Aliases: []string{"rm"},
		Short:   "Remove a project tag from a task",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromSite(*site)
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
	})
}

func addSetPrioCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Use:     "setprio <T12345> <priority>",
		Aliases: []string{"sp", "prio"},
		Short:   "Set priority: u=unbreak t=triage h=high n=normal l=low ll=lowest",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromSite(*site)
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
	})
}

func addSetStatusCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(&cobra.Command{
		Use:     "setstatus <T12345> <status>",
		Aliases: []string{"st"},
		Short:   "Set status: o=open r=resolved p=progress s=stalled d=declined dup=duplicate i=invalid",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := clientFromSite(*site)
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
	})
}
