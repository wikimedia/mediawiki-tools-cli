package secrets

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keepassxc"
)

// KeePassXCGroup names the CLI-owned top-level group for new entries.
// Existing references use UUIDs, so they remain valid in older or renamed groups.
const KeePassXCGroup = "a-wikimedia-cli"

// App paths stay here; the reusable provider has no CLI dependencies.
type KeePassXCBrowser keepassxc.Reference

func browserClient() *keepassxc.Client {
	return &keepassxc.Client{PairingFile: KeePassXCPairingPath(), Group: KeePassXCGroup}
}

func PairKeePassXCBrowser(ctx context.Context, proxy string) error {
	return browserClient().Pair(ctx, proxy)
}
func KeePassXCBrowserStatus() (string, error) { return browserClient().Status() }
func ForgetKeePassXCBrowser() error           { return browserClient().Disconnect() }
func FindKeePassXCBrowserEntry(ctx context.Context, path string) (KeePassXCBrowser, string, error) {
	ref, value, err := browserClient().Find(ctx, path)
	if err != nil {
		return KeePassXCBrowser{}, "", fmt.Errorf("%w; use config secrets keepassxc connect to check the application pairing", err)
	}
	return KeePassXCBrowser(ref), value, err
}

func (k KeePassXCBrowser) Read() (string, error) {
	return browserClient().Read(context.Background(), keepassxc.Reference(k))
}

func WriteKeePassXCBrowser(ctx context.Context, existing *KeePassXCBrowser, key, account, endpoint, value string) (KeePassXCBrowser, error) {
	var ref *keepassxc.Reference
	if existing != nil {
		converted := keepassxc.Reference(*existing)
		ref = &converted
	}
	saved, err := browserClientForCredential(key).Write(ctx, ref, key, account, endpoint, value)
	return KeePassXCBrowser(saved), err
}

func browserClientForCredential(key string) *keepassxc.Client {
	client := browserClient()
	// KeePassXC derives the title from the URL hostname. A credential-specific
	// URL keeps profiles distinct in one flat group, even with equal usernames.
	host := strings.ToLower(key)
	if host != key {
		// Hostnames are case insensitive; preserve case-sensitive key identity.
		host += fmt.Sprintf("-%x", sha256.Sum256([]byte(key)))
	}
	client.EntryURL = "a-wikimedia-cli://" + host
	return client
}
