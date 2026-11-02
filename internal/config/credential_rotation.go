package config

import (
	"context"
	"fmt"
)

// Replace changes an explicitly selected credential's account binding. Callers
// must validate the replacement against its service before calling this method.
// The key, service and endpoint remain fixed; the previous account must still
// match current profile metadata when the configuration lock is acquired.
func (c Credential) Replace(previous Credential, value string, insecure bool, metadata map[string]string, provider string) error {
	unlock, err := lockCredentials(context.Background())
	if err != nil {
		return err
	}
	defer unlock()
	if c.Key != previous.Key || c.Service != previous.Service || c.Endpoint != previous.Endpoint {
		return fmt.Errorf("credential replacement must keep the same key, service and endpoint")
	}
	current, err := CredentialForKey(c.Key)
	if err != nil {
		return err
	}
	if current != previous {
		return fmt.Errorf("profile changed before credential replacement; retry login")
	}
	expected, err := previous.reference()
	if err != nil {
		return err
	}
	target, err := previous.CredentialProvider(provider)
	if err != nil {
		return err
	}
	if insecure {
		if provider != "" {
			return fmt.Errorf("a provider cannot be combined with insecure storage")
		}
		if HasCredential(c.Key) && State().OnDiskKoanf.Exists(credentialReferenceKey(c.Key)) && target != "plaintext" {
			return fmt.Errorf("use config secrets transfer %s --provider plaintext before changing storage", c.Key)
		}
		target = "plaintext"
	}
	typed := make(map[string]any, len(metadata))
	for key, value := range metadata {
		typed[key] = value
	}
	if c.Account == previous.Account {
		return c.saveWithProviderMetadata(value, false, typed, target)
	}
	original := State().OnDiskKoanf.Copy()
	key := credentialReferenceKey(c.Key)
	var source *credentialReference
	if original.Exists(key) {
		var ref credentialReference
		if err := original.Unmarshal(key, &ref); err != nil {
			return fmt.Errorf("invalid source credential reference")
		}
		if ref.Service != expected.Service || ref.Endpoint != expected.Endpoint || ref.Account != expected.Account || ref.Identity != expected.Identity {
			return fmt.Errorf("stored credential does not match the profile; replacement refused")
		}
		if ref.Provider == "keepassxc" {
			return fmt.Errorf("relink this legacy entry through KeePassXC before replacement")
		}
		source = &ref
	}
	working := original.Copy()
	working.Delete(key)
	working.Delete(c.Key)
	if source != nil {
		if err := retainCredential(working, c.Key, "credential replaced", *source); err != nil {
			return err
		}
	}
	// This temporary in-memory view lets the normal verified save create a new
	// account slot. The on-disk reference stays untouched until that save commits.
	if err := ReApplyKoanfConf(working); err != nil {
		return err
	}
	if err := c.saveWithProviderMetadata(value, false, typed, target); err != nil {
		if restoreErr := ReApplyKoanfConf(original); restoreErr != nil {
			return fmt.Errorf("%w; restore configuration: %v", err, restoreErr)
		}
		return err
	}
	if source != nil && source.Provider != "plaintext" {
		id, err := physicalCredentialID(*source)
		if err != nil {
			return fmt.Errorf("replacement saved, but source cleanup could not be identified: %w", err)
		}
		active, err := activeCredential(State().OnDiskKoanf, *source)
		if err == nil && !active {
			err = removeDetachedEntry(context.Background(), detachedInfo(id, c.Key, "credential replaced", "", "tracked", *source))
		}
		if err != nil {
			return fmt.Errorf("replacement saved; source cleanup failed and its record was retained. Use config secrets detached: %w", err)
		}
	}
	return nil
}
