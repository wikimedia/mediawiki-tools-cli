package cobrautil

import (
	"context"

	"github.com/sirupsen/logrus"
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

func CallAllPersistentPreRun(cmd *cobra.Command, args []string) {
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
	logrus.Tracef("CallAllPersistentPreRun for %s", cmd.Name())
	parent := cmd.Parent()
	if parent == nil {
		logrus.Tracef("No parent for %s", cmd.Name())
		return
	}
	if parent.PersistentPreRun != nil {
		logrus.Tracef("Calling parent PersistentPreRun for %s", cmd.Name())
		parent.PersistentPreRun(parent, args)
	} else {
		logrus.Tracef("No parent PersistentPreRun for %s", cmd.Name())
		// Recurse
		CallAllPersistentPreRun(parent, args)
	}
}
