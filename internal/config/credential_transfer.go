package config

import (
	"context"
	"errors"
	"fmt"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

// TransferTo deliberately changes a saved provider, retaining the source entry.
// The destination must verify before its reference replaces the source reference.
func (c Credential) TransferTo(provider string) error {
	if err := ValidateCredentialProvider(provider); err != nil {
		return err
	}
	key := credentialReferenceKey(c.Key)
	disk := State().OnDiskKoanf.Copy()
	if !disk.Exists(key) {
		return fmt.Errorf("no linked credential; use config secrets migrate for plaintext credentials")
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
	missing := false
	if provider == "keyring" {
		destination, err = credentialStore.Get(ref.Service, ref.Account)
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
	if provider == "keyring" {
		if !missing && destination != value {
			return fmt.Errorf("destination keyring already contains a different value; nothing was overwritten")
		}
		if missing {
			if err := credentialStore.Set(ref.Service, ref.Account, value); err != nil {
				return fmt.Errorf("destination keyring write failed; source reference retained: %w", err)
			}
		}
		verified, err := credentialStore.Get(ref.Service, ref.Account)
		if err != nil || verified != value {
			return fmt.Errorf("destination keyring did not verify; source reference retained")
		}
		saved["provider"] = "keyring"
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
	disk.Delete(key) // Koanf otherwise merges the old provider's metadata.
	if err := disk.Set(key, saved); err != nil {
		return err
	}
	if err := PutDiskConfig(disk); err != nil {
		return fmt.Errorf("destination was saved but reference update failed; source entry retained: %w", err)
	}
	return nil
}
