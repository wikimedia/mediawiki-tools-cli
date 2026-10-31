package phabricator

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestImportPhabConfigCopiesValidatedCredentials(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	path := filepath.Join(t.TempDir(), "phab.cfg")
	contents := "[main]\ndefault=bot\n[bot]\nurl=https://phabricator.wikimedia.org\nusername=Addbot\nkey=api-bot-secret\ndefault_project=example\n[other]\nkey=not-selected\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	cmd := NewPhabricatorCmd()
	cmd.SetOut(out)
	cmd.SetArgs([]string{"auth", "login", "--method", "phab-cfg", "--config-file", path, "--profile", "imported-bot", "--provider", "keyring"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig("imported-bot")
	if err != nil || cfg.Username != "Addbot" || cfg.Key != "api-bot-secret" || cfg.DefaultProject != "example" {
		t.Fatalf("imported config: %+v %v", cfg, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != contents {
		t.Fatal("source file changed", err)
	}
	saved, err := os.ReadFile(mwconfig.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved)+out.String(), "api-bot-secret") || strings.Contains(out.String(), "not-selected") {
		t.Fatal("import leaked a token")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	cfg, err = loadConfig("imported-bot")
	if err != nil || cfg.Key != "api-bot-secret" {
		t.Fatal("profile still depends on source", err)
	}
}

func TestPhabConfigDetectionDoesNotUseToken(t *testing.T) {
	isolatedAuthConfig(t)
	directory := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.WriteFile("phab.cfg", []byte("[main]\ndefault=personal\n[personal]\nkey=external-private-fixture\nurl=https://untrusted.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = authTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("detected credential sent over network")
		return nil, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	_, err = loadConfig("personal")
	if err == nil || !strings.Contains(err.Error(), "--method phab-cfg") || strings.Contains(err.Error(), "external-private-fixture") {
		t.Fatalf("missing safe import suggestion: %v", err)
	}
	if mwconfig.HasCredential("phabricator.profiles.personal.key") {
		t.Fatal("detection saved credentials")
	}
}

func TestImportPhabConfigRejectsInvalidAccountAndDestination(t *testing.T) {
	for _, scenario := range []string{"invalid token", "wrong account", "different destination"} {
		t.Run(scenario, func(t *testing.T) {
			isolatedAuthConfig(t)
			mockLoginIdentity(t)
			path := filepath.Join(t.TempDir(), "phab.cfg")
			contents := "[main]\ndefault=source\n[source]\nurl=https://phabricator.wikimedia.org\nusername=Addbot\nkey=api-bot-secret\n"
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"auth", "login", "--method", "phab-cfg", "--config-file", path, "--profile", "imported", "--provider", "keyring"}
			switch scenario {
			case "invalid token":
				http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error_code":"ERR-INVALID-AUTH","error_info":"Invalid token"}`)), Request: r}, nil
				})
			case "wrong account":
				args = append(args, "--username", "Addshore")
			case "different destination":
				args = append(args, "--url", "https://other.example")
			}
			cmd := NewPhabricatorCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("invalid import succeeded")
			}
			if mwconfig.HasCredential("phabricator.profiles.imported.key") {
				t.Fatal("invalid import saved credentials")
			}
			if phabProfiles.Default() != "" {
				t.Fatal("invalid import changed default")
			}
		})
	}
}
