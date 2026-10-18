package config

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestSecretConfigCommands(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	mwconfig.State()
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	if err := mwconfig.PutKeyValueOnDisk("gerrit.username", "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutKeyValueOnDisk("gerrit.password", "fixture-secret"); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("locked"))
	for _, args := range [][]string{{"show"}, {"get", "gerrit"}, {"secrets", "migrate", "--dry-run"}} {
		out := &bytes.Buffer{}
		cmd := Cmd()
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "fixture-secret") {
			t.Fatalf("secret in output for %v", args)
		}
	}
	failed := Cmd()
	failed.SetArgs([]string{"secrets", "migrate"})
	failed.SetOut(&bytes.Buffer{})
	failed.SetErr(&bytes.Buffer{})
	if err := failed.Execute(); err == nil {
		t.Fatal("migration ignored unavailable keyring")
	}
	if mwconfig.State().OnDiskKoanf.String("gerrit.password") != "fixture-secret" {
		t.Fatal("failed migration removed plaintext")
	}
	keyring.MockInit()
	out := &bytes.Buffer{}
	cmd := Cmd()
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("replacement-secret\n"))
	cmd.SetArgs([]string{"set", "gerrit.password", "--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(mwconfig.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data)+out.String(), "replacement-secret") || strings.Contains(string(data), "fixture-secret") {
		t.Fatal("set leaked credential")
	}
	credential, err := mwconfig.CredentialForKey("gerrit.password")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := credential.Read(); got != "replacement-secret" || err != nil {
		t.Fatalf("wrong keyring value: %v", err)
	}
}
