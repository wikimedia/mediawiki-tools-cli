package phabricator

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func commandFlagEnabled(cmd *cobra.Command, name string) bool {
	flag := cmd.Flag(name)
	return flag != nil && flag.Value.String() == "true"
}

func confirmPhabricatorChange(cmd *cobra.Command) (bool, error) {
	if commandFlagEnabled(cmd, "yes") {
		return true, nil
	}
	if commandFlagEnabled(cmd, "no-interaction") || !cli.CanAskForInput() {
		return false, fmt.Errorf("no changes made; use --yes to proceed without interactive confirmation")
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

func confirmProfileRemoval(cmd *cobra.Command, site *string) *cobra.Command {
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		name, err := phabProfiles.Selected(*site)
		if err != nil {
			return err
		}
		cmd.Printf("Remove profile %q?\n", name)
		approved, err := confirmPhabricatorChange(cmd)
		if err != nil || !approved {
			return err
		}
		previous := *site
		*site = name
		defer func() { *site = previous }()
		return run(cmd, args)
	}
	cmd.Flags().Bool("yes", false, "Confirm profile removal without prompting")
	cmd.Long += " Confirm interactively or pass --yes."
	return cmd
}
