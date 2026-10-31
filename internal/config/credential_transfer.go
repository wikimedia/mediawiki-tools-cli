package config

import (
	"context"
	"errors"
	"fmt"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

type TransferOptions struct{ KeepSource bool }

// TransferTo moves a saved credential after verifying and persisting its destination.
// The destination must verify before its reference replaces the source reference.
func (c Credential) TransferTo(provider string, options ...TransferOptions) error {
	if len(options) > 1 {
		return fmt.Errorf("provide at most one set of transfer options")
	}
	keepSource := len(options) == 1 && options[0].KeepSource
	unlock, lockErr := lockCredentials(context.Background())
	if lockErr != nil {
		return lockErr
	}
	defer unlock()

	if err := ValidateCredentialProvider(provider); err != nil {
		return err
	}
	key := credentialReferenceKey(c.Key)
	disk := State().OnDiskKoanf.Copy()
	if !disk.Exists(key) {
		return fmt.Errorf("no linked credential; use config secrets migrate for plaintext credentials")
	}
	var source credentialReference
	if err := disk.Unmarshal(key, &source); err != nil {
		return fmt.Errorf("invalid source credential reference")
	}
	current, err := c.CredentialProvider("")
	if err != nil {
		return err
	}
	if current == provider {
		return fmt.Errorf("credential already uses %s", provider)
	}
	ref, err := c.reference()
	if err != nil {
		return err
	}

	// Check destination availability before opening the source GUI. Never replace
	// an existing keyring value with a different credential during a transfer.
	var destination string
	var destinationStore secrets.Store
	if provider == secrets.WindowsProvider {
		if plaintext := disk.String(c.Key); len(plaintext) > 2560 {
			return fmt.Errorf("credential exceeds the Windows Credential Manager limit")
		}
	}

	missing := false
	if provider == "keyring" || provider == secrets.WindowsProvider {
		destinationStore, err = keyringStore(provider)
		if err != nil {
			return err
		}
		destination, err = destinationStore.Get(ref.Service, ref.Account)
		missing = errors.Is(err, secrets.ErrNotFound)
		if err != nil && !missing {
			return fmt.Errorf("destination keyring is unavailable; source reference retained: %w", err)
		}
	}
	value, err := c.Read()
	if err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("source credential is empty; nothing changed")
	}
	if plaintext := disk.String(c.Key); plaintext != "" && plaintext != value {
		return fmt.Errorf("plaintext differs from the linked credential; neither copy was changed")
	}

	saved := map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity}
	if provider == "keyring" || provider == secrets.WindowsProvider {
		if provider == secrets.WindowsProvider && len(value) > 2560 {
			return fmt.Errorf("credential exceeds the Windows Credential Manager limit; source reference retained")
		}
		if !missing && destination != value {
			return fmt.Errorf("destination keyring already contains a different value; nothing was overwritten")
		}
		if missing {
			if err := destinationStore.Set(ref.Service, ref.Account, value); err != nil {
				return fmt.Errorf("destination keyring write failed; source reference retained: %w", err)
			}
		}
		verified, err := destinationStore.Get(ref.Service, ref.Account)
		if err != nil || verified != value {
			return fmt.Errorf("destination keyring did not verify; source reference retained")
		}
		saved["provider"] = provider
	} else if provider == "plaintext" {
		WarnPlaintextStorage()
		saved["provider"] = "plaintext"
	} else {
		browser, err := writeKeePassXCBrowser(context.Background(), nil, c.Key, c.Account, ref.Endpoint, value)
		if err != nil {
			return err
		}
		if browser.Login != c.Account {
			return fmt.Errorf("destination KeePassXC account did not match; source reference retained")
		}
		saved["provider"] = "keepassxc-browser"
		saved["browser"] = map[string]any{"database_hash": browser.DatabaseHash, "entry_id": browser.EntryID, "login": browser.Login}
	}
	if provider == "plaintext" {
		if err := disk.Set(c.Key, value); err != nil {
			return err
		}
	} else {
		disk.Delete(c.Key)
	}
	if err := retainCredential(disk, c.Key, "transferred to "+provider, source); err != nil {
		return err
	}
	disk.Delete(key) // Koanf otherwise merges the old provider's metadata.
	if err := disk.Set(key, saved); err != nil {
		return err
	}
	if err := PutDiskConfig(disk); err != nil {
		return fmt.Errorf("destination was saved but reference update failed; source entry retained: %w", err)
	}
	if !keepSource && source.Provider != "plaintext" {
		id, err := physicalCredentialID(source)
		if err != nil {
			return fmt.Errorf("transfer completed, but source cleanup could not be identified; review retained entries: %w", err)
		}
		active, err := activeCredential(State().OnDiskKoanf, source)
		if err != nil {
			return fmt.Errorf("transfer completed, but source cleanup could not check other references: %w", err)
		}
		if !active {
			entry := detachedInfo(id, c.Key, "transferred to "+provider, "", "tracked", source)
			if err := removeDetachedEntry(context.Background(), entry); err != nil {
				return fmt.Errorf("transfer to %s completed; source cleanup failed and its record was retained. Use config secrets detached: %w", provider, err)
			}
		}
	}
	return nil
}
