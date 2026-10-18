package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

var (
	credentialStorageInteractive = cli.CanAskForInput
	chooseCredentialProvider     = promptCredentialProvider
	detectKeyring                = secrets.DetectKeyring
	detectKeePassXC              = secrets.DetectKeePassXC
	pairCredentialGUI            = func() error { return secrets.PairKeePassXCBrowser(context.Background(), "") }
)

func WarnPlaintextStorage() {
	logrus.Warnf("Credentials will be stored unencrypted in %s.", Path())
}

func promptCredentialProvider() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	keyring := detectKeyring(ctx)
	keepass := detectKeePassXC()
	fmt.Fprintf(os.Stderr, "Choose where to store credentials. This becomes the default for new credentials.\nOS keyring: %s\nKeePassXC: %s\n", keyring.Detail, keepass.Detail)
	providers := []string{}
	labels := []string{}
	if keyring.Available {
		providers = append(providers, "keyring")
		labels = append(labels, "OS keyring")
	}
	if keepass.Available {
		providers = append(providers, "keepassxc")
		labels = append(labels, "KeePassXC ("+keepass.Detail+")")
	}
	providers = append(providers, "plaintext")
	labels = append(labels, "Plaintext configuration (unencrypted)")
	selected, err := askCredentialStorage(labels)
	if err != nil {
		return "", err
	}
	if selected < 0 || selected >= len(providers) {
		return "", fmt.Errorf("invalid credential storage selection")
	}
	provider := providers[selected]
	if provider == "keepassxc" && keepass.NeedsPairing {
		fmt.Fprintln(os.Stderr, "Unlock the intended KeePassXC database and approve the connection request.")
		if err := pairCredentialGUI(); err != nil {
			return "", err
		}
	}
	return provider, nil
}

// Kept separate so tests can exercise first-use selection without opening a TTY
// prompt or a password-manager window.
var askCredentialStorage = func(labels []string) (int, error) {
	var selected int
	err := survey.AskOne(&survey.Select{Message: "Credential storage:", Options: labels}, &selected)
	return selected, err
}
