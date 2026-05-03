package docker

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewMwddDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "destroy",
		GroupID: "core",
		Short:   "Destroy all containers and data",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := mwdd.DefaultForUser()
			err := m.DockerCompose().Down(dockercompose.DownOptions{
				Volumes:       true,
				RemoveOrphans: true,
				Timeout:       1,
			})
			if err != nil {
				return err
			}
			logrus.Debug("Removing used hosts file")
			m.RemoveUsedHostsIfExists()

			if err := cleanupRecipeRuntimeState(m, false); err != nil {
				return err
			}

			mediaWikiPath := m.Env().Get("MEDIAWIKI_VOLUMES_CODE")
			if mediaWikiPath != "" {
				localSettingsDPath := filepath.Clean(filepath.Join(mediaWikiPath, "LocalSettings.d"))
				// Remove recipe-managed files from LocalSettings.d
				// Note: We do NOT delete LocalSettings.php as it may contain user edits
				if err := cleanupRecipeLocalSettingsDFiles(localSettingsDPath); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
