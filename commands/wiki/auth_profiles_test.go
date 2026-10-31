package wiki

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestAuthProfilesListsAccountsWithoutUnlocking(t *testing.T) {
	isolateWikiProfiles(t)
	seedWikiOAuth(t, "personal")
	keyring.MockInitWithError(errors.New("locked"))
	for _, name := range []string{"profiles", "list"} {
		out := &bytes.Buffer{}
		cmd := NewWikiCmd()
		cmd.SetOut(out)
		cmd.SetArgs([]string{"auth", name})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "wikimedia/personal") || strings.Contains(out.String(), "access-fixture") {
			t.Fatal(out.String())
		}
	}
}
