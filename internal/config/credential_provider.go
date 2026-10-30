package config

import (
	"context"
	"fmt"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

func ValidateCredentialProvider(provider string) error {
	if provider != "keyring" && provider != "keepassxc" && provider != "plaintext" {
		return fmt.Errorf("credential provider must be keyring, keepassxc or plaintext")
	}
	return nil
}

func DefaultCredentialProvider() (string, error) {
	provider := State().EffectiveKoanf.String("secrets.provider")
	if provider == "" {
		return "", nil
	}
	return provider, ValidateCredentialProvider(provider)
}

// CredentialProvider resolves an override, an existing link, then the default.
// Overrides never silently replace a previously bound credential provider.
func (c Credential) CredentialProvider(override string) (string, error) {
	if override != "" {
		if err := ValidateCredentialProvider(override); err != nil {
			return "", err
		}
	}
	key := credentialReferenceKey(c.Key)
	if State().OnDiskKoanf.Exists(key) {
		var stored credentialReference
		if err := State().OnDiskKoanf.Unmarshal(key, &stored); err != nil {
			return "", fmt.Errorf("invalid credential reference")
		}
		provider := stored.Provider
		switch provider {
		case "", "keyring":
			provider = "keyring"
		case "plaintext":
			provider = "plaintext"
		case "keepassxc", "keepassxc-browser":
			provider = "keepassxc"
		default:
			return "", fmt.Errorf("unknown credential provider")
		}
		if override != "" && override != provider {
			return "", fmt.Errorf("credential already uses %s; log out before changing providers", provider)
		}
		return provider, nil
	}
	if override != "" {
		return override, nil
	}
	return DefaultCredentialProvider()
}

var writeKeePassXCBrowser = secrets.WriteKeePassXCBrowser

func (c Credential) SaveWithProvider(value string, insecure bool, metadata map[string]string, provider string) error {
	if insecure {
		if provider != "" {
			return fmt.Errorf("a provider cannot be combined with insecure storage")
		}
		WarnPlaintextStorage()
		return c.saveKeyring(value, true, metadata)
	}
	target, err := c.CredentialProvider(provider)
	if err != nil {
		return err
	}
	if target == "" {
		if credentialStorageInteractive() {
			target, err = chooseCredentialProvider()
			if err != nil {
				return err
			}
			if err := ValidateCredentialProvider(target); err != nil {
				return err
			}
			selectedMetadata := make(map[string]string, len(metadata)+1)
			for key, value := range metadata {
				selectedMetadata[key] = value
			}
			selectedMetadata["secrets.provider"] = target
			metadata = selectedMetadata
		} else {
			target = "keyring"
		}
	}
	if target == "plaintext" {
		WarnPlaintextStorage()
		return c.saveKeyring(value, true, metadata)
	}
	if target == "keyring" {
		return c.saveKeyring(value, false, metadata)
	}
	if value == "" {
		return fmt.Errorf("refusing to save an empty credential")
	}
	ref, err := c.reference()
	if err != nil {
		return err
	}
	disk := State().OnDiskKoanf.Copy()
	var existing *secrets.KeePassXCBrowser
	key := credentialReferenceKey(c.Key)
	if disk.Exists(key) {
		var old credentialReference
		if err := disk.Unmarshal(key, &old); err != nil {
			return fmt.Errorf("invalid credential reference")
		}
		if old.Service != ref.Service || old.Endpoint != ref.Endpoint || old.Account != ref.Account || old.Identity != ref.Identity {
			return fmt.Errorf("stored credential does not match this account or endpoint")
		}
		if old.Provider != "keepassxc-browser" || old.Browser == nil || old.Browser.Login != c.Account {
			return fmt.Errorf("relink this entry first with config secrets use-keepassxc %s", c.Key)
		}
		existing = old.Browser
	}
	for key, value := range metadata {
		if SensitiveKey(key) {
			return fmt.Errorf("secret values cannot be passed as metadata")
		}
		if err := disk.Set(key, value); err != nil {
			return err
		}
	}
	var previous *secretstore.Reference
	if existing != nil {
		converted := coreReference(credentialReference{Provider: "keepassxc-browser", Browser: existing})
		previous = &converted
	}
	store, err := c.packageStore("keepassxc", previous, func(entry secretstore.Entry) error {
		fields := entry.Reference.Fields
		disk.Delete(c.Key)
		disk.Delete(key)
		if err := disk.Set(key, map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": "keepassxc-browser", "browser": map[string]any{"database_hash": fields["database_hash"], "entry_id": fields["entry_id"], "login": fields["login"]}}); err != nil {
			return err
		}
		return PutDiskConfig(disk)
	})
	if err != nil {
		return err
	}
	return store.SetWithMetadata(context.Background(), c.Key, []byte(value), map[string]string{"account": c.Account, "endpoint": ref.Endpoint})
}

// MigrationProvider allows an explicitly requested migration to upgrade a
// plaintext binding while ordinary saves continue to respect that binding.
func (c Credential) MigrationProvider(override string) (string, error) {
	current, err := c.CredentialProvider("")
	if err != nil {
		return "", err
	}
	if current == "plaintext" {
		if override != "" {
			return override, ValidateCredentialProvider(override)
		}
		target, err := DefaultCredentialProvider()
		if err != nil {
			return "", err
		}
		if target == "" {
			target = "plaintext"
		}
		return target, nil
	}
	return c.CredentialProvider(override)
}

// MigrateTo never overwrites a provider's value with a stale plaintext copy.
func (c Credential) MigrateTo(provider string) error {
	target, err := c.MigrationProvider(provider)
	if err != nil {
		return err
	}
	value := State().OnDiskKoanf.String(c.Key)
	if value == "" {
		return fmt.Errorf("no plaintext credential to migrate")
	}
	if target == "plaintext" {
		return c.SaveWithProvider(value, false, nil, target)
	}
	if State().OnDiskKoanf.Exists(credentialReferenceKey(c.Key)) {
		current, err := c.CredentialProvider("")
		if err != nil {
			return err
		}
		if current == "plaintext" && target != "plaintext" {
			return c.TransferTo(target)
		}
		saved, err := c.Read()
		if err != nil {
			return err
		}
		if saved != value {
			return fmt.Errorf("saved provider credential does not match plaintext; neither copy was changed")
		}
		disk := State().OnDiskKoanf.Copy()
		disk.Delete(c.Key)
		return PutDiskConfig(disk)
	}
	return c.SaveWithProvider(value, false, nil, target)
}

// ReplaceFromPlaintext is an explicit choice of the local plaintext value as
// authoritative. The normal provider write verifies its destination before
// removing plaintext, and preserves the existing account/endpoint binding.
func (c Credential) ReplaceFromPlaintext(provider string) error {
	value := State().OnDiskKoanf.String(c.Key)
	if value == "" {
		return fmt.Errorf("no plaintext credential to migrate")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	target, err := c.MigrationProvider(provider)
	if err != nil {
		return err
	}
	if target != "keyring" && target != "keepassxc" {
		return fmt.Errorf("replacement requires a secure destination provider; select keyring or keepassxc")
	}
	if State().OnDiskKoanf.Exists(credentialReferenceKey(c.Key)) {
		current, err := c.CredentialProvider("")
		if err != nil {
			return err
		}
		if current == "plaintext" {
			return c.MigrateTo(target)
		}
	}
	return c.SaveWithProvider(value, false, nil, target)
}
