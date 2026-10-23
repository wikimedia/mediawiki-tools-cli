package hosts

import "testing"

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
