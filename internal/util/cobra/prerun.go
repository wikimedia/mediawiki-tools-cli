package cobrautil

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CallAllPersistentPreRun(cmd *cobra.Command, args []string) {
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
