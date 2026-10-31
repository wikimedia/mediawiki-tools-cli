package config

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	native "gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keyring"
)

type metadataStore struct {
	*memorySecrets
	entries []native.CredentialMetadata
}

func (s *metadataStore) ListMetadata() ([]native.CredentialMetadata, error) { return s.entries, nil }

func TestTransfersTrackDetachedEntriesAndProtectRelinkedOnes(t *testing.T) {
	isolatedConfig(t)
	windows := mockWindowsStore(t)
	cred := seedGUITransfer(t)
	if err := cred.TransferTo(secrets.WindowsProvider, TransferOptions{KeepSource: true}); err != nil {
		t.Fatal(err)
	}
	entries, err := DetachedSecrets("")
	if err != nil || len(entries) != 1 || entries[0].Provider != "keepassxc" || entries[0].EntryID == "" {
		t.Fatalf("retained KeePassXC entry missing: %+v, %v", entries, err)
	}
	keepassID := entries[0].ID
	data, _ := os.ReadFile(Path())
	if strings.Contains(string(data), "legacy-secret") {
		t.Fatal("retention record contains secret")
	}
	oldWrite := writeKeePassXCBrowser
	t.Cleanup(func() { writeKeePassXCBrowser = oldWrite })
	writeKeePassXCBrowser = func(context.Context, *secrets.KeePassXCBrowser, string, string, string, string) (secrets.KeePassXCBrowser, error) {
		return secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: "fixture"}, nil
	}
	if err := cred.TransferTo("keepassxc", TransferOptions{KeepSource: true}); err != nil {
		t.Fatal(err)
	}
	entries, err = DetachedSecrets("")
	if err != nil || len(entries) != 1 || entries[0].Provider != secrets.WindowsProvider {
		t.Fatalf("linked vault entry not suppressed: %+v, %v", entries, err)
	}
	if err := CleanupDetached(t.Context(), keepassID[:12], ""); err == nil {
		t.Fatal("deleted a relinked entry")
	}
	if len(windows.values) != 1 {
		t.Fatal("source removed during transfer")
	}
	if err := CleanupDetached(t.Context(), entries[0].ID[:12], ""); err != nil {
		t.Fatal(err)
	}
	if len(windows.values) != 0 {
		t.Fatal("Windows entry not deleted")
	}
	entries, err = DetachedSecrets("")
	if err != nil || len(entries) != 0 {
		t.Fatalf("cleanup record not removed: %+v, %v", entries, err)
	}
	if got, err := cred.Read(); err != nil || got != "legacy-secret" {
		t.Fatal("cleanup damaged active provider")
	}
}

func TestRemovedKeePassXCProfileCanBeExplicitlyCleaned(t *testing.T) {
	isolatedConfig(t)
	cred := seedGUITransfer(t)
	if err := cred.Delete(); err != nil {
		t.Fatal(err)
	}
	entries, err := DetachedSecrets("")
	if err != nil || len(entries) != 1 {
		t.Fatalf("removed entry not retained: %v", err)
	}
	oldDelete := deleteRetainedKeePassXC
	t.Cleanup(func() { deleteRetainedKeePassXC = oldDelete })
	deleteRetainedKeePassXC = func(context.Context, secrets.KeePassXCBrowser) error { return errors.New("cancelled") }
	if err := CleanupDetached(t.Context(), entries[0].ID, ""); err == nil {
		t.Fatal("denied cleanup succeeded")
	}
	still, err := DetachedSecrets("")
	if err != nil || len(still) != 1 {
		t.Fatal("denial discarded recovery metadata")
	}
	deleteRetainedKeePassXC = func(_ context.Context, ref secrets.KeePassXCBrowser) error {
		if ref.EntryID != strings.Repeat("b", 32) {
			t.Fatal("wrong UUID")
		}
		return nil
	}
	if err := CleanupDetached(t.Context(), entries[0].ID, ""); err != nil {
		t.Fatal(err)
	}
	still, err = DetachedSecrets("")
	if err != nil || len(still) != 0 {
		t.Fatal("successful cleanup retained metadata")
	}
}

func TestWindowsScanFindsOlderCopyWithoutReadingValues(t *testing.T) {
	isolatedConfig(t)
	cred := seedGUITransfer(t)
	ref, err := cred.reference()
	if err != nil {
		t.Fatal(err)
	}
	store := &metadataStore{memorySecrets: &memorySecrets{values: map[string]string{}, failGet: true}, entries: []native.CredentialMetadata{
		{Service: ref.Service, Account: ref.Account, Target: ref.Service + ":" + ref.Account},
		{Service: "unrelated", Account: "other", Target: "unrelated:other"},
	}}
	oldWindows := windowsCredentialStore
	t.Cleanup(func() { windowsCredentialStore = oldWindows })
	windowsCredentialStore = func() (secrets.Store, error) { return store, nil }
	before, _ := os.ReadFile(Path())
	entries, err := DetachedSecrets("windows-credential-manager")
	if err != nil || len(entries) != 1 || entries[0].Key != cred.Key || entries[0].Source != "windows-scan" {
		t.Fatalf("older copy not identified: %+v, %v", entries, err)
	}
	if store.reads != 0 {
		t.Fatal("metadata scan read credential values")
	}
	after, _ := os.ReadFile(Path())
	if !bytes.Equal(before, after) {
		t.Fatal("discovery changed configuration")
	}
	if err := CleanupDetached(t.Context(), entries[0].ID[:12], ""); err == nil {
		t.Fatal("scanned entry deleted without opting into discovery")
	}
	store.failGet = false
	if err := CleanupDetached(t.Context(), entries[0].ID[:12], "windows-credential-manager"); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupKeepsRecordWhenNativeDeletionFails(t *testing.T) {
	isolatedConfig(t)
	windows := mockWindowsStore(t)
	cred := seedGUITransfer(t)
	if err := cred.TransferTo(secrets.WindowsProvider, TransferOptions{KeepSource: true}); err != nil {
		t.Fatal(err)
	}
	if err := cred.TransferTo("plaintext", TransferOptions{KeepSource: true}); err != nil {
		t.Fatal(err)
	}
	entries, err := DetachedSecrets("")
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, entry := range entries {
		if entry.Provider == secrets.WindowsProvider {
			id = entry.ID
		}
	}
	if id == "" {
		t.Fatal("missing Windows retained entry")
	}
	windows.failDelete = true
	before, _ := os.ReadFile(Path())
	if err := CleanupDetached(t.Context(), id, ""); err == nil {
		t.Fatal("failed delete reported success")
	}
	after, _ := os.ReadFile(Path())
	if !bytes.Equal(before, after) {
		t.Fatal("failed cleanup lost its reference")
	}
}

func TestCleanupReloadsReferencesBeforeDeleting(t *testing.T) {
	isolatedConfig(t)
	windows := mockWindowsStore(t)
	cred := seedGUITransfer(t)
	if err := cred.TransferTo(secrets.WindowsProvider, TransferOptions{KeepSource: true}); err != nil {
		t.Fatal(err)
	}
	var ref credentialReference
	if err := State().OnDiskKoanf.Unmarshal(credentialReferenceKey(cred.Key), &ref); err != nil {
		t.Fatal(err)
	}
	id, err := physicalCredentialID(ref)
	if err != nil {
		t.Fatal(err)
	}
	disk := State().OnDiskKoanf.Copy()
	if err := retainCredential(disk, cred.Key, "old transfer", ref); err != nil {
		t.Fatal(err)
	}
	disk.Delete(credentialReferenceKey(cred.Key))
	if err := PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	// Simulate another process relinking the entry without refreshing our cache.
	latest := disk.Copy()
	if err := latest.Set(credentialReferenceKey(cred.Key), ref); err != nil {
		t.Fatal(err)
	}
	if err := putDiskConfig(latest); err != nil {
		t.Fatal(err)
	}
	if err := CleanupDetached(t.Context(), id[:12], ""); err == nil {
		t.Fatal("deleted a freshly relinked entry")
	}
	if len(windows.values) != 1 {
		t.Fatal("active Windows credential removed")
	}
}

func TestWindowsCleanupIdentityUsesActualTargetName(t *testing.T) {
	first, err := physicalCredentialID(credentialReference{Provider: secrets.WindowsProvider, Service: "a-wikimedia-cli:service", Account: "profile:slot"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := physicalCredentialID(credentialReference{Provider: secrets.WindowsProvider, Service: "a-wikimedia-cli:service:profile", Account: "slot"})
	if err != nil || first != second {
		t.Fatal("same Windows target could evade active-reference protection")
	}
}
