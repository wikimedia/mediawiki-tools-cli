package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

var (
	statusKeyring      = secrets.DetectKeyring
	statusWindows      = secrets.DetectWindowsCredentials
	statusNativeStore  = func() secrets.Store { return secrets.Keyring{} }
	statusWindowsStore = secrets.WindowsCredentialStore
)

func NewSecretStatusCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:       "status [keyring|windows-credential-manager|keepassxc]",
		Short:     "Show credential-store availability and optionally test native storage",
		Long:      "Show native keyring, Windows Credential Manager, and KeePassXC availability without retrieving saved credentials. With --check and a native provider, write, read, and delete a temporary test item. This does not authenticate to Gerrit or other services; use secrets list --verify for that.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"keyring", secrets.WindowsProvider, "keepassxc"},
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := ""
			if len(args) == 1 {
				selected = args[0]
			}
			if selected != "" && selected != "keyring" && selected != secrets.WindowsProvider && selected != "keepassxc" {
				return fmt.Errorf("select keyring, windows-credential-manager or keepassxc")
			}
			if check && selected != "keyring" && selected != secrets.WindowsProvider {
				return fmt.Errorf("--check requires keyring or windows-credential-manager; use secrets list --check --verify for saved credentials")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			providers := []string{"keyring", secrets.WindowsProvider, "keepassxc"}
			for _, provider := range providers {
				if selected != "" && selected != provider {
					continue
				}
				var available secrets.Availability
				switch provider {
				case "keyring":
					available = statusKeyring(ctx)
				case secrets.WindowsProvider:
					available = statusWindows(ctx)
				case "keepassxc":
					available = secrets.DetectKeePassXC()
				}
				state := "unavailable"
				if available.Available {
					state = "available"
				}
				cmd.Printf("%s: %s — %s\n", provider, state, available.Detail)
				if selected == "keyring" && runtime.GOOS == "linux" {
					cmd.Println("Uses Linux Secret Service, including in WSL. If locked, unlock the default keyring in your desktop password manager (for example Passwords and Keys).")
				}
				if check {
					if !available.Available {
						return fmt.Errorf("%s is unavailable; fix the connection before testing storage", provider)
					}
					client := statusNativeStore()
					if provider == secrets.WindowsProvider {
						var err error
						client, err = statusWindowsStore()
						if err != nil {
							return err
						}
					}
					cmd.Println("Testing a temporary item; unlock the credential store if prompted...")
					if err := probeCredentialStore(client); err != nil {
						return err
					}
					cmd.Println("Write, read-back verification, and cleanup succeeded. No saved application credentials were read or changed.")
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Write, verify, and delete a temporary item in the selected native credential store")
	return cmd
}

func probeCredentialStore(client secrets.Store) (result error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return fmt.Errorf("could not generate test item identifier")
	}
	account := hex.EncodeToString(id)
	service := secrets.CredentialServicePrefix + "credential-store-test"
	value := "temporary-credential-store-probe"
	// Always attempt cleanup, including when an interrupted write may have saved.
	defer func() {
		if err := client.Delete(service, account); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			result = fmt.Errorf("temporary item cleanup failed; remove %s / %s from the credential store", service, account)
		}
	}()
	if err := client.Set(service, account, value); err != nil {
		return fmt.Errorf("credential-store test write failed; check that the store is unlocked")
	}
	got, err := client.Get(service, account)
	if err != nil || got != value {
		return fmt.Errorf("credential-store test read-back failed")
	}
	return nil
}
