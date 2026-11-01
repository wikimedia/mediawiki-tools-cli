package config

import (
	"context"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keepassxc"
)

func TestKeePassXCScanFiltersOwnershipAndProtectsActiveUUIDs(t *testing.T) {
	isolatedConfig(t)
	cred := seedGUITransfer(t)
	mockWindowsStore(t)
	oldList, oldDelete := listKeePassXCMetadata, deleteRetainedKeePassXC
	t.Cleanup(func() { listKeePassXCMetadata, deleteRetainedKeePassXC = oldList, oldDelete })
	listKeePassXCMetadata = func(context.Context) (string, []keepassxc.EntryMetadata, error) {
		return strings.Repeat("a", 64), []keepassxc.EntryMetadata{
			{UUID: strings.Repeat("b", 32), Title: "active", URL: secrets.KeePassXCEntryURL(cred.Key)},
			{UUID: strings.Repeat("c", 32), Title: "detached", URL: "a-wikimedia-cli://wiki.profiles.wikimedia.old.token"},
			{UUID: strings.Repeat("d", 32), Title: "unrelated", URL: "https://example.org"},
		}, nil
	}
	readKeePassXCBrowser = func(_ context.Context, _ secrets.KeePassXCBrowser) (string, error) {
		t.Fatal("scan retrieved credential value")
		return "", nil
	}
	entries, err := DetachedSecrets("keepassxc")
	if err != nil || len(entries) != 1 || entries[0].EntryID != strings.Repeat("c", 32) {
		t.Fatalf("scan did not filter active/unrelated entries: %+v, %v", entries, err)
	}
	deleted := false
	deleteRetainedKeePassXC = func(_ context.Context, ref secrets.KeePassXCBrowser) error {
		if ref.EntryID != strings.Repeat("c", 32) {
			t.Fatal("wrong UUID selected")
		}
		deleted = true
		return nil
	}
	if err := CleanupDetached(t.Context(), entries[0].ID[:12], "windows-credential-manager"); err == nil {
		t.Fatal("accepted mismatching provider")
	}
	if deleted {
		t.Fatal("provider mismatch deleted an entry")
	}
	if err := CleanupDetached(t.Context(), entries[0].ID[:12], "keepassxc"); err != nil || !deleted {
		t.Fatalf("explicit scanned-entry cleanup: %v", err)
	}
}
