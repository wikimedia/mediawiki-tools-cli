package gerrit

import (
	"bytes"
	"crypto/ed25519"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"golang.org/x/crypto/ssh"
)

func TestRegisterGerritPublicKey(t *testing.T) {
	setupAuthTest(t)
	prefix := gerritProfiles.Prefix("fixture")
	credential := config.Credential{Key: prefix + ".password", Service: "gerrit", Endpoint: gerritProfiles.Endpoint, Account: "fixture"}
	if err := credential.SaveWithProvider("fixture-secret", false, map[string]string{prefix + ".username": "fixture"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	publicPath := filepath.Join(directory, "id_ed25519.pub")
	publicLine := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + " fixture"
	if err := os.WriteFile(publicPath, []byte(publicLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(directory, "id_ed25519")
	if err := os.WriteFile(privatePath, []byte("-----BEGIN OPENSSH PRIVATE KEY-----"), 0o600); err != nil {
		t.Fatal(err)
	}

	requests := 0
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Method != http.MethodPost || r.URL.Host != gerritSSHHost || r.URL.Path != "/r/a/accounts/self/sshkeys" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "fixture" || password != "fixture-secret" {
			t.Error("unexpected HTTP credentials")
		}
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != publicLine {
			t.Errorf("expected raw OpenSSH public key: %v", err)
		}
		result := ")]}'\n" + `{"seq":2,"algorithm":"ssh-ed25519","valid":true}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(result)), Request: r}, nil
	})

	run := func(path string) (string, error) {
		var output bytes.Buffer
		cmd := NewGerritCmd()
		cmd.SetArgs([]string{"ssh-key", "add", path, "--profile", "fixture"})
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		err := cmd.Execute()
		return output.String(), err
	}
	if _, err := run(privatePath); err == nil || requests != 0 {
		t.Fatalf("private-key file should fail without making a request, got %d requests and error %v", requests, err)
	}
	if output, err := run(publicPath); err != nil || requests != 1 || !strings.Contains(output, "Registered SSH key 2 for fixture") {
		t.Fatalf("registration failed with %d requests: %v\n%s", requests, err, output)
	}
}
