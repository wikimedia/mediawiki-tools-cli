package secrets

import (
	"context"
	"fmt"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keepassxc"
)

// App paths stay here; the reusable provider has no CLI dependencies.
type KeePassXCBrowser keepassxc.Reference

func browserClient() *keepassxc.Client {
	return &keepassxc.Client{PairingFile: KeePassXCPairingPath(), Group: "wm"}
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
	saved, err := browserClient().Write(ctx, ref, key, account, endpoint, value)
	return KeePassXCBrowser(saved), err
}
