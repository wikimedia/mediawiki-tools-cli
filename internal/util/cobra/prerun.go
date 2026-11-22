package cobrautil

import (
	"context"

	"github.com/spf13/cobra"
)

type executingCommandKey struct{}

// ExecutingCommand returns the original command while parent setup hooks run.
func ExecutingCommand(cmd *cobra.Command) *cobra.Command {
	if ctx := cmd.Root().Context(); ctx != nil {
		if original, ok := ctx.Value(executingCommandKey{}).(*cobra.Command); ok {
			return original
		}
	}
	return cmd
}

func CallAllPersistentPreRunE(cmd *cobra.Command, args []string) error {
	root := cmd.Root()
	previous := root.Context()
	ctx := previous
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Value(executingCommandKey{}) == nil {
		root.SetContext(context.WithValue(ctx, executingCommandKey{}, cmd))
		defer root.SetContext(previous)
	}
	parent := cmd.Parent()
	if parent == nil {
		return nil
	}
	if parent.PersistentPreRunE != nil {
		return parent.PersistentPreRunE(parent, args)
	}
	if parent.PersistentPreRun != nil {
		parent.PersistentPreRun(parent, args)
	} else {
		// Recurse
		return CallAllPersistentPreRunE(parent, args)
	}
	return nil
}
