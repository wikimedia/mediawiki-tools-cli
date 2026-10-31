package secrets

import (
	"encoding/base64"
	"testing"
)

func TestCredentialNamespaceRejectsOldAndUnrelatedServices(t *testing.T) {
	encoded := "secretstore.v1." + base64.RawURLEncoding.EncodeToString([]byte("a-wikimedia-cli"))
	for _, tc := range []struct {
		service string
		owned   bool
	}{
		{"a-wikimedia-cli:gerrit:https://gerrit.wikimedia.org/r", true},
		{encoded, true},
		{"mwcli:gerrit:https://gerrit.wikimedia.org/r", false},
		{"secretstore.v1.bXdjbGk", false},
		{"a-wikimedia-clients:gerrit", false},
		{"other-app:credential", false},
		{"", false},
	} {
		if got := OwnsCredentialService(tc.service); got != tc.owned {
			t.Fatalf("ownership for %q: %t", tc.service, got)
		}
	}
}
