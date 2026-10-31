package secrets

import (
	"path/filepath"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func KeePassXCPairingPath() string {
	return filepath.Join(cli.UserDirectoryPath(), "secrets", "keepassxc-pairing.json")
}
