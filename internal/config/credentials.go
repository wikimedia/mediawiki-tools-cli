package config

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

var (
	credentialStore      secrets.Store = secrets.Keyring{}
	readKeePassXCBrowser               = func(provider secrets.KeePassXCBrowser) (string, error) { return provider.Read() }
	findKeePassXCBrowser               = secrets.FindKeePassXCBrowserEntry
)

type Credential struct {
	Key, Service, Endpoint, Account string
}

type credentialReference struct {
	Service   string                    `koanf:"service"`
	Endpoint  string                    `koanf:"endpoint"`
	Account   string                    `koanf:"account"`
	Identity  string                    `koanf:"identity"`
	Provider  string                    `koanf:"provider"`
	KeePassXC *legacyKeePassXCReference `koanf:"keepassxc"`
	Browser   *secrets.KeePassXCBrowser `koanf:"browser"`
}

// Retain only the old entry path so file-backed links can be switched safely.
type legacyKeePassXCReference struct {
	Entry string `koanf:"entry"`
}

func credentialReferenceKey(key string) string {
	return fmt.Sprintf("credential_storage.%x", sha256.Sum256([]byte(key)))
}

func (c Credential) Validate() error { _, err := c.reference(); return err }

func (c Credential) reference() (credentialReference, error) {
	endpoint, err := secrets.Endpoint(c.Endpoint)
	if err != nil {
		return credentialReference{}, err
	}
	if c.Key == "" || c.Service == "" || c.Account == "" {
		return credentialReference{}, fmt.Errorf("credential needs a service, configuration key and account")
	}
	// Separate credential slots without tying references to a config directory.
	account := fmt.Sprintf("%s:%x", c.Account, sha256.Sum256([]byte(c.Key)))
	return credentialReference{Service: "mwcli:" + c.Service + ":" + endpoint, Endpoint: endpoint, Account: account, Identity: c.Account}, nil
}

func HasCredential(key string) bool {
	state := State()
	return state.EffectiveKoanf.String(key) != "" || state.OnDiskKoanf.Exists(credentialReferenceKey(key))
}

func (c Credential) Read() (string, error) {
	state := State()
	// A saved provider must never fall back to leftover plaintext, even if locked.
	key := credentialReferenceKey(c.Key)
	if !state.OnDiskKoanf.Exists(key) {
		return state.EffectiveKoanf.String(c.Key), nil
	}
	var stored credentialReference
	if err := state.OnDiskKoanf.Unmarshal(key, &stored); err != nil {
		return "", fmt.Errorf("invalid credential reference for %s", c.Key)
	}
	expected, err := c.reference()
	if err != nil {
		return "", err
	}
	if stored.Service != expected.Service || stored.Endpoint != expected.Endpoint || stored.Account != expected.Account || stored.Identity != expected.Identity {
		return "", fmt.Errorf("stored credential does not match this account or endpoint; log in again")
	}
	if stored.Provider == "keepassxc-browser" {
		if stored.Browser == nil || stored.Browser.Login != c.Account {
			return "", fmt.Errorf("invalid KeePassXC GUI account reference")
		}
		return readKeePassXCBrowser(*stored.Browser)
	}
	if stored.Provider == "keepassxc" {
		return "", fmt.Errorf("link this entry through KeePassXC with config secrets use-keepassxc %s", c.Key)
	}

	if stored.Provider != "" && stored.Provider != "keyring" {
		return "", fmt.Errorf("unknown credential provider")
	}
	value, err := credentialStore.Get(stored.Service, stored.Account)
	if err != nil {
		return "", fmt.Errorf("read keyring credential for %s: %w", c.Service, err)
	}
	return value, nil
}

func (c Credential) Save(value string, insecure bool) error {
	return c.SaveWithMetadata(value, insecure, nil)
}

// SaveWithMetadata commits identity and secret metadata in one config write.
func (c Credential) SaveWithMetadata(value string, insecure bool, metadata map[string]string) error {
	return c.saveKeyring(value, insecure, metadata)
}

func (c Credential) saveKeyring(value string, insecure bool, metadata map[string]string) error {
	if value == "" {
		return fmt.Errorf("refusing to save an empty credential")
	}
	ref, err := c.reference()
	if err != nil {
		return err
	}
	disk := State().OnDiskKoanf.Copy()
	for key, value := range metadata {
		if SensitiveKey(key) {
			return fmt.Errorf("secret values cannot be passed as metadata")
		}
		if err := disk.Set(key, value); err != nil {
			return err
		}
	}
	key := credentialReferenceKey(c.Key)
	if disk.Exists(key) {
		var previous credentialReference
		if err := disk.Unmarshal(key, &previous); err != nil {
			return fmt.Errorf("invalid stored credential reference")
		}
		if previous.Provider == "keepassxc" || previous.Provider == "keepassxc-browser" {
			return fmt.Errorf("credential is managed by KeePassXC; update its entry there, or log out before changing providers")
		}
		if previous.Provider == "keyring" {
			previous.Provider = ""
		}
		if previous != ref {
			return fmt.Errorf("log out before changing the account, endpoint or provider of a saved credential")
		}
	}
	if insecure {
		// Do not leave an unreachable copy behind when switching backends.
		if disk.Exists(key) {
			return fmt.Errorf("log out before switching an existing keyring credential to plaintext storage")
		}
		if err := disk.Set(c.Key, value); err != nil {
			return err
		}
		return PutDiskConfig(disk)
	}
	if err := credentialStore.Set(ref.Service, ref.Account, value); err != nil {
		return fmt.Errorf("keyring storage failed; no plaintext fallback was used (use --insecure-storage explicitly on a headless system): %w", err)
	}
	saved, err := credentialStore.Get(ref.Service, ref.Account)
	if err != nil || saved != value {
		return fmt.Errorf("could not verify the saved keyring credential; plaintext configuration was not removed")
	}
	disk.Delete(c.Key)
	if err := disk.Set(key, map[string]interface{}{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity}); err != nil {
		return err
	}
	// Only this successful atomic write removes a legacy plaintext value.
	return PutDiskConfig(disk)
}

func (c Credential) Delete() error {
	disk := State().OnDiskKoanf.Copy()
	key := credentialReferenceKey(c.Key)
	if disk.Exists(key) {
		var ref credentialReference
		if err := disk.Unmarshal(key, &ref); err != nil {
			return err
		}
		expected, err := c.reference()
		if err != nil {
			return err
		}
		if ref.Service != expected.Service || ref.Endpoint != expected.Endpoint || ref.Account != expected.Account || ref.Identity != expected.Identity {
			return fmt.Errorf("stored credential does not match this account or endpoint; refusing to delete another credential")
		}
		switch ref.Provider {
		case "keepassxc", "keepassxc-browser": // Forget the reference; the external entry belongs to the user.
		case "", "keyring":
			if err := credentialStore.Delete(ref.Service, ref.Account); err != nil && !errors.Is(err, secrets.ErrNotFound) {
				return fmt.Errorf("delete keyring credential: %w", err)
			}
		default:
			return fmt.Errorf("unknown credential provider; refusing to delete")
		}
	}
	disk.Delete(c.Key)
	disk.Delete(key)
	return PutDiskConfig(disk)
}

// CredentialForKey covers the credentials historically kept in config.json.
func CredentialForKey(key string) (Credential, error) {
	state := State().OnDiskKoanf
	switch {
	case key == "gerrit.password":
		return Credential{Key: key, Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: state.String("gerrit.username")}, nil
	case strings.HasPrefix(key, "cloud_vps.projects.") && strings.HasSuffix(key, ".credential.secret"):
		project := strings.TrimSuffix(strings.TrimPrefix(key, "cloud_vps.projects."), ".credential.secret")
		if state.String(strings.TrimSuffix(key, "secret")+"id") == "" {
			return Credential{}, fmt.Errorf("application credential ID is missing for project %s", project)
		}
		return Credential{Key: key, Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: project + ":" + state.String(strings.TrimSuffix(key, "secret")+"id")}, nil
	case key == "phabricator.key" || (strings.HasPrefix(key, "phabricator.sites.") && strings.HasSuffix(key, ".key")):
		prefix := strings.TrimSuffix(key, "key")
		endpoint := state.String(prefix + "url")
		if endpoint == "" {
			endpoint = state.String("phabricator.url")
		}
		if endpoint == "" {
			endpoint = secrets.PhabricatorEndpoint
		}
		account := state.String(prefix + "username")
		if account == "" {
			account = state.String("phabricator.username")
		}
		return Credential{Key: key, Service: "phabricator", Endpoint: endpoint, Account: account}, nil
	}
	return Credential{}, fmt.Errorf("no credential provider is defined for %s", key)
}

// UseKeePassXC links a credential through the paired KeePassXC application.
// A direct KeePassXC link can supply the old path without unlocking its file.
func (c Credential) UseKeePassXC(ctx context.Context, entry string) error {
	ref, err := c.reference()
	if err != nil {
		return err
	}
	disk := State().OnDiskKoanf.Copy()
	key := credentialReferenceKey(c.Key)
	if disk.Exists(key) {
		var old credentialReference
		if err := disk.Unmarshal(key, &old); err != nil {
			return fmt.Errorf("invalid credential reference")
		}
		if old.Service != ref.Service || old.Endpoint != ref.Endpoint || old.Account != ref.Account || old.Identity != ref.Identity {
			return fmt.Errorf("stored credential does not match this account or endpoint")
		}
		switch old.Provider {
		case "keepassxc":
			if entry == "" && old.KeePassXC != nil {
				entry = old.KeePassXC.Entry
			}
		case "keepassxc-browser":
			if entry == "" {
				if old.Browser == nil || old.Browser.Login != c.Account {
					return fmt.Errorf("invalid KeePassXC GUI account reference")
				}
				value, err := readKeePassXCBrowser(*old.Browser)
				if err != nil {
					return err
				}
				if previous := disk.String(c.Key); previous != "" && previous != value {
					return fmt.Errorf("KeePassXC entry does not match the saved credential; plaintext was not removed")
				}
				disk.Delete(c.Key)
				return PutDiskConfig(disk)
			}
		default:
			return fmt.Errorf("log out before replacing a different credential provider")
		}
	}
	if entry == "" {
		entry = "wm/" + c.Key
	}
	browser, value, err := findKeePassXCBrowser(ctx, entry)
	if err != nil {
		return err
	}
	if browser.Login != c.Account {
		return fmt.Errorf("KeePassXC entry username does not match the configured account; nothing changed")
	}
	if previous := disk.String(c.Key); previous != "" && previous != value {
		return fmt.Errorf("KeePassXC entry does not match the saved credential; plaintext was not removed")
	}
	values := map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": "keepassxc-browser", "browser": map[string]any{"database_hash": browser.DatabaseHash, "entry_id": browser.EntryID, "login": browser.Login}}
	disk.Delete(c.Key)
	disk.Delete(key)
	if err := disk.Set(key, values); err != nil {
		return err
	}
	return PutDiskConfig(disk)
}

// MigrateTo never overwrites a provider's value with a stale plaintext copy.
func (c Credential) MigrateTo() error {
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
	return c.Save(value, false)
}
