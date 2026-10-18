package config

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func seedGUITransfer(t *testing.T) Credential {
	t.Helper()
	cred := seedCredential(t)
	oldFind, oldRead := findKeePassXCBrowser, readKeePassXCBrowser
	t.Cleanup(func() { findKeePassXCBrowser, readKeePassXCBrowser = oldFind, oldRead })
	findKeePassXCBrowser = func(context.Context, string) (secrets.KeePassXCBrowser, string, error) {
		return secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: "fixture"}, "legacy-secret", nil
	}
	readKeePassXCBrowser = func(secrets.KeePassXCBrowser) (string, error) { return "legacy-secret", nil }
	if err := cred.UseKeePassXC(context.Background(), "wm/Gerrit"); err != nil {
		t.Fatal(err)
	}
	return cred
}

type unverifiableTransferStore struct {
	*memorySecrets
	written bool
}

func (s *unverifiableTransferStore) Set(service, account, value string) error {
	s.written = true
	return s.memorySecrets.Set(service, account, value)
}

func (s *unverifiableTransferStore) Get(service, account string) (string, error) {
	if s.written {
		return "different", nil
	}
	return s.memorySecrets.Get(service, account)
}

func TestTransferKeePassXCToKeyring(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedGUITransfer(t)
	if err := PutKeyValueOnDisk("secrets.provider", "keepassxc"); err != nil {
		t.Fatal(err)
	}
	if err := cred.TransferTo("keyring"); err != nil {
		t.Fatal(err)
	}
	if got, err := cred.Read(); err != nil || got != "legacy-secret" {
		t.Fatal("cannot read transferred credential")
	}
	if got, err := cred.CredentialProvider(""); err != nil || got != "keyring" {
		t.Fatal("reference did not change")
	}
	if got, err := DefaultCredentialProvider(); err != nil || got != "keepassxc" {
		t.Fatal("transfer changed the default")
	}
	if len(store.values) != 1 {
		t.Fatal("destination not saved")
	}
	var ref credentialReference
	if err := State().OnDiskKoanf.Unmarshal(credentialReferenceKey(cred.Key), &ref); err != nil {
		t.Fatal(err)
	}
	if ref.Browser != nil || ref.KeePassXC != nil {
		t.Fatal("old provider metadata remained")
	}
	data, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "legacy-secret") {
		t.Fatal("transfer wrote plaintext")
	}
}

func TestTransferFailureKeepsSourceReference(t *testing.T) {
	for _, mode := range []string{"unavailable", "write-failed", "unverified", "conflict", "denied", "plaintext-conflict"} {
		t.Run(mode, func(t *testing.T) {
			store := isolatedConfig(t)
			cred := seedGUITransfer(t)
			ref, err := cred.reference()
			if err != nil {
				t.Fatal(err)
			}
			sourceReads := 0
			readKeePassXCBrowser = func(secrets.KeePassXCBrowser) (string, error) {
				sourceReads++
				if mode == "denied" {
					return "", errors.New("denied")
				}
				return "legacy-secret", nil
			}
			switch mode {
			case "unavailable":
				store.failGet = true
			case "write-failed":
				store.failSet = true
			case "unverified":
				credentialStore = &unverifiableTransferStore{memorySecrets: store}
			case "conflict":
				store.values[ref.Service+ref.Account] = "different"
			case "plaintext-conflict":
				if err := PutKeyValueOnDisk(cred.Key, "stale"); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			if err := cred.TransferTo("keyring"); err == nil {
				t.Fatal("expected failure")
			}
			after, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("failed transfer changed source reference")
			}
			if mode == "unavailable" && sourceReads != 0 {
				t.Fatal("opened source GUI despite unavailable destination")
			}
			if mode == "conflict" && store.values[ref.Service+ref.Account] != "different" {
				t.Fatal("overwrote destination")
			}
		})
	}
}

func TestTransferKeyringToKeePassXCRetainsOriginal(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedCredential(t)
	if err := cred.Save("legacy-secret", false); err != nil {
		t.Fatal(err)
	}
	old := writeKeePassXCBrowser
	t.Cleanup(func() { writeKeePassXCBrowser = old })
	writeKeePassXCBrowser = func(_ context.Context, existing *secrets.KeePassXCBrowser, key, account, endpoint, value string) (secrets.KeePassXCBrowser, error) {
		if existing != nil || key != cred.Key || account != cred.Account || value != "legacy-secret" {
			t.Fatal("incorrect transfer destination")
		}
		return secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: account}, nil
	}
	if err := cred.TransferTo("keepassxc"); err != nil {
		t.Fatal(err)
	}
	if len(store.values) != 1 {
		t.Fatal("transfer deleted source")
	}
	if got, err := cred.CredentialProvider(""); err != nil || got != "keepassxc" {
		t.Fatal("destination not selected")
	}
}
