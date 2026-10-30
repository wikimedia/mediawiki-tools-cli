package keyring

import (
	"bytes"
	"errors"
	"testing"

	oskeyring "github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

type fakeClient struct{ values map[string]string }

func (c fakeClient) Get(service, account string) (string, error) {
	value, ok := c.values[service+"\x00"+account]
	if !ok {
		return "", oskeyring.ErrNotFound
	}
	return value, nil
}
func (c fakeClient) Set(service, account, value string) error {
	c.values[service+"\x00"+account] = value
	return nil
}
func (c fakeClient) Delete(service, account string) error {
	delete(c.values, service+"\x00"+account)
	return nil
}

func TestBackendBinaryRoundTripAndCollisionProtection(t *testing.T) {
	client := fakeClient{values: map[string]string{}}
	b := &Backend{Client: client}
	r := secretstore.NewMemoryRegistry()
	store, err := secretstore.Open(secretstore.Config{Namespace: "app", Provider: b.Name(), Registry: r, Backends: []secretstore.Backend{b}})
	if err != nil {
		t.Fatal(err)
	}
	value := []byte{0, 255, 10, 128}
	if err := store.Set(t.Context(), "token", value); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get(t.Context(), "token"); err != nil || !bytes.Equal(got, value) {
		t.Fatal("binary round trip failed")
	}
	other, err := secretstore.Open(secretstore.Config{Namespace: "app", Provider: b.Name(), Registry: secretstore.NewMemoryRegistry(), Backends: []secretstore.Backend{b}})
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Set(t.Context(), "token", []byte("different")); !errors.Is(err, secretstore.ErrConflict) {
		t.Fatal("unregistered existing value overwritten")
	}
}

func TestNativeTargetsDoNotCollideAcrossNamespaceSeparators(t *testing.T) {
	b := &Backend{Client: fakeClient{values: map[string]string{}}}
	a, err := b.Set(t.Context(), secretstore.WriteRequest{Namespace: "a:b", Key: "c"}, []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := b.Set(t.Context(), secretstore.WriteRequest{Namespace: "a", Key: "b:c"}, []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Fields["service"]+":"+a.Fields["account"] == c.Fields["service"]+":"+c.Fields["account"] {
		t.Fatal("flattened native targets collided")
	}
}
