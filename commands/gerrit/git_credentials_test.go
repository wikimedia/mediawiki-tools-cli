package gerrit

import (
	"bytes"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestGerritGitHelperOnlySuppliesCredentialsForMatchingHTTPSAccount(t *testing.T) {
	setupAuthTest(t)
	prefix := gerritProfiles.Prefix("fixture")
	credential := config.Credential{Key: prefix + ".password", Service: "gerrit", Endpoint: gerritProfiles.Endpoint, Account: "fixture"}
	if err := credential.SaveWithProvider("fixture-secret", false, map[string]string{prefix + ".username": "fixture"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		input          string
		wantCredential bool
	}{
		{"protocol=https\nhost=gerrit.wikimedia.org\npath=r/test/gerrit-ping\n\n", true},
		{"protocol=https\nhost=another.example\npath=r/test/gerrit-ping\n\n", false},
		{"protocol=http\nhost=gerrit.wikimedia.org\npath=r/test/gerrit-ping\n\n", false},
		{"protocol=https\nhost=gerrit.wikimedia.org\npath=r/test/gerrit-ping\nusername=other\n\n", false},
		{"protocol=https\nhost=gerrit.wikimedia.org\npath=unrelated\n\n", false},
	} {
		var stdout, stderr bytes.Buffer
		cmd := NewGerritCmd()
		cmd.SetArgs([]string{"auth", "git-credential", "--profile", "fixture", "get"})
		cmd.SetIn(strings.NewReader(test.input))
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		err := cmd.ExecuteContext(t.Context())
		if test.wantCredential {
			if err != nil || stdout.String() != "username=fixture\npassword=fixture-secret\n\n" {
				t.Fatalf("could not read credential protocol: %v", err)
			}
		} else if err == nil || strings.Contains(stdout.String()+stderr.String(), "fixture-secret") {
			t.Fatalf("invalid request succeeded or leaked credentials: %q", test.input)
		}
	}
}

func TestGerritHTTPSPushKeepsCertificateVerification(t *testing.T) {
	env := gerritHTTPSPushEnv([]string{"GIT_SSL_NO_VERIFY=false", "git_ssl_no_verify=1", "GIT_SSL_CAINFO=/test/ca"})
	if strings.Contains(strings.ToUpper(strings.Join(env, "\n")), "GIT_SSL_NO_VERIFY=") {
		t.Fatal("Git must not inherit any value for GIT_SSL_NO_VERIFY")
	}
	if !strings.Contains(strings.Join(env, "\n"), "GIT_SSL_CAINFO=/test/ca") {
		t.Fatal("Git should retain the configured CA bundle")
	}
}
