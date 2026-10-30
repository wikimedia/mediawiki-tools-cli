package config

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestPlaintextReplacementIsExplicitAndVerified(t *testing.T) {
	store := isolatedConfig(t)
	credential := seedCredential(t)
	if err := credential.SaveWithProvider("old-provider-fixture", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk(credential.Key, "new-plaintext-fixture"); err != nil {
		t.Fatal(err)
	}
	if err := credential.MigrateTo(""); err == nil {
		t.Fatal("ordinary migration replaced a differing provider value")
	}
	if err := credential.ReplaceFromPlaintext(""); err != nil {
		t.Fatal(err)
	}
	if State().OnDiskKoanf.Exists(credential.Key) {
		t.Fatal("verified replacement retained plaintext")
	}
	got, err := credential.Read()
	if err != nil || got != "new-plaintext-fixture" {
		t.Fatal("replacement was not saved")
	}
	if store.reads == 0 {
		t.Fatal("replacement not verified")
	}
}

func TestPlaintextReplacementFailureKeepsSource(t *testing.T) {
	for _, failure := range []string{"write", "verification", "identity"} {
		t.Run(failure, func(t *testing.T) {
			store := isolatedConfig(t)
			credential := seedCredential(t)
			if err := credential.SaveWithProvider("old-provider-fixture", false, nil, "keyring"); err != nil {
				t.Fatal(err)
			}
			if err := PutKeyValueOnDisk(credential.Key, "new-plaintext-fixture"); err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "write":
				store.failSet = true
			case "verification":
				store.failGet = true
			case "identity":
				credential.Account = "another-user"
			}
			if err := credential.ReplaceFromPlaintext(""); err == nil {
				t.Fatal("replacement ignored failure")
			}
			if got := State().OnDiskKoanf.String(credential.Key); got != "new-plaintext-fixture" {
				t.Fatal("failed replacement removed source")
			}
		})
	}
}

func TestPlaintextReplacementRejectsInsecureDestination(t *testing.T) {
	isolatedConfig(t)
	credential := seedCredential(t)
	if err := credential.ReplaceFromPlaintext("plaintext"); err == nil || !strings.Contains(err.Error(), "secure") {
		t.Fatal("replacement accepted plaintext destination")
	}
}

func TestPlaintextReplacementUpdatesTheLinkedKeePassXCEntry(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "verified", true: "write-failed"}[fail], func(t *testing.T) {
			isolatedConfig(t)
			credential := seedCredential(t)
			previousWrite, previousRead := writeKeePassXCBrowser, readKeePassXCBrowser
			t.Cleanup(func() { writeKeePassXCBrowser, readKeePassXCBrowser = previousWrite, previousRead })
			entry := secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: credential.Account}
			stored := "old-provider-fixture"
			writeKeePassXCBrowser = func(_ context.Context, existing *secrets.KeePassXCBrowser, key, account, endpoint, value string) (secrets.KeePassXCBrowser, error) {
				if existing != nil {
					if *existing != entry || key != credential.Key || account != credential.Account || endpoint != credential.Endpoint {
						t.Fatal("replacement changed credential identity or created another entry")
					}
					if fail {
						return secrets.KeePassXCBrowser{}, errors.New("write declined")
					}
				}
				stored = value
				return entry, nil
			}
			readKeePassXCBrowser = func(secrets.KeePassXCBrowser) (string, error) { return stored, nil }
			if err := credential.SaveWithProvider(stored, false, nil, "keepassxc"); err != nil {
				t.Fatal(err)
			}
			if err := PutKeyValueOnDisk(credential.Key, "new-plaintext-fixture"); err != nil {
				t.Fatal(err)
			}
			if err := credential.MigrateTo(""); err == nil {
				t.Fatal("ordinary migration overwrote differing KeePassXC value")
			}
			err := credential.ReplaceFromPlaintext("")
			if fail {
				if err == nil || stored != "old-provider-fixture" || State().OnDiskKoanf.String(credential.Key) != "new-plaintext-fixture" {
					t.Fatal("failed replacement changed a credential")
				}
				return
			}
			if err != nil || stored != "new-plaintext-fixture" || State().OnDiskKoanf.Exists(credential.Key) {
				t.Fatal("replacement did not update the linked entry and remove plaintext")
			}
		})
	}
}
