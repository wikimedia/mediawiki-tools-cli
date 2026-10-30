package config

import (
	"context"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keepassxc"
	native "gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keyring"
)

// The CLI's existing config is its metadata registry. Only this adapter knows
// about koanf or service identity; provider operations use the reusable store.
type credentialRegistry struct {
	*secretstore.MemoryRegistry
	persist func(secretstore.Entry) error
}

func (r *credentialRegistry) Put(ctx context.Context, entry secretstore.Entry) error {
	if r.persist != nil {
		if err := r.persist(entry); err != nil {
			return err
		}
	}
	return r.MemoryRegistry.Put(ctx, entry)
}

type credentialBrowser struct{}

func (credentialBrowser) Read(ctx context.Context, ref keepassxc.Reference) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return readKeePassXCBrowser(secrets.KeePassXCBrowser(ref))
}
func (credentialBrowser) Write(ctx context.Context, existing *keepassxc.Reference, key, account, endpoint, value string) (keepassxc.Reference, error) {
	var previous *secrets.KeePassXCBrowser
	if existing != nil {
		converted := secrets.KeePassXCBrowser(*existing)
		previous = &converted
	}
	ref, err := writeKeePassXCBrowser(ctx, previous, key, account, endpoint, value)
	return keepassxc.Reference(ref), err
}

func (c Credential) packageStore(provider string, existing *secretstore.Reference, persist func(secretstore.Entry) error) (*secretstore.Store, error) {
	r := &credentialRegistry{MemoryRegistry: secretstore.NewMemoryRegistry(), persist: persist}
	if existing != nil {
		if err := r.MemoryRegistry.Put(context.Background(), secretstore.Entry{Namespace: cli.APPNAME, Key: c.Key, Reference: *existing}); err != nil {
			return nil, err
		}
	}
	return secretstore.Open(secretstore.Config{Namespace: cli.APPNAME, Provider: provider, Registry: r, Backends: []secretstore.Backend{&native.Backend{Client: credentialStore}, keepassxc.New(credentialBrowser{})}})
}

func coreReference(ref credentialReference) secretstore.Reference {
	if ref.Provider == "keepassxc-browser" && ref.Browser != nil {
		return secretstore.Reference{Provider: "keepassxc", Fields: map[string]string{"database_hash": ref.Browser.DatabaseHash, "entry_id": ref.Browser.EntryID, "login": ref.Browser.Login}}
	}
	return secretstore.Reference{Provider: "keyring", Owned: true, Fields: map[string]string{"service": ref.Service, "account": ref.Account}}
}
