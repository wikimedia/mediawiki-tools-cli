package config

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func interactiveStorageFixture(t *testing.T, provider string) *int {
	t.Helper()
	oldInteractive, oldChoose := credentialStorageInteractive, chooseCredentialProvider
	t.Cleanup(func() { credentialStorageInteractive, chooseCredentialProvider = oldInteractive, oldChoose })
	calls := new(int)
	credentialStorageInteractive = func() bool { return true }
	chooseCredentialProvider = func() (string, error) { *calls++; return provider, nil }
	return calls
}

func TestFirstCredentialCanChoosePlaintextWithoutKeyring(t *testing.T) {
	store := isolatedConfig(t)
	store.failSet = true
	cred := seedCredential(t)
	calls := interactiveStorageFixture(t, "plaintext")
	logs := &bytes.Buffer{}
	oldOut := logrus.StandardLogger().Out
	logrus.SetOutput(logs)
	t.Cleanup(func() { logrus.SetOutput(oldOut) })
	if err := cred.Save("new-plaintext-secret", false); err != nil {
		t.Fatal(err)
	}
	if *calls != 1 || store.reads != 0 {
		t.Fatal("plaintext selection prompted repeatedly or opened keyring")
	}
	if provider, err := DefaultCredentialProvider(); err != nil || provider != "plaintext" {
		t.Fatal("selection was not saved as default")
	}
	if value, err := cred.Read(); err != nil || value != "new-plaintext-secret" {
		t.Fatal("plaintext credential not usable")
	}
	if !strings.Contains(logs.String(), "unencrypted") || strings.Contains(logs.String(), "new-plaintext-secret") {
		t.Fatal("missing warning or credential in warning")
	}
	if err := PutKeyValueOnDisk("secrets.provider", "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := cred.Save("updated-secret", false); err != nil {
		t.Fatal(err)
	}
	if *calls != 1 {
		t.Fatal("existing plaintext binding asked again")
	}
	if value, err := cred.Read(); err != nil || value != "updated-secret" {
		t.Fatal("default change moved existing credential")
	}
	if err := cred.Delete(); err != nil {
		t.Fatal(err)
	}
	if HasCredential(cred.Key) {
		t.Fatal("plaintext logout left saved credential")
	}
}

func TestFirstCredentialSelectionOnlyPersistsAfterSuccess(t *testing.T) {
	for _, mode := range []string{"cancel", "write-failed", "keyring-success"} {
		t.Run(mode, func(t *testing.T) {
			store := isolatedConfig(t)
			cred := seedCredential(t)
			interactiveStorageFixture(t, "keyring")
			if mode == "cancel" {
				chooseCredentialProvider = func() (string, error) { return "", errors.New("cancelled") }
			}
			store.failSet = mode == "write-failed"
			before, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			err = cred.Save("replacement", false)
			if mode == "keyring-success" {
				if err != nil {
					t.Fatal(err)
				}
				if provider, err := DefaultCredentialProvider(); err != nil || provider != "keyring" {
					t.Fatal("successful selection not remembered")
				}
			} else {
				if err == nil {
					t.Fatal("expected failure")
				}
				after, err := os.ReadFile(Path())
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatal("failed selection changed configuration")
				}
			}
		})
	}
}

func TestExistingProviderFailureNeverChoosesPlaintext(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedCredential(t)
	if err := cred.Save("vault-secret", false); err != nil {
		t.Fatal(err)
	}
	calls := interactiveStorageFixture(t, "plaintext")
	store.failSet = true
	if err := cred.Save("replacement", false); err == nil {
		t.Fatal("expected failure")
	}
	if *calls != 0 || State().OnDiskKoanf.Exists(cred.Key) {
		t.Fatal("existing provider failure triggered plaintext storage")
	}
}

func TestNoninteractiveUnsetProviderNeverFallsBackToPlaintext(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedCredential(t)
	calls := interactiveStorageFixture(t, "plaintext")
	credentialStorageInteractive = func() bool { return false }
	store.failSet = true
	if err := cred.Save("replacement", false); err == nil {
		t.Fatal("expected keyring failure")
	}
	if *calls != 0 || State().OnDiskKoanf.String(cred.Key) != "legacy-secret" || State().OnDiskKoanf.String("secrets.provider") != "" {
		t.Fatal("noninteractive failure selected or wrote plaintext")
	}
}

func TestExplicitPlaintextOverrideSkipsFirstUsePrompt(t *testing.T) {
	isolatedConfig(t)
	cred := seedCredential(t)
	calls := interactiveStorageFixture(t, "keyring")
	if err := cred.SaveWithProvider("plain-secret", false, nil, "plaintext"); err != nil {
		t.Fatal(err)
	}
	if *calls != 0 || State().OnDiskKoanf.String("secrets.provider") != "" {
		t.Fatal("override prompted or changed default")
	}
	if err := cred.MigrateTo(""); err != nil {
		t.Fatal(err)
	}
	if value, err := cred.Read(); err != nil || value != "plain-secret" {
		t.Fatal("migration deleted plaintext credential")
	}
}

func TestStorageChooserDiscoveryDoesNotOpenVaults(t *testing.T) {
	for _, mode := range []string{"none", "keyring", "paired", "unpaired", "pairing-denied"} {
		t.Run(mode, func(t *testing.T) {
			oldKeyring, oldKeepass, oldAsk, oldPair := detectKeyring, detectKeePassXC, askCredentialStorage, pairCredentialGUI
			t.Cleanup(func() {
				detectKeyring, detectKeePassXC, askCredentialStorage, pairCredentialGUI = oldKeyring, oldKeepass, oldAsk, oldPair
			})
			detectKeyring = func(ctx context.Context) secrets.Availability {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("discovery has no deadline")
				}
				return secrets.Availability{Available: mode == "keyring", Detail: "fixture keyring status"}
			}
			detectKeePassXC = func() secrets.Availability {
				return secrets.Availability{Available: mode != "none" && mode != "keyring", NeedsPairing: mode == "unpaired" || mode == "pairing-denied", Detail: "fixture KeePassXC status"}
			}
			askCredentialStorage = func(labels []string) (int, error) {
				if !strings.Contains(labels[len(labels)-1], "unencrypted") {
					t.Fatal("plaintext choice not explicit")
				}
				if mode == "none" && len(labels) != 1 {
					t.Fatal("unavailable providers offered as ready")
				}
				return 0, nil
			}
			pairs := 0
			pairCredentialGUI = func() error {
				pairs++
				if mode == "pairing-denied" {
					return errors.New("denied")
				}
				return nil
			}
			got, err := promptCredentialProvider()
			want := "keepassxc"
			if mode == "none" {
				want = "plaintext"
			}
			if mode == "keyring" {
				want = "keyring"
			}
			if mode == "pairing-denied" {
				if err == nil || got != "" {
					t.Fatal("denied pairing selected a provider")
				}
			} else if err != nil || got != want {
				t.Fatalf("selection: %s %v", got, err)
			}
			expectedPairs := 0
			if mode == "unpaired" || mode == "pairing-denied" {
				expectedPairs = 1
			}
			if pairs != expectedPairs {
				t.Fatal("discovery unexpectedly opened a GUI")
			}
		})
	}
}

func TestPlaintextMigrationCanUpgradeItsBinding(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "override"}[explicit], func(t *testing.T) {
			store := isolatedConfig(t)
			cred := seedCredential(t)
			if err := cred.SaveWithProvider("plain-secret", false, nil, "plaintext"); err != nil {
				t.Fatal(err)
			}
			provider := ""
			if explicit {
				provider = "keyring"
			} else if err := PutKeyValueOnDisk("secrets.provider", "keyring"); err != nil {
				t.Fatal(err)
			}
			if err := cred.MigrateTo(provider); err != nil {
				t.Fatal(err)
			}
			if State().OnDiskKoanf.Exists(cred.Key) || len(store.values) != 1 {
				t.Fatal("migration left plaintext or did not populate keyring")
			}
			if got, err := cred.Read(); err != nil || got != "plain-secret" {
				t.Fatal("upgraded credential is not readable")
			}
		})
	}
}
