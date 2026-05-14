package phabricator

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newPhabricatorAuthStatusCmd(site *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Status of Wikimedia Phabricator authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(*site)
			if err != nil {
				return err
			}
			if strings.TrimSpace(cfg.Username) == "" || strings.TrimSpace(cfg.Key) == "" {
				return fmt.Errorf("no credentials found")
			}

			cmd.Println("Site:", cfg.SiteName)
			cmd.Println("URL:", cfg.URL)
			cmd.Println("Username:", cfg.Username)
			cmd.Println("Token:", tokenMask(cfg.Key))

			whoami, err := validatePhabricatorCredentials(cmd, cfg)
			if err != nil {
				return fmt.Errorf("not authenticated: %w", err)
			}

			cmd.Printf("Authenticated as %s\n", whoami)
			return nil
		},
	}

	return cmd
}
