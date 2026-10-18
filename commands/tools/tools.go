package tools

import (
	_ "embed"
	"errors"
	osexec "os/exec"
	"strconv"

	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

//go:embed exec.example
var execExample string

//go:embed cp.example
var cpExample string

//go:embed exec.long.md
var execLong string

func NewToolsCmd() *cobra.Command {
	command := toolforge.NewCommand()
	command.Use = "tools"
	command.GroupID = "service"
	command.Aliases = []string{"toolforge"}
	command.Short = "Manage Toolforge tools, jobs, builds and deployments"
	for _, child := range command.Commands() {
		if child.Name() == "kubernetes" {
			// Imported kubectl examples include full command paths, pipelines and
			// shell setup; they follow upstream conventions rather than mw's.
			if child.Annotations == nil {
				child.Annotations = make(map[string]string)
			}
			child.Annotations["mwcli-lint-skip"] = "upstream kubectl command tree"
			child.Annotations["mwcli-lint-skip-children"] = "upstream kubectl command tree"
		}
		if child.Name() == "shell" {
			child.Example = cobrautil.NormalizeExample(execExample)
			child.Long = cli.RenderMarkdown(execLong)
			run := child.RunE
			child.RunE = func(cmd *cobra.Command, args []string) error {
				return preserveExitCode(cmd, run(cmd, args))
			}
		}
	}
	command.AddCommand(newCopyCmd())
	inheritMwSetup(command, command)
	return command
}

// Preserve mw's telemetry/verbosity setup while retaining upstream's API and
// kubectl hooks. Cobra otherwise runs only the nearest persistent setup hook.
func inheritMwSetup(tree, command *cobra.Command) {
	preRunE, preRun := command.PersistentPreRunE, command.PersistentPreRun
	if command == tree || preRunE != nil || preRun != nil {
		command.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			if root != tree {
				if root.PersistentPreRunE != nil {
					if err := root.PersistentPreRunE(cmd, args); err != nil {
						return err
					}
				} else if root.PersistentPreRun != nil {
					root.PersistentPreRun(cmd, args)
				}
			}
			if preRunE != nil {
				return preRunE(cmd, args)
			}
			if preRun != nil {
				preRun(cmd, args)
			}
			return nil
		}
	}
	for _, child := range command.Commands() {
		inheritMwSetup(tree, child)
	}
}

func preserveExitCode(cmd *cobra.Command, err error) error {
	var exit *osexec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() > 0 {
		root := cmd.Root()
		if root.Annotations == nil {
			root.Annotations = make(map[string]string)
		}
		root.Annotations["exitCode"] = strconv.Itoa(exit.ExitCode())
	}
	return err
}

// Cleanup closes upstream SSH tunnels even when command execution fails.
func Cleanup() {
	toolforge.Cleanup()
}
