package keepassxc

import "context"

// Keep the protocol regression fixtures readable while exercising instance APIs.
var testClient *Client

func PairKeePassXCBrowser(ctx context.Context, proxy string) error {
	return testClient.Pair(ctx, proxy)
}
func loadBrowserPairing() (browserPairing, error) { return testClient.loadBrowserPairing() }
func saveBrowserPairing(p browserPairing) error   { return testClient.saveBrowserPairing(p) }
func pairingDirectory(create bool) error          { return testClient.pairingDirectory(create) }
func KeePassXCPairingPath() string                { return testClient.PairingFile }
func KeePassXCBrowserStatus() (string, error)     { return testClient.Status() }
func FindKeePassXCBrowserEntry(ctx context.Context, path string) (Reference, string, error) {
	return testClient.Find(ctx, path)
}
func WriteKeePassXCBrowser(ctx context.Context, existing *Reference, key, account, endpoint, value string) (Reference, error) {
	return testClient.Write(ctx, existing, key, account, endpoint, value)
}
func DetectKeePassXC() Availability { return testClient.Detect() }

func ForgetKeePassXCBrowser() error { return testClient.Disconnect() }
