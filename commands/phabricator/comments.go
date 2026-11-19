package phabricator

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
)

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
