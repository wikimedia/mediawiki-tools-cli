package config

import (
	"errors"
	"testing"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

func TestMigrationReadsLatestPlaintextUnderLock(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "migrate", true: "replace"}[replace], func(t *testing.T) {
			isolatedConfig(t)
			cred := seedCredential(t)
			stale := State().OnDiskKoanf.Copy()
			if err := PutKeyValueOnDisk(cred.Key, "latest-fixture"); err != nil {
				t.Fatal(err)
			}
			ReApplyKoanfConf(stale) // Another process still has its startup snapshot.
			var err error
			if replace {
				err = cred.ReplaceFromPlaintext("keyring")
			} else {
				err = cred.MigrateTo("keyring")
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, err := cred.Read(); err != nil || got != "latest-fixture" {
				t.Fatal("newer plaintext was lost", err)
			}
			if State().OnDiskKoanf.Exists(cred.Key) {
				t.Fatal("verified plaintext was not removed")
			}
		})
	}
}

func TestUnregisteredKeyringSlotsAreProtected(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{false: "conflict", true: "same value"}[same], func(t *testing.T) {
			store := isolatedConfig(t)
			cred := seedCredential(t)
			if err := cred.SaveWithProvider("existing-fixture", false, nil, "keyring"); err != nil {
				t.Fatal(err)
			}
			ref, err := cred.reference()
			if err != nil {
				t.Fatal(err)
			}
			// A separate configuration has the same public identity but no local link.
			if err := UpdateDiskConfig(func(disk *koanf.Koanf) error { disk.Delete(credentialReferenceKey(cred.Key)); return nil }); err != nil {
				t.Fatal(err)
			}
			value := "different-fixture"
			if same {
				value = "existing-fixture"
			}
			err = cred.SaveWithProvider(value, false, nil, "keyring")
			if same && err != nil {
				t.Fatal(err)
			}
			if !same && !errors.Is(err, secretstore.ErrConflict) {
				t.Fatalf("expected conflict, got %v", err)
			}
			if store.values[ref.Service+ref.Account] != "existing-fixture" {
				t.Fatal("unregistered slot was overwritten")
			}
			if State().OnDiskKoanf.Exists(credentialReferenceKey(cred.Key)) != same {
				t.Fatal("failed save created a reference")
			}
		})
	}
}
