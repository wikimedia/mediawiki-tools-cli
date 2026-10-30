package keepassxc

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"unicode/utf8"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

type fakeBrowser struct {
	value string
	reads int
}

func (b *fakeBrowser) Read(context.Context, Reference) (string, error) {
	b.reads++
	return b.value, nil
}
func (b *fakeBrowser) Write(_ context.Context, existing *Reference, key, account, endpoint, value string) (Reference, error) {
	if !utf8.ValidString(value) {
		return Reference{}, errors.New("invalid protocol string")
	}
	b.value = value
	return Reference{DatabaseHash: testBrowserHash, EntryID: testBrowserUUID, Login: account}, nil
}

func TestBackendSupportsBytesAndDoesNotPretendToDelete(t *testing.T) {
	browser := &fakeBrowser{}
	b := New(browser)
	store, err := secretstore.Open(secretstore.Config{Namespace: "app", Provider: b.Name(), Registry: secretstore.NewMemoryRegistry(), Backends: []secretstore.Backend{b}})
	if err != nil {
		t.Fatal(err)
	}
	value := []byte{0, 255, 128, 10}
	if err := store.Set(t.Context(), "key", value); err != nil {
		t.Fatal(err)
	}
	if browser.reads != 0 {
		t.Fatal("verified provider unnecessarily prompted for another read")
	}
	if got, err := store.Get(t.Context(), "key"); err != nil || !bytes.Equal(got, value) {
		t.Fatal("binary value not preserved")
	}
	if err := store.Delete(t.Context(), "key"); !errors.Is(err, secretstore.ErrUnsupported) {
		t.Fatal("unsupported delete claimed success")
	}
	if err := store.Unlink(t.Context(), "key"); err != nil {
		t.Fatal(err)
	}
	if browser.value == "" {
		t.Fatal("unlink changed external entry")
	}
}

func TestClientUsesApplicationSelectedGroup(t *testing.T) {
	pairing := browserFixture(t, "create")
	testClient.Group = "another-app"
	t.Setenv("SECRETSTORE_TEST_GROUP", "another-app")
	if err := testClient.saveBrowserPairing(pairing); err != nil {
		t.Fatal(err)
	}
	if _, err := testClient.Write(t.Context(), nil, "gerrit.password", "fixture", "https://gerrit.wikimedia.org", "updated-fixture-secret"); err != nil {
		t.Fatal(err)
	}
}
