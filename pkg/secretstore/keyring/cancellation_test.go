package keyring

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

type blockingReadClient struct{ started, release, finished chan struct{} }

func (c blockingReadClient) Get(string, string) (string, error) {
	close(c.started)
	<-c.release
	close(c.finished)
	return "never-exposed-fixture", nil
}
func (blockingReadClient) Set(string, string, string) error { return nil }
func (blockingReadClient) Delete(string, string) error      { return nil }
func TestBlockedKeyringReadCanBeCanceled(t *testing.T) {
	client := blockingReadClient{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	backend := &Backend{Client: client}
	reference := secretstore.Reference{Provider: "keyring", Fields: map[string]string{"service": "fixture", "account": "fixture"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := backend.Get(ctx, reference); done <- err }()
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("read not started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked read ignored cancellation")
	}
	close(client.release)
	<-client.finished
}
