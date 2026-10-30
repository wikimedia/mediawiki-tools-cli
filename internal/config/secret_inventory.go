package config

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/credentialcheck"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

// SecretInfo describes locally configured credentials, not secret values.
// Linked entries are not proof that an external entry exists or is unlocked.
type SecretInfo struct {
	Key              string `json:"key"`
	Source           string `json:"source"`
	Provider         string `json:"provider"`
	Service          string `json:"service,omitempty"`
	Account          string `json:"account,omitempty"`
	Endpoint         string `json:"endpoint,omitempty"`
	Location         string `json:"location"`
	Status           string `json:"status"`
	Reference        string `json:"reference,omitempty"`
	DatabaseID       string `json:"database_id,omitempty"`
	EntryID          string `json:"entry_id,omitempty"`
	PlaintextPresent bool   `json:"plaintext_present"`
	Comparison       string `json:"comparison,omitempty"`
	Validation       string `json:"validation,omitempty"`
	AuthenticatedAs  string `json:"authenticated_as,omitempty"`
}

// SecretInventory never reads a credential provider or the pairing file's data.
// Enumerate known keys as well as plaintext keys: secure saves remove the latter.
func SecretInventory() []SecretInfo {
	disk := State().OnDiskKoanf
	candidates := map[string]bool{"gerrit.password": true, "phabricator.key": true}
	for _, project := range disk.MapKeys("cloud_vps.projects") {
		candidates["cloud_vps.projects."+project+".credential.secret"] = true
	}
	for _, site := range disk.MapKeys("phabricator.sites") {
		candidates["phabricator.sites."+site+".key"] = true
	}
	for _, realm := range disk.MapKeys("wiki.accounts") {
		for _, account := range disk.MapKeys("wiki.accounts." + realm) {
			prefix := "wiki.accounts." + realm + "." + account + "."
			candidates[prefix+"password"], candidates[prefix+"token"] = true, true
		}
	}
	for _, key := range disk.Keys() {
		if strings.HasPrefix(key, "credential_storage.") || !SensitiveKey(key) {
			continue
		}
		value := disk.Get(key)
		if value == nil {
			continue
		}
		if s, ok := value.(string); ok && s == "" {
			continue
		}
		candidates[key] = true
	}
	entries := make([]SecretInfo, 0)
	seen := map[string]bool{}
	for key := range candidates {
		reference := credentialReferenceKey(key)
		linked := disk.Exists(reference)
		value := disk.Get(key)
		plaintext := value != nil
		if s, ok := value.(string); ok && s == "" {
			plaintext = false
		}
		if !linked && !plaintext {
			continue
		}
		info := SecretInfo{Key: key, Source: "plaintext-config", Provider: "plaintext", Location: Path(), Status: "plaintext", PlaintextPresent: plaintext}
		credential, credentialErr := CredentialForKey(key)
		if credentialErr == nil {
			info.Service, info.Account, info.Endpoint = credential.Service, credential.Account, credential.Endpoint
		}
		if linked {
			plaintextInfo := info
			seen[reference] = true
			info = describeCredentialReference(reference, info)
			if info.Status != "invalid reference" && credentialErr != nil {
				info.Status = "linked; configuration incomplete or unsupported"
			} else if info.Status != "invalid reference" {
				if expected, err := credential.reference(); err != nil {
					info.Status = "linked; invalid account or endpoint"
				} else {
					var ref credentialReference
					if disk.Unmarshal(reference, &ref) == nil && (ref.Service != expected.Service || ref.Endpoint != expected.Endpoint || ref.Account != expected.Account || ref.Identity != expected.Identity) {
						info.Status = "linked; account or endpoint mismatch"
					}
				}
			}
			if plaintext && info.Provider != "plaintext" {
				info.Comparison = "not-checked"
				info.Status += "; plaintext copy also present"
				plaintextInfo.Comparison = "not-checked"
				plaintextInfo.Status = "additional plaintext value (not compared with provider)"
				entries = append(entries, plaintextInfo)
			}
		} else if credentialErr != nil {
			info.Status = "plaintext; no credential provider mapping"
		}
		entries = append(entries, info)
	}
	for _, id := range disk.MapKeys("credential_storage") {
		reference := "credential_storage." + id
		if seen[reference] {
			continue
		}
		info := describeCredentialReference(reference, SecretInfo{Key: reference})
		if info.Status == "linked (not checked)" {
			info.Status = "unmapped reference (not checked)"
		}
		entries = append(entries, info)
	}
	if file, err := os.Lstat(secrets.KeePassXCPairingPath()); err == nil {
		status := "present (not checked)"
		if !file.Mode().IsRegular() {
			status = "not a regular file"
		}
		entries = append(entries, SecretInfo{Key: "keepassxc-pairing", Source: "pairing-file", Provider: "pairing-file", Service: "keepassxc", Location: secrets.KeePassXCPairingPath(), Status: status, PlaintextPresent: true})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Key == entries[j].Key {
			return entries[i].Source < entries[j].Source
		}
		return entries[i].Key < entries[j].Key
	})
	return entries
}

// CheckSecretCopies compares only entries having both a plaintext value and a
// linked secure provider. It never changes configuration or returns values,
// hashes, lengths, or provider error messages. Provider access may prompt.
func CheckSecretCopies(entries []SecretInfo) {
	InspectSecretInventory(context.Background(), entries, true, false)
}

var checkServiceCredential = credentialcheck.Check

// InspectSecretInventory checks each requested copy independently. Credentials
// stay in memory for this invocation; shared reads avoid repeated vault prompts.
func InspectSecretInventory(ctx context.Context, entries []SecretInfo, compare, verify bool) {
	type readResult struct {
		value string
		err   error
	}
	cache := map[string]readResult{}
	credentialForEntry := func(entry SecretInfo) (Credential, error) {
		credential, err := CredentialForKey(entry.Key)
		if err != nil {
			return Credential{}, err
		}
		if entry.Source == "credential-reference" && entry.Provider != "plaintext" {
			// Inventory inspects the saved identity, even when current config
			// has moved to another account. Normal command reads still enforce
			// the current config's account and endpoint binding.
			credential.Account, credential.Endpoint = entry.Account, entry.Endpoint
		}
		return credential, nil
	}
	read := func(entry SecretInfo) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		id := entry.Key + "\x00" + entry.Source
		if previous, ok := cache[id]; ok {
			return previous.value, previous.err
		}
		var result readResult
		credential, err := credentialForEntry(entry)
		if err != nil {
			result.err = err
		} else if err := credential.Validate(); err != nil {
			result.err = err
		} else if entry.Source == "plaintext-config" || entry.Provider == "plaintext" {
			result.value = State().OnDiskKoanf.String(entry.Key)
			if result.value == "" {
				result.err = fmt.Errorf("plaintext value unavailable")
			}
		} else {
			result.value, result.err = credential.Read()
		}
		cache[id] = result
		return result.value, result.err
	}
	results := map[string]string{}
	for _, entry := range entries {
		if !compare || entry.Source != "credential-reference" || entry.Comparison != "not-checked" {
			continue
		}
		result := "unavailable"
		if saved, err := read(entry); err == nil && saved != "" {
			plaintext := State().OnDiskKoanf.String(entry.Key)
			if subtle.ConstantTimeCompare([]byte(saved), []byte(plaintext)) == 1 {
				result = "match"
			} else {
				result = "different"
			}
		}
		results[entry.Key] = result
	}
	for i := range entries {
		if result, ok := results[entries[i].Key]; ok {
			entries[i].Comparison = result
			if entries[i].Source == "plaintext-config" {
				entries[i].Status = "additional plaintext value"
			}
			if result == "match" || result == "different" {
				entries[i].Status = strings.Replace(entries[i].Status, "linked (not checked)", "linked; value read", 1)
			}
		}
		if !verify {
			continue
		}
		entry := &entries[i]
		if entry.Source == "pairing-file" {
			entry.Validation = "not-applicable"
			continue
		}
		credential, err := credentialForEntry(*entry)
		if err != nil {
			entry.Validation = "unsupported"
			continue
		}
		input := credentialcheck.Request{Service: credential.Service, Endpoint: credential.Endpoint, Account: credential.Account}
		if credential.Service == "wiki" {
			parts := strings.Split(entry.Key, ".")
			if len(parts) != 5 {
				entry.Validation = "unsupported"
				continue
			}
			account, err := LoadWikiAccount(parts[2], parts[3])
			if err != nil || !WikiRealmAllows(account.Realm, account.Wiki) {
				entry.Validation = "unavailable"
				continue
			}
			input.WikiMethod, input.WikiEndpoint = account.Method, account.Wiki
		}
		input.Value, err = read(*entry)
		if err != nil || input.Value == "" {
			entry.Validation = "unavailable"
			continue
		}
		entry.Status = strings.Replace(entry.Status, "linked (not checked)", "linked; value read", 1)
		result := checkServiceCredential(ctx, input)
		entry.Validation, entry.AuthenticatedAs = result.Status, result.Identity
		if result.Status == "wrong-account" && result.Identity != "" {
			entry.Status += fmt.Sprintf("; authenticated as %s; expected %s", result.Identity, input.Account)
		}
	}
}

func describeCredentialReference(reference string, info SecretInfo) SecretInfo {
	info.Source = "credential-reference"
	info.Reference = reference
	info.Status = "linked (not checked)"
	var ref credentialReference
	if State().OnDiskKoanf.Unmarshal(reference, &ref) != nil {
		info.Provider = "unknown"
		info.Location = Path()
		info.Status = "invalid reference"
		return info
	}
	info.Account, info.Endpoint = ref.Identity, ref.Endpoint
	info.Service = strings.TrimPrefix(ref.Service, "mwcli:")
	if service, _, ok := strings.Cut(info.Service, ":https:"); ok {
		info.Service = service
	} else if service, _, ok := strings.Cut(info.Service, ":http:"); ok {
		info.Service = service
	}
	switch ref.Provider {
	case "", "keyring":
		info.Provider = "keyring"
		info.Location = "OS keyring"
		switch runtime.GOOS {
		case "linux":
			info.Location = "Linux Secret Service"
		case "darwin":
			info.Location = "macOS Keychain"
		case "windows":
			info.Location = "Windows Credential Manager"
		}
	case "plaintext":
		info.Provider = "plaintext"
		info.Location = Path()
		info.Status = "plaintext"
		if !info.PlaintextPresent {
			info.Status = "plaintext value missing"
		}
	case "keepassxc-browser":
		info.Provider = "keepassxc"
		info.Location = "KeePassXC database"
		if ref.Browser == nil {
			info.Status = "invalid KeePassXC reference"
		} else {
			info.DatabaseID, info.EntryID = ref.Browser.DatabaseHash, ref.Browser.EntryID
		}
	case "keepassxc":
		info.Provider = "keepassxc"
		info.Location = "KeePassXC database"
		info.Status = "legacy link; needs relinking"
	default:
		info.Provider = "unknown"
		info.Location = Path()
		info.Status = "unknown credential provider"
	}
	return info
}
