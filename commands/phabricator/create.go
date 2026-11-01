package phabricator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func newPhabricatorCreateCmd(site *string) *cobra.Command {
	return newPhabricatorCreateCmdWithInteraction(site, cli.CanAskForInput, func() (bool, error) {
		var yes bool
		err := survey.AskOne(&survey.Confirm{Message: "Create this Phabricator task?", Default: false}, &yes)
		return yes, err
	})
}

func newPhabricatorCreateCmdWithInteraction(site *string, canAsk func() bool, confirm func() (bool, error)) *cobra.Command {
	var title, description, descriptionFile, inputFile string
	var projects []string
	var yes, dryRun bool
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a Phabricator task",
		Long:    "Preview and create a task using a saved Phabricator profile. Projects are exact project slugs (as used in hashtags) or project PHIDs. Confirm interactively, or pass --yes for scripts. Use --dry-run to preview without accessing credentials or contacting Phabricator.",
		Example: "create --title 'Clarify development setup' --description-file issue.txt --project a_wikimedia_cli\ncreate --title 'Documentation issue' --description-file - --yes",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if inputFile != "" {
				data, err := readTaskFile(cmd, inputFile)
				if err != nil {
					return err
				}
				var input struct {
					Title       string   `json:"title"`
					Description string   `json:"description"`
					Projects    []string `json:"projects"`
				}
				if err := json.Unmarshal(data, &input); err != nil {
					return fmt.Errorf("parse task JSON: %w", err)
				}
				title, description, projects = input.Title, input.Description, input.Projects
			}
			title = strings.TrimSpace(title)
			if title == "" {
				return fmt.Errorf("--title must not be empty")
			}
			if descriptionFile != "" {
				data, err := readTaskFile(cmd, descriptionFile)
				if err != nil {
					return err
				}
				description = string(data)
			}
			for i, project := range projects {
				projects[i] = strings.TrimPrefix(strings.TrimSpace(project), "#")
				if projects[i] == "" {
					return fmt.Errorf("--project must not be empty")
				}
			}
			cmd.Printf("Title: %s\n", title)
			if len(projects) > 0 {
				cmd.Printf("Projects: %s\n", strings.Join(projects, ", "))
			}
			cmd.Printf("\n%s\n\n", description)
			if dryRun {
				cmd.Println("Preview only; no task created.")
				return nil
			}
			if !yes {
				if !canAsk() {
					return fmt.Errorf("task was not created; use --yes to create without an interactive confirmation")
				}
				approved, err := confirm()
				if err != nil {
					return err
				}
				if !approved {
					cmd.Println("Cancelled; no task created.")
					return nil
				}
			}
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			id, err := client.createTask(title, description, projects)
			if err != nil {
				return err
			}
			cmd.Printf("Created T%d: %s/T%d\n", id, strings.TrimRight(client.baseURL, "/"), id)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Task title")
	cmd.Flags().StringVar(&description, "description", "", "Task description (Phabricator Remarkup)")
	cmd.Flags().StringVar(&descriptionFile, "description-file", "", "Read the description from a file, or - for stdin")
	cmd.Flags().StringVar(&inputFile, "input", "", "Read task JSON (title, description, projects) from a file, or - for stdin")
	cmd.MarkFlagsMutuallyExclusive("description", "description-file")
	cmd.Flags().StringSliceVar(&projects, "project", nil, "Project slug or PHID (repeatable or comma-separated)")
	for _, flag := range []string{"title", "description", "description-file", "project"} {
		cmd.MarkFlagsMutuallyExclusive("input", flag)
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Create without asking for confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without accessing credentials or creating a task")
	return cmd
}

func readTaskFile(cmd *cobra.Command, path string) ([]byte, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(cmd.InOrStdin())
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read task input: %w", err)
	}
	return data, nil
}

func (c *conduitClient) createTask(title, description string, projects []string) (int, error) {
	transactions := []map[string]interface{}{
		{"type": "title", "value": title},
		{"type": "description", "value": description},
	}
	var phids []string
	for _, project := range projects {
		if strings.HasPrefix(project, "PHID-PROJ-") {
			phids = append(phids, project)
			continue
		}
		result, err := c.post("project.search", map[string]interface{}{
			"constraints": map[string]interface{}{"slugs": []string{project}},
		})
		if err != nil {
			return 0, err
		}
		var found struct {
			Data []phabProject `json:"data"`
		}
		if err := json.Unmarshal(result, &found); err != nil {
			return 0, err
		}
		if len(found.Data) != 1 || found.Data[0].PHID == "" {
			return 0, fmt.Errorf("project slug %q did not resolve to exactly one project", project)
		}
		phids = append(phids, found.Data[0].PHID)
	}
	if len(phids) > 0 {
		transactions = append(transactions, map[string]interface{}{"type": "projects.add", "value": phids})
	}
	result, err := c.post("maniphest.edit", map[string]interface{}{"transactions": transactions})
	if err != nil {
		return 0, err
	}
	var created struct {
		Object struct {
			ID int `json:"id"`
		} `json:"object"`
	}
	if err := json.Unmarshal(result, &created); err != nil {
		return 0, err
	}
	if created.Object.ID <= 0 {
		return 0, fmt.Errorf("Phabricator did not return a created task ID; check your recent tasks before retrying")
	}
	return created.Object.ID, nil
}
