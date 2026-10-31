package wiki

import (
	"context"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestOAuthPortConflictFallsBackToPaste(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	redirect := "http://" + listener.Addr().String() + "/auth/login"
	a, err := wikiauth.Begin(wikiauth.DefaultClientID, redirect)
	if err != nil {
		t.Fatal(err)
	}
	callback := redirect + "?" + url.Values{"code": {"private-fixture-code"}, "state": {a.State}}.Encode()
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(callback + "\n"))
	var output strings.Builder
	cmd.SetErr(&output)
	got, err := authorizeWiki(context.Background(), cmd, a, false, true)
	if err != nil || got != callback {
		t.Fatal("occupied port did not allow paste", err)
	}
	if strings.Contains(output.String(), "private-fixture-code") {
		t.Fatal("callback code leaked")
	}
	if !strings.Contains(output.String(), "Could not listen") {
		t.Fatal("missing fallback explanation")
	}
}

func TestOAuthListenerWaitHonorsCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redirect := "http://" + listener.Addr().String() + "/auth/login"
	_ = listener.Close()
	a, err := wikiauth.Begin(wikiauth.DefaultClientID, redirect)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetErr(io.Discard)
	_, err = authorizeWiki(ctx, cmd, a, false, true)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("listener wait ignored cancellation: %v", err)
	}
}
