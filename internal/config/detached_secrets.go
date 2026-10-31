package config

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	native "gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keyring"
)

// DetachedSecret exposes identifiers only. It never contains a secret value.
type DetachedSecret struct {
	ID         string `json:"id"`
	Key        string `json:"key,omitempty"`
	Provider   string `json:"provider"`
	Account    string `json:"account,omitempty"`
	Endpoint   string `json:"endpoint,omitempty"`
	Reason     string `json:"reason"`
	RetainedAt string `json:"retained_at,omitempty"`
	Service    string `json:"service,omitempty"`
	Target     string `json:"target,omitempty"`
	DatabaseID string `json:"database_id,omitempty"`
	EntryID    string `json:"entry_id,omitempty"`
	Source     string `json:"source"`
	ref        credentialReference
}

func physicalCredentialID(ref credentialReference) (string, error) {
	provider := ref.Provider
	if provider == "plaintext" {
		return "", nil
	}
	if provider == "" {
		provider = "keyring"
	}
	var identity []string
	switch provider {
	case "keyring", secrets.WindowsProvider:
		if runtime.GOOS == "windows" {
			provider = secrets.WindowsProvider
		}
		account := coreReference(ref).Fields["account"]
		if ref.Service == "" || account == "" {
			return "", fmt.Errorf("invalid native credential reference")
		}
		if provider == secrets.WindowsProvider {
			// Windows identifies a generic credential by the flattened target name.
			identity = []string{provider, ref.Service + ":" + account}
		} else {
			identity = []string{provider, ref.Service, account}
		}
	case "keepassxc-browser":
		if ref.Browser == nil || ref.Browser.DatabaseHash == "" || ref.Browser.EntryID == "" {
			return "", fmt.Errorf("invalid KeePassXC reference")
		}
		identity = []string{"keepassxc", ref.Browser.DatabaseHash, ref.Browser.EntryID}
	default:
		return "", fmt.Errorf("credential reference has no supported cleanup identity")
	}
	data, _ := json.Marshal(identity)
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func retainCredential(disk *koanf.Koanf, key, reason string, ref credentialReference) error {
	if ref.Provider == "plaintext" || ref.Provider == "keepassxc" {
		return nil
	}
	id, err := physicalCredentialID(ref)
	if err != nil || id == "" {
		return err
	}
	// Serialize only the credential-reference struct, never a provider value.
	return disk.Set("credential_retired."+id, map[string]any{
		"key": key, "reason": reason, "retained_at": time.Now().UTC().Format(time.RFC3339), "reference": ref,
	})
}

func activeCredential(disk *koanf.Koanf, ref credentialReference) (bool, error) {
	id, err := physicalCredentialID(ref)
	if err != nil {
		return false, err
	}
	for _, key := range disk.MapKeys("credential_storage") {
		var active credentialReference
		if disk.Unmarshal("credential_storage."+key, &active) != nil {
			return false, fmt.Errorf("invalid active credential reference; cleanup refused")
		}
		// Legacy path-only KeePassXC links cannot be compared to a UUID safely.
		if active.Provider == "keepassxc" && ref.Provider == "keepassxc-browser" {
			return false, fmt.Errorf("relink legacy KeePassXC references before cleanup")
		}
		if active.Provider == "plaintext" || active.Provider == "keepassxc" {
			continue
		}
		other, err := physicalCredentialID(active)
		if err != nil {
			return false, fmt.Errorf("unrecognized active credential reference; cleanup refused")
		}
		if id == other {
			return true, nil
		}
	}
	return false, nil
}

func detachedInfo(id, key, reason, retainedAt, source string, ref credentialReference) DetachedSecret {
	provider := ref.Provider
	if provider == "" {
		provider = "keyring"
	}
	entry := DetachedSecret{ID: id, Key: key, Provider: provider, Account: ref.Identity, Endpoint: ref.Endpoint, Service: ref.Service, Reason: reason, RetainedAt: retainedAt, Source: source, ref: ref}
	if provider == "keepassxc-browser" && ref.Browser != nil {
		entry.Provider = "keepassxc"
		entry.DatabaseID, entry.EntryID = ref.Browser.DatabaseHash, ref.Browser.EntryID
	} else {
		entry.Target = ref.Service + ":" + coreReference(ref).Fields["account"]
		if entry.Account == "" {
			entry.Account = coreReference(ref).Fields["account"]
		}
	}
	return entry
}

// DetachedSecrets lists retained identifiers. Optional Windows discovery is
// prefix-filtered by the helper and returns metadata, not credential values.
func DetachedSecrets(provider string) ([]DetachedSecret, error) {
	if provider != "" && provider != "keyring" && provider != secrets.WindowsProvider && provider != "keepassxc" {
		return nil, fmt.Errorf("select keyring, windows-credential-manager or keepassxc")
	}
	disk := State().OnDiskKoanf
	entries := map[string]DetachedSecret{}
	for _, id := range disk.MapKeys("credential_retired") {
		prefix := "credential_retired." + id + "."
		var ref credentialReference
		if err := disk.Unmarshal(prefix+"reference", &ref); err != nil {
			return nil, fmt.Errorf("invalid retained credential reference")
		}
		actualID, err := physicalCredentialID(ref)
		if err != nil || actualID != id {
			return nil, fmt.Errorf("invalid retained credential identity")
		}
		active, err := activeCredential(disk, ref)
		if err != nil {
			return nil, err
		}
		if !active {
			entries[id] = detachedInfo(id, disk.String(prefix+"key"), disk.String(prefix+"reason"), disk.String(prefix+"retained_at"), "tracked", ref)
		}
	}
	if provider == secrets.WindowsProvider {
		client, err := keyringStore(secrets.WindowsProvider)
		if err != nil {
			return nil, err
		}
		lister, ok := client.(native.MetadataLister)
		if !ok {
			return nil, fmt.Errorf("Windows credential store does not support metadata discovery")
		}
		metadata, err := lister.ListMetadata()
		if err != nil {
			return nil, fmt.Errorf("Windows credential metadata scan failed; check Windows Credential Manager access")
		}
		// Recover readable config keys where a current profile matches an older
		// Windows slot, even if the transfer happened before retention tracking.
		known := map[string]struct {
			key string
			ref credentialReference
		}{}
		for _, info := range SecretInventory() {
			credential, err := CredentialForKey(info.Key)
			if err != nil {
				continue
			}
			ref, err := credential.reference()
			if err != nil {
				continue
			}
			ref.Provider = secrets.WindowsProvider
			ref.StorageAccount = ""
			id, _ := physicalCredentialID(ref)
			known[id] = struct {
				key string
				ref credentialReference
			}{info.Key, ref}
		}
		for _, item := range metadata {
			if !secrets.OwnsCredentialService(item.Service) || item.Account == "" || item.Target != item.Service+":"+item.Account {
				continue
			}
			ref := credentialReference{Provider: secrets.WindowsProvider, Service: item.Service, Account: item.Account}
			id, err := physicalCredentialID(ref)
			if err != nil {
				continue
			}
			active, err := activeCredential(disk, ref)
			if err != nil {
				return nil, err
			}
			if active {
				continue
			}
			if _, tracked := entries[id]; tracked {
				continue
			}
			key := ""
			if match, ok := known[id]; ok {
				key, ref = match.key, match.ref
			}
			entries[id] = detachedInfo(id, key, "not referenced by this configuration", "", "windows-scan", ref)
		}
	}
	if provider == "keepassxc" {
		hash, metadata, err := listKeePassXCMetadata(context.Background())
		if err != nil {
			return nil, err
		}
		known := map[string]string{}
		for _, info := range SecretInventory() {
			known[secrets.KeePassXCEntryURL(info.Key)] = info.Key
		}
		for _, item := range metadata {
			parsed, err := url.Parse(item.URL)
			if err != nil || parsed.Scheme != "a-wikimedia-cli" || parsed.Hostname() == "" || parsed.User != nil || parsed.Port() != "" || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
				continue
			}
			ref := credentialReference{Provider: "keepassxc-browser", Browser: &secrets.KeePassXCBrowser{DatabaseHash: hash, EntryID: item.UUID}}
			id, err := physicalCredentialID(ref)
			if err != nil {
				return nil, err
			}
			active, err := activeCredential(disk, ref)
			if err != nil {
				return nil, err
			}
			if active {
				continue
			}
			if _, tracked := entries[id]; tracked {
				continue
			}
			key := known[item.URL]
			if key == "" {
				key = parsed.Hostname()
			}
			entries[id] = detachedInfo(id, key, "not referenced by this configuration", "", "keepassxc-scan", ref)
		}
	}
	result := make([]DetachedSecret, 0, len(entries))
	for _, entry := range entries {
		if provider == "" || entry.Provider == provider {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

var (
	deleteRetainedKeePassXC = secrets.DeleteKeePassXCBrowser
	listKeePassXCMetadata   = secrets.ListKeePassXCEntryMetadata
)

// CleanupDetached deletes one explicitly selected entry after reloading config
// under the same mutation lock used by transfers, saves and profile removal.
func CleanupDetached(ctx context.Context, selected string, provider string) error {
	unlock, err := lockCredentials(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if len(selected) < 12 {
		return fmt.Errorf("specify at least 12 characters of the detached entry ID")
	}
	entries, err := DetachedSecrets("")
	if err != nil {
		return err
	}
	if provider != "" && provider != "keyring" && provider != secrets.WindowsProvider && provider != "keepassxc" {
		return fmt.Errorf("select keyring, windows-credential-manager or keepassxc")
	}
	trackedMatch := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.ID, selected) && (provider == "" || entry.Provider == provider) {
			trackedMatch = true
		}
	}
	if !trackedMatch && provider != "" {
		entries, err = DetachedSecrets(provider)
		if err != nil {
			return err
		}
	}
	var match *DetachedSecret
	for i := range entries {
		if provider != "" && entries[i].Provider != provider {
			continue
		}
		if strings.HasPrefix(entries[i].ID, selected) {
			if match != nil {
				return fmt.Errorf("ambiguous detached ID; use the full ID")
			}
			match = &entries[i]
		}
	}
	if match == nil {
		return fmt.Errorf("detached entry not found; it may be linked again, removed, or require --provider for discovery")
	}
	return removeDetachedEntry(ctx, *match)
}

// Caller holds the credential mutation lock and has persisted the destination.
func removeDetachedEntry(ctx context.Context, entry DetachedSecret) error {
	match := &entry
	if active, err := activeCredential(State().OnDiskKoanf, match.ref); err != nil || active {
		return fmt.Errorf("entry is linked or could not be checked; cleanup refused")
	}
	if match.Provider == "keepassxc" {
		if match.ref.Browser == nil {
			return fmt.Errorf("invalid retained KeePassXC reference")
		}
		if err := deleteRetainedKeePassXC(ctx, *match.ref.Browser); err != nil {
			return err
		}
	} else {
		client, err := keyringStore(match.Provider)
		if err != nil {
			return err
		}
		if err := client.Delete(match.ref.Service, coreReference(match.ref).Fields["account"]); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return fmt.Errorf("credential-store deletion failed; retained record kept")
		}
		if _, err := client.Get(match.ref.Service, coreReference(match.ref).Fields["account"]); !errors.Is(err, secrets.ErrNotFound) {
			return fmt.Errorf("credential-store deletion could not be verified; retained record kept")
		}
	}
	disk := State().OnDiskKoanf.Copy()
	disk.Delete("credential_retired." + match.ID)
	return commitDiskConfig(disk)
}
