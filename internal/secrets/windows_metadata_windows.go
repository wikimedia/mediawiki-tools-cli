package secrets

import (
	"strings"

	"github.com/danieljoos/wincred"
	native "gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keyring"
)

// ListMetadata restricts the OS query itself to CLI-owned credential targets.
// Windows returns credential blobs along with metadata; they are discarded and
// never forwarded to WSL, logged, or persisted by this operation.
func (Keyring) ListMetadata() ([]native.CredentialMetadata, error) {
	result := []native.CredentialMetadata{}
	for _, prefix := range CredentialTargetPrefixes() {
		entries, err := wincred.FilteredList(prefix + "*")
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			clear(entry.CredentialBlob)
			if !strings.HasPrefix(entry.TargetName, prefix) || entry.UserName == "" {
				continue
			}
			suffix := ":" + entry.UserName
			if !strings.HasSuffix(entry.TargetName, suffix) {
				continue
			}
			result = append(result, native.CredentialMetadata{Service: strings.TrimSuffix(entry.TargetName, suffix), Account: entry.UserName, Target: entry.TargetName})
		}
	}
	return result, nil
}
