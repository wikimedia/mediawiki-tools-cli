package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestCredentialUpdateCancelsKeePassReadBeforeTransformation(t *testing.T) {
	isolatedConfig(t)
	cred := seedGUITransfer(t)
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readKeePassXCBrowser = func(got context.Context, _ secrets.KeePassXCBrowser) (string, error) {
		if got != ctx {
			return "", errors.New("command context was replaced")
		}
		close(started)
		<-got.Done()
		return "", got.Err()
	}
	done := make(chan error, 1)
	transformed := false
	go func() {
		_, err := cred.Update(ctx, func(string) (string, error) { transformed = true; return "should-not-write", nil })
		done <- err
	}()
	select {
	case <-started:
	case err := <-done:
		t.Fatal("read lost context", err)
	case <-time.After(time.Second):
		t.Fatal("read did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || transformed {
			t.Fatal("transformation ran after cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled read did not return")
	}
	// Cancellation released the mutation lock; a later credential read can proceed.
	readKeePassXCBrowser = func(context.Context, secrets.KeePassXCBrowser) (string, error) { return "legacy-secret", nil }
	if _, err := cred.Update(context.Background(), func(value string) (string, error) { return value, nil }); err != nil {
		t.Fatal(err)
	}
}
