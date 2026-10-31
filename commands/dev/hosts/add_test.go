package hosts

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

func TestHostsHelpDoesNotResolveDefaultIP(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	previous := net.DefaultResolver
	t.Cleanup(func() { net.DefaultResolver = previous })
	var lookups atomic.Int32
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			lookups.Add(1)
			return nil, errors.New("DNS must not be needed for command construction or help")
		},
	}
	for _, newCmd := range []func() *cobra.Command{NewHostsAddCmd, NewHostsRemoveCmd} {
		cmd := newCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--help"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if got := lookups.Load(); got != 0 {
		t.Fatalf("command construction/help performed %d DNS requests", got)
	}
}

func TestHostsToAddIncludesExtraHosts(t *testing.T) {
	hosts := hostsToAdd([]string{"client.mediawiki.local.wmftest.net"})

	foundDefault := false
	foundClient := false
	for _, host := range hosts {
		if host == "default.mediawiki.local.wmftest.net" {
			foundDefault = true
		}
		if host == "client.mediawiki.local.wmftest.net" {
			foundClient = true
		}
	}

	if !foundDefault {
		t.Fatal("expected default.mediawiki.local.wmftest.net in hosts list")
	}
	if !foundClient {
		t.Fatal("expected client.mediawiki.local.wmftest.net in hosts list")
	}
}
