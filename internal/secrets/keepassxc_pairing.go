package secrets

import (
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"path/filepath"
)

func KeePassXCPairingPath() string {
	return filepath.Join(cli.UserDirectoryPath(), "secrets", "keepassxc-pairing.json")
}
