package phabricator

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
)

func newPhabricatorAuthStatusCmd(site *string) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataAccountMetadata},
		}),
		Use:   "status",
		Short: "Status of Wikimedia Phabricator authentication",
		Long:  "Show the selected profile, endpoint and username, then verify the saved token with user.whoami. The token is masked in output. This reads credentials for authentication, not task or paste content.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*site)
			if err != nil {
				return err
			}
			if strings.TrimSpace(cfg.Username) == "" || strings.TrimSpace(cfg.Key) == "" {
				return fmt.Errorf("no credentials found")
			}

			cmd.Println("Profile:", cfg.SiteName)
			cmd.Println("URL:", cfg.URL)
			cmd.Println("Username:", cfg.Username)
			cmd.Println("Token:", tokenMask(cfg.Key))

			whoami, err := validatePhabricatorCredentials(cmd, cfg)
			if err != nil {
				return fmt.Errorf("not authenticated: %w", err)
			}
			if !strings.EqualFold(cfg.Username, whoami) {
				return fmt.Errorf("authenticated as %s, expected %s", whoami, cfg.Username)
			}

			cmd.Printf("Authenticated as %s\n", whoami)
			return nil
		},
	}

	return cmd
}
