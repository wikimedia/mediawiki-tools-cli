package config

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func mockWindowsStore(t *testing.T) *memorySecrets {
	t.Helper()
	store := &memorySecrets{values: map[string]string{}}
	old := windowsCredentialStore
	windowsCredentialStore = func() (secrets.Store, error) { return store, nil }
	t.Cleanup(func() { windowsCredentialStore = old })
	return store
}

func TestCredentialTransfersAcrossAllNativeStores(t *testing.T) {
	nativeStore := isolatedConfig(t)
	windows := mockWindowsStore(t)
	cred := seedGUITransfer(t)
	if err := PutKeyValueOnDisk("secrets.provider", "keepassxc"); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{secrets.WindowsProvider, "keyring", secrets.WindowsProvider} {
		if err := cred.TransferTo(provider, TransferOptions{KeepSource: true}); err != nil {
			t.Fatalf("transfer to %s: %v", provider, err)
		}
		if got, err := cred.Read(); err != nil || got != "legacy-secret" {
			t.Fatalf("read %s: %v", provider, err)
		}
		if got, err := cred.CredentialProvider(""); err != nil || got != provider {
			t.Fatal("provider binding lost")
		}
		if got, err := DefaultCredentialProvider(); err != nil || got != "keepassxc" {
			t.Fatal("default changed")
		}
		entries := SecretInventory()
		found := false
		for _, entry := range entries {
			if entry.Key == cred.Key && entry.Provider == provider {
				found = true
			}
		}
		if !found {
			t.Fatal("inventory did not expose the active provider")
		}
	}
	if len(nativeStore.values) != 1 || len(windows.values) != 1 {
		t.Fatal("source entries were removed")
	}
	if err := cred.Delete(); err != nil || len(windows.values) != 0 || len(nativeStore.values) != 1 {
		t.Fatalf("remove addressed wrong provider: %v", err)
	}
}

func TestWindowsSaveAndMigration(t *testing.T) {
	isolatedConfig(t)
	store := mockWindowsStore(t)
	cred := seedCredential(t)
	if err := cred.MigrateTo(secrets.WindowsProvider); err != nil {
		t.Fatal(err)
	}
	if err := cred.SaveWithProvider("updated-fixture", false, map[string]string{"gerrit.username": "fixture"}, secrets.WindowsProvider); err != nil {
		t.Fatal(err)
	}
	if got, err := cred.Read(); err != nil || got != "updated-fixture" {
		t.Fatalf("update: %v", err)
	}
	data, err := os.ReadFile(Path())
	if err != nil || strings.Contains(string(data), "updated-fixture") || len(store.values) != 1 {
		t.Fatal("secret leaked into config")
	}
}

func TestWindowsTransferFailureRetainsSource(t *testing.T) {
	for _, mode := range []string{"unavailable", "write-failed", "unverified", "conflict", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			isolatedConfig(t)
			store := mockWindowsStore(t)
			cred := seedGUITransfer(t)
			ref, err := cred.reference()
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "unavailable":
				windowsCredentialStore = func() (secrets.Store, error) { return nil, errors.New("not configured") }
			case "write-failed":
				store.failSet = true
			case "unverified":
				windowsCredentialStore = func() (secrets.Store, error) { return &unverifiableTransferStore{memorySecrets: store}, nil }
			case "conflict":
				store.values[ref.Service+ref.Account] = "other"
			case "oversized":
				readKeePassXCBrowser = func(secrets.KeePassXCBrowser) (string, error) { return strings.Repeat("x", 2561), nil }
			}
			before, _ := os.ReadFile(Path())
			if err := cred.TransferTo(secrets.WindowsProvider, TransferOptions{KeepSource: true}); err == nil {
				t.Fatal("accepted failed transfer")
			}
			after, _ := os.ReadFile(Path())
			if !bytes.Equal(before, after) {
				t.Fatal("failed transfer changed source reference")
			}
			if mode == "conflict" && store.values[ref.Service+ref.Account] != "other" {
				t.Fatal("overwrote a conflicting destination")
			}
		})
	}
}
