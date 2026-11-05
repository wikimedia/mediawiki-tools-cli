package enterprise

import "github.com/spf13/cobra"

func newDownloadCmd(profile *string, use, short, example string, argc int, path func(*cobra.Command, []string) (string, error)) *cobra.Command {
	var file string
	var resume bool
	cmd := &cobra.Command{
		Use: use, Short: short, Example: example, Args: cobra.ExactArgs(argc),
		Long: short + ". Downloads remain compressed. --file is required and existing destinations are never overwritten. Interrupted downloads keep a .part file and metadata; --resume checks the remote ETag before continuing. Progress is written to stderr.",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := path(cmd, args)
			if err != nil {
				return err
			}
			c, err := client(*profile)
			if err != nil {
				return err
			}
			return c.Download(cmd.Context(), p+"/download", file, resume, cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Destination filename (required)")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume a partial download after checking its remote ETag")
	cmd.MarkFlagRequired("file")
	return cmd
}
