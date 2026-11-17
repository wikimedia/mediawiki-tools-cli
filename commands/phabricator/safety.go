package phabricator

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/agent"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
)

func commandFlagEnabled(cmd *cobra.Command, name string) bool {
	flag := cmd.Flag(name)
	return flag != nil && flag.Value.String() == "true"
}

func commandAgentMode(cmd *cobra.Command) (bool, error) {
	detected, err := agent.Detect()
	return detected != nil || commandFlagEnabled(cmd, "agent"), err
}

func checkPhabricatorAgentPolicy(cmd *cobra.Command) error {
	agentMode, err := commandAgentMode(cmd)
	if err != nil {
		return err
	}
	err = commandmeta.CheckAgentPolicy(cmd, agentMode, commandFlagEnabled(cmd, "dry-run"))
	if err != nil && cmd.Flags().Lookup("dry-run") != nil {
		return fmt.Errorf("%w; --dry-run is available for an offline preview only", err)
	}
	return err
}

func confirmPhabricatorChange(cmd *cobra.Command) (bool, error) {
	if commandFlagEnabled(cmd, "yes") {
		return true, nil
	}
	if commandFlagEnabled(cmd, "no-interaction") || !cli.CanAskForInput() {
		return false, fmt.Errorf("no changes made; use --dry-run to preview or --yes to proceed without interactive confirmation")
	}
	var approved bool
	if err := survey.AskOne(&survey.Confirm{Message: "Apply these Phabricator changes?", Default: false}, &approved); err != nil {
		return false, err
	}
	if !approved {
		cmd.Println("Cancelled; no changes made.")
	}
	return approved, nil
}

// The preview describes supplied arguments only; it performs no remote lookups.
func taskMutationCommand(cmd *cobra.Command, validateValue func(string) error) *cobra.Command {
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := checkPhabricatorAgentPolicy(cmd); err != nil {
			return err
		}
		ref, err := requireTaskArg(args)
		if err != nil {
			return err
		}
		if strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("the change value must not be empty")
		}
		if validateValue != nil {
			if err := validateValue(args[1]); err != nil {
				return err
			}
		}
		cmd.Printf("Requested operation: %s %s %q\n", cmd.CommandPath(), ref, args[1])
		if flag := cmd.Flag("project"); flag != nil {
			project := flag.Value.String()
			if project == "" {
				project = "(configured default; resolved only when applying)"
			}
			cmd.Printf("Project: %s\n", project)
		}
		if commandFlagEnabled(cmd, "dry-run") {
			cmd.Println("Preview only; no credentials accessed or remote names resolved, and no task changed.")
			return nil
		}
		approved, err := confirmPhabricatorChange(cmd)
		if err != nil || !approved {
			return err
		}
		return run(cmd, args)
	}
	addMutationFlags(cmd)
	cmd.Long = cmd.Short + ". Preview the requested change with --dry-run without accessing credentials or contacting Phabricator. " +
		"Remote targets and configured defaults are resolved only when applying. Confirm interactively or pass --yes. " +
		"--no-interaction never assumes confirmation. Agents may preview but cannot apply changes; --yes does not override agent policy."
	return cmd
}

func addMutationFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("yes", false, "Apply changes without asking for confirmation (does not override agent policy)")
	cmd.Flags().Bool("dry-run", false, "Preview without applying changes or accessing credentials")
}

func profileMutationCommand(cmd *cobra.Command, site *string) *cobra.Command {
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := checkPhabricatorAgentPolicy(cmd); err != nil {
			return err
		}
		if cmd.Name() == "default" && strings.TrimSpace(*site) == "" {
			return fmt.Errorf("provide --profile")
		}
		name, err := phabProfiles.Selected(*site)
		if err != nil {
			return err
		}
		cmd.Printf("Requested profile operation: %s --profile %q\n", cmd.CommandPath(), name)
		if commandFlagEnabled(cmd, "dry-run") {
			cmd.Println("Preview only; no credentials accessed or profiles changed.")
			return nil
		}
		approved, err := confirmPhabricatorChange(cmd)
		if err != nil || !approved {
			return err
		}
		previous := *site
		*site = name
		defer func() { *site = previous }()
		return run(cmd, args)
	}
	addMutationFlags(cmd)
	if cmd.Long == "" {
		cmd.Long = cmd.Short + "."
	}
	cmd.Example = cmd.Name() + " --profile personal --dry-run\n" + cmd.Name() + " --profile personal --yes"
	cmd.Long += "\n\nPreview the selected profile operation with --dry-run without accessing credentials. Confirm interactively or pass --yes. Agents may preview but cannot apply profile changes."
	return cmd
}
