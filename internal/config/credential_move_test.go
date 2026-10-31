package config

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

type observedDeleteStore struct {
	*memorySecrets
	onDelete func()
}

func (s *observedDeleteStore) Delete(service, account string) error {
	s.onDelete()
	return s.memorySecrets.Delete(service, account)
}

func TestWindowsMoveDeletesSourceOnlyAfterDestinationCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native keyring and Windows provider address the same underlying store")
	}
	native := isolatedConfig(t)
	windows := mockWindowsStore(t)
	cred := seedGUITransfer(t)
	if err := cred.TransferTo(secrets.WindowsProvider, TransferOptions{KeepSource: true}); err != nil {
		t.Fatal(err)
	}
	deleted := false
	windowsCredentialStore = func() (secrets.Store, error) {
		return &observedDeleteStore{memorySecrets: windows, onDelete: func() {
			deleted = true
			if provider, err := cred.CredentialProvider(""); err != nil || provider != "keyring" {
				t.Fatal("source deleted before destination reference was committed")
			}
			if len(native.values) != 1 {
				t.Fatal("source deleted before destination was saved")
			}
		}}, nil
	}
	if err := cred.TransferTo("keyring"); err != nil {
		t.Fatal(err)
	}
	if !deleted || len(windows.values) != 0 {
		t.Fatal("move left the Windows source behind")
	}
	if value, err := cred.Read(); err != nil || value != "legacy-secret" {
		t.Fatal("destination not usable")
	}
}

func TestKeePassXCMoveRetainsSourceRecordOnDeniedCleanup(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "approved", true: "denied"}[denied], func(t *testing.T) {
			isolatedConfig(t)
			mockWindowsStore(t)
			cred := seedGUITransfer(t)
			oldDelete := deleteRetainedKeePassXC
			t.Cleanup(func() { deleteRetainedKeePassXC = oldDelete })
			calls := 0
			deleteRetainedKeePassXC = func(context.Context, secrets.KeePassXCBrowser) error {
				calls++
				if provider, err := cred.CredentialProvider(""); err != nil || provider != secrets.WindowsProvider {
					t.Fatal("cleanup happened before destination commit")
				}
				if denied {
					return errors.New("GUI approval denied")
				}
				return nil
			}
			err := cred.TransferTo(secrets.WindowsProvider)
			if (err != nil) != denied || calls != 1 {
				t.Fatalf("cleanup: %v", err)
			}
			if denied && !strings.Contains(err.Error(), "transfer to windows-credential-manager completed") {
				t.Fatal("partial success not explained")
			}
			if value, err := cred.Read(); err != nil || value != "legacy-secret" {
				t.Fatal("cleanup failure rolled back destination")
			}
			entries, err := DetachedSecrets("")
			if err != nil || (len(entries) == 1) != denied {
				t.Fatal("wrong retained-source state")
			}
		})
	}
}

func TestMoveKeepsSourceUsedByAnotherReference(t *testing.T) {
	isolatedConfig(t)
	windows := mockWindowsStore(t)
	cred := seedGUITransfer(t)
	if err := cred.TransferTo(secrets.WindowsProvider, TransferOptions{KeepSource: true}); err != nil {
		t.Fatal(err)
	}
	disk := State().OnDiskKoanf.Copy()
	var source credentialReference
	if err := disk.Unmarshal(credentialReferenceKey(cred.Key), &source); err != nil {
		t.Fatal(err)
	}
	if err := disk.Set("credential_storage.another", source); err != nil {
		t.Fatal(err)
	}
	if err := PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	if err := cred.TransferTo("keyring"); err != nil {
		t.Fatal(err)
	}
	if len(windows.values) != 1 {
		t.Fatal("deleted another reference's source")
	}
}
