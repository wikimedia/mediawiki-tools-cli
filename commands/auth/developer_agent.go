package auth

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewDeveloperAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Check ssh-agent status and provide setup instructions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Getenv("SSH_AUTH_SOCK") == "" {
				fmt.Println("ssh-agent is not running.")
				fmt.Println("You can start it for your current session by running:")
				fmt.Println("  eval \"$(ssh-agent -s)\"")
				return nil
			}

			fmt.Println("ssh-agent is running.")

			c := config.State()
			keyPath := c.Effective.Developer.SSHKeyPath
			if keyPath == "" {
				fmt.Println("SSH key path is not configured.")
				fmt.Println("Please run `mwdev auth developer create` to configure it.")
				return nil
			}

			// Get fingerprint of the key
			out, err := exec.Command("ssh-keygen", "-lf", keyPath).Output()
			if err != nil {
				return fmt.Errorf("could not get fingerprint of key %s: %w", keyPath, err)
			}
			fingerprint := strings.Split(string(out), " ")[1]

			// Check if key is already in agent
			out, err = exec.Command("ssh-add", "-l").Output()
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); !ok || exitError.ExitCode() != 1 {
					return fmt.Errorf("could not list ssh keys in agent: %w", err)
				}
			}

			if strings.Contains(string(out), fingerprint) {
				fmt.Printf("Your SSH key (%s) is already loaded into the agent.\n", keyPath)
			} else {
				fmt.Printf("Your SSH key (%s) is not loaded into the agent.\n", keyPath)
				fmt.Println("You can add it by running:")
				fmt.Printf("  ssh-add %s\n", keyPath)
			}

			return nil
		},
	}
	return cmd
}
