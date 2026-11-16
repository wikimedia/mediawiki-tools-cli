package config

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

var (
	credentialStore      secrets.Store = secrets.Keyring{}
	readKeePassXCBrowser               = func(ctx context.Context, provider secrets.KeePassXCBrowser) (string, error) {
		return provider.ReadContext(ctx)
	}
	findKeePassXCBrowser = secrets.FindKeePassXCBrowserEntry
)

type Credential struct {
	Key, Service, Endpoint, Account string
}

type credentialReference struct {
	Service        string                    `koanf:"service"`
	Endpoint       string                    `koanf:"endpoint"`
	Account        string                    `koanf:"account"`
	StorageAccount string                    `koanf:"storage_account"`
	Identity       string                    `koanf:"identity"`
	Provider       string                    `koanf:"provider"`
	KeePassXC      *legacyKeePassXCReference `koanf:"keepassxc"`
	Browser        *secrets.KeePassXCBrowser `koanf:"browser"`
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
	ref := credentialReference{Service: secrets.CredentialServicePrefix + c.Service + ":" + endpoint, Endpoint: endpoint, Account: account, Identity: c.Account}
	// A renamed configuration key does not move the external keyring item.
	ref.StorageAccount = State().OnDiskKoanf.String(credentialReferenceKey(c.Key) + ".storage_account")
	return ref, nil
}

func HasCredential(key string) bool {
	state := State()
	return state.EffectiveKoanf.String(key) != "" || state.OnDiskKoanf.Exists(credentialReferenceKey(key))
}

func (c Credential) Read() (string, error) { return c.ReadContext(context.Background()) }

// ReadContext carries cancellation through password-manager access.
func (c Credential) ReadContext(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
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
	if stored.Provider == "plaintext" {
		return state.EffectiveKoanf.String(c.Key), nil
	}
	if stored.Provider == "keepassxc-browser" {
		if stored.Browser == nil || stored.Browser.Login != c.Account {
			return "", fmt.Errorf("invalid KeePassXC GUI account reference")
		}
		reference := coreReference(stored)
		store, err := c.packageStore("keepassxc", &reference, nil)
		if err != nil {
			return "", err
		}
		value, err := store.Get(ctx, c.Key)
		return string(value), err
	}
	if stored.Provider == "keepassxc" {
		return "", fmt.Errorf("link this entry through KeePassXC with config secrets use-keepassxc %s", c.Key)
	}

	if stored.Provider != "" && stored.Provider != "keyring" && stored.Provider != secrets.WindowsProvider {
		return "", fmt.Errorf("unknown credential provider")
	}
	reference := coreReference(stored)
	store, err := c.packageStore(reference.Provider, &reference, nil)
	if err != nil {
		return "", err
	}
	value, err := store.Get(ctx, c.Key)
	if err != nil {
		return "", fmt.Errorf("read keyring credential for %s: %w", c.Service, err)
	}
	return string(value), nil
}

func (c Credential) Save(value string, insecure bool) error {
	return c.SaveWithMetadata(value, insecure, nil)
}

// SaveWithMetadata commits identity and secret metadata in one config write.
func (c Credential) SaveWithMetadata(value string, insecure bool, metadata map[string]string) error {
	return c.SaveWithProvider(value, insecure, metadata, "")
}

func (c Credential) saveKeyring(value string, insecure bool, metadata map[string]any) error {
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
	plaintextLinked := false
	if disk.Exists(key) {
		var previous credentialReference
		if err := disk.Unmarshal(key, &previous); err != nil {
			return fmt.Errorf("invalid stored credential reference")
		}
		if previous.Provider == "keepassxc" || previous.Provider == "keepassxc-browser" {
			return fmt.Errorf("credential is managed by KeePassXC; use config secrets transfer %s --provider PROVIDER to change providers", c.Key)
		}
		plaintextLinked = previous.Provider == "plaintext"
		if plaintextLinked && !insecure {
			return fmt.Errorf("credential uses plaintext; use config secrets transfer to change providers")
		}
		if previous.Provider == "keyring" || plaintextLinked {
			previous.Provider = ""
		}
		if previous != ref {
			return fmt.Errorf("credential is bound to another account, endpoint or provider; use a separate profile or auth remove --profile NAME before logging in again; use config secrets transfer %s --provider PROVIDER to change providers", c.Key)
		}
	}
	if insecure {
		// Do not leave an unreachable copy behind when switching backends.
		if disk.Exists(key) && !plaintextLinked {
			return fmt.Errorf("use config secrets transfer %s --provider plaintext to switch an existing keyring credential to plaintext storage", c.Key)
		}
		if err := disk.Set(c.Key, value); err != nil {
			return err
		}
		if err := disk.Set(key, map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": "plaintext"}); err != nil {
			return err
		}
		return commitDiskConfig(disk)
	}
	reference := coreReference(ref)
	if !disk.Exists(key) {
		previous, err := credentialStore.Get(ref.Service, reference.Fields["account"])
		if err == nil && previous != value {
			return secretstore.ErrConflict
		}
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return secretstore.SafeError("get", "keyring", err)
		}
	}
	store, err := c.packageStore("keyring", &reference, func(secretstore.Entry) error {
		disk.Delete(c.Key)
		if err := disk.Set(key, map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "storage_account": ref.StorageAccount}); err != nil {
			return err
		}
		return commitDiskConfig(disk)
	})
	if err != nil {
		return err
	}
	return store.Set(context.Background(), c.Key, []byte(value))
}

func (c Credential) Delete() error {
	return UpdateDiskConfig(c.deleteFromDisk)
}

// RemoveCredentialProfile removes a credential and its profile metadata in one
// locked transaction, so a concurrent login cannot leave an orphaned reference.
func RemoveCredentialProfile(key string, update func(*koanf.Koanf) error) error {
	return UpdateDiskConfig(func(disk *koanf.Koanf) error {
		if HasCredential(key) {
			credential, err := CredentialForKey(key)
			if err != nil {
				return err
			}
			if err := credential.deleteFromDisk(disk); err != nil {
				return err
			}
		}
		return update(disk)
	})
}

func (c Credential) deleteFromDisk(disk *koanf.Koanf) error {
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
		case "plaintext", "keepassxc", "keepassxc-browser": // Forget the reference; external entries belong to the user.
			if ref.Provider == "keepassxc-browser" {
				if err := retainCredential(disk, c.Key, "profile removed", ref); err != nil {
					return err
				}
			}
		case "", "keyring", secrets.WindowsProvider:
			client, err := keyringStore(ref.Provider)
			if err != nil {
				return err
			}
			if err := client.Delete(ref.Service, coreReference(ref).Fields["account"]); err != nil && !errors.Is(err, secrets.ErrNotFound) {
				return fmt.Errorf("delete keyring credential: %w", err)
			}
			if id, err := physicalCredentialID(ref); err == nil {
				disk.Delete("credential_retired." + id)
			}
		default:
			return fmt.Errorf("unknown credential provider; refusing to delete")
		}
	}
	disk.Delete(c.Key)
	disk.Delete(key)
	return nil
}

// Update serializes reading, transforming and saving an existing credential.
// The callback must not perform another configuration mutation.
func (c Credential) Update(ctx context.Context, update func(string) (string, error)) (string, error) {
	unlock, err := lockCredentials(ctx)
	if err != nil {
		return "", err
	}
	defer unlock()
	if !HasCredential(c.Key) {
		return "", fmt.Errorf("saved credential was removed; log in again")
	}
	previous, err := c.ReadContext(ctx)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	value, err := update(previous)
	if err != nil {
		return "", err
	}
	// A successful transformation may already have rotated a server-side token.
	// Persist it even if cancellation arrived just after the response.
	if value != previous {
		if err := c.saveWithProviderMetadata(value, false, nil, ""); err != nil {
			return "", fmt.Errorf("could not save updated credential: %w", err)
		}
	}
	return value, nil
}

// CredentialForKey covers the credentials historically kept in config.json.
func CredentialForKey(key string) (Credential, error) {
	state := State().OnDiskKoanf
	switch {
	case strings.HasPrefix(key, "enterprise.profiles.") && strings.HasSuffix(key, ".token"):
		parts := strings.Split(key, ".")
		if len(parts) != 4 || parts[2] == "" {
			break
		}
		return Credential{Key: key, Service: "enterprise", Endpoint: "https://api.enterprise.wikimedia.com", Account: state.String(strings.TrimSuffix(key, "token") + "username")}, nil
	case strings.HasPrefix(key, "wiki.profiles."):
		parts := strings.Split(key, ".")
		if len(parts) != 5 || (parts[4] != "password" && parts[4] != "token") {
			break
		}
		account, err := LoadWikiProfile(parts[2], parts[3])
		if err != nil {
			return Credential{}, err
		}
		credential := account.Credential()
		if credential.Key != key {
			return Credential{}, fmt.Errorf("credential key does not match the wiki profile authentication method")
		}
		return credential, nil
	case strings.HasPrefix(key, "gitlab.profiles.") && strings.HasSuffix(key, ".token"):
		prefix := strings.TrimSuffix(key, "token")
		endpoint := state.String(prefix + "url")
		if endpoint == "" {
			endpoint = "https://gitlab.wikimedia.org"
		}
		return Credential{Key: key, Service: "gitlab", Endpoint: endpoint, Account: state.String(prefix + "username")}, nil
	case strings.HasPrefix(key, "gerrit.profiles.") && strings.HasSuffix(key, ".password"):
		return Credential{Key: key, Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: state.String(strings.TrimSuffix(key, "password") + "username")}, nil
	case key == "gerrit.password":
		return Credential{Key: key, Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: state.String("gerrit.username")}, nil
	case strings.HasPrefix(key, "cloud_vps.profiles.") && strings.HasSuffix(key, ".secret"):
		prefix := strings.TrimSuffix(key, "secret")
		project, id := state.String(prefix+"project"), state.String(prefix+"id")
		if project == "" || id == "" {
			return Credential{}, fmt.Errorf("Cloud VPS profile needs a project and application credential ID")
		}
		return Credential{Key: key, Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: project + ":" + id}, nil
	case strings.HasPrefix(key, "cloud_vps.projects.") && strings.HasSuffix(key, ".credential.secret"):
		project := strings.TrimSuffix(strings.TrimPrefix(key, "cloud_vps.projects."), ".credential.secret")
		if state.String(strings.TrimSuffix(key, "secret")+"id") == "" {
			return Credential{}, fmt.Errorf("application credential ID is missing for project %s", project)
		}
		return Credential{Key: key, Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: project + ":" + state.String(strings.TrimSuffix(key, "secret")+"id")}, nil
	case key == "phabricator.key" || (strings.HasPrefix(key, "phabricator.profiles.") && strings.HasSuffix(key, ".key")):
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
	unlock, lockErr := lockCredentials(ctx)
	if lockErr != nil {
		return lockErr
	}
	defer unlock()

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
		case "plaintext": // Explicit linking verifies the plaintext below.
		case "keepassxc":
			if entry == "" && old.KeePassXC != nil {
				entry = old.KeePassXC.Entry
			}
		case "keepassxc-browser":
			if entry == "" {
				if old.Browser == nil || old.Browser.Login != c.Account {
					return fmt.Errorf("invalid KeePassXC GUI account reference")
				}
				value, err := readKeePassXCBrowser(context.Background(), *old.Browser)
				if err != nil {
					return err
				}
				if previous := disk.String(c.Key); previous != "" && previous != value {
					return fmt.Errorf("KeePassXC entry does not match the saved credential; plaintext was not removed")
				}
				disk.Delete(c.Key)
				return commitDiskConfig(disk)
			}
		default:
			return fmt.Errorf("use config secrets transfer %s --provider keepassxc to change providers", c.Key)
		}
	}
	if entry == "" {
		entry = secrets.KeePassXCGroup + "/" + c.Key
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
	if disk.Exists(key) {
		var old credentialReference
		if err := disk.Unmarshal(key, &old); err != nil {
			return err
		}
		if old.Provider == "keepassxc-browser" {
			if err := retainCredential(disk, c.Key, "relinked to another entry", old); err != nil {
				return err
			}
		}
	}
	values := map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": "keepassxc-browser", "browser": map[string]any{"database_hash": browser.DatabaseHash, "entry_id": browser.EntryID, "login": browser.Login}}
	disk.Delete(c.Key)
	disk.Delete(key)
	if err := disk.Set(key, values); err != nil {
		return err
	}
	return commitDiskConfig(disk)
}
