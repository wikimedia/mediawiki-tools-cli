package config

import (
	"context"
	"fmt"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func ValidateCredentialProvider(provider string) error {
	if provider != "keyring" && provider != "keepassxc" {
		return fmt.Errorf("credential provider must be keyring or keepassxc")
	}
	return nil
}

func DefaultCredentialProvider() (string, error) {
	provider := State().EffectiveKoanf.String("secrets.provider")
	if provider == "" {
		provider = "keyring"
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
		return c.saveKeyring(value, true, metadata)
	}
	target, err := c.CredentialProvider(provider)
	if err != nil {
		return err
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
	browser, err := writeKeePassXCBrowser(context.Background(), existing, c.Key, c.Account, ref.Endpoint, value)
	if err != nil {
		return err
	}
	if browser.Login != c.Account {
		return fmt.Errorf("saved KeePassXC account did not match; configuration was not changed")
	}
	disk.Delete(c.Key)
	disk.Delete(key)
	if err := disk.Set(key, map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": "keepassxc-browser", "browser": map[string]any{"database_hash": browser.DatabaseHash, "entry_id": browser.EntryID, "login": browser.Login}}); err != nil {
		return err
	}
	return PutDiskConfig(disk)
}

// MigrateTo never overwrites a provider's value with a stale plaintext copy.
func (c Credential) MigrateTo(provider string) error {
	target, err := c.CredentialProvider(provider)
	if err != nil {
		return err
	}
	value := State().OnDiskKoanf.String(c.Key)
	if value == "" {
		return fmt.Errorf("no plaintext credential to migrate")
	}
	if State().OnDiskKoanf.Exists(credentialReferenceKey(c.Key)) {
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
