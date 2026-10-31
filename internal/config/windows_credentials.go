package config

import (
	"context"
	"errors"
	"fmt"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

func (c Credential) saveWindowsCredential(value string, metadata map[string]any) error {
	if value == "" {
		return fmt.Errorf("refusing to save an empty credential")
	}
	if len(value) > 2560 {
		return secretstore.ErrTooLarge
	}
	ref, err := c.reference()
	if err != nil {
		return err
	}
	ref.Provider = secrets.WindowsProvider
	disk := State().OnDiskKoanf.Copy()
	key := credentialReferenceKey(c.Key)
	if disk.Exists(key) {
		var previous credentialReference
		if err := disk.Unmarshal(key, &previous); err != nil || previous != ref {
			return fmt.Errorf("stored credential does not match the account, endpoint or provider; use config secrets transfer to change providers")
		}
	}
	for key, value := range metadata {
		if err := disk.Set(key, value); err != nil {
			return err
		}
	}
	reference := coreReference(ref)
	if !disk.Exists(key) {
		client, err := keyringStore(secrets.WindowsProvider)
		if err != nil {
			return err
		}
		old, err := client.Get(ref.Service, reference.Fields["account"])
		if err == nil && old != value {
			return secretstore.ErrConflict
		}
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return secretstore.SafeError("get", secrets.WindowsProvider, err)
		}
	}
	store, err := c.packageStore(secrets.WindowsProvider, &reference, func(secretstore.Entry) error {
		disk.Delete(c.Key)
		disk.Delete(key)
		if err := disk.Set(key, map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "storage_account": ref.StorageAccount, "provider": ref.Provider}); err != nil {
			return err
		}
		return PutDiskConfig(disk)
	})
	if err != nil {
		return err
	}
	return store.Set(context.Background(), c.Key, []byte(value))
}
