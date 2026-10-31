package config

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestConfigMutationProcess(t *testing.T) {
	if os.Getenv("MWCLI_TEST_METADATA_WRITER") != "1" {
		return
	}
	State() // Deliberately cache the configuration before the other process moves it.
	fmt.Println("ready")
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk("gerrit.default_profile", "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteKeyValueFromDisk("review.remove"); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataWriterPreservesConcurrentCredentialTransfer(t *testing.T) {
	isolatedConfig(t)
	mockWindowsStore(t)
	cred := Credential{Key: "gerrit.profiles.fixture.password", Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: "fixture"}
	if err := cred.SaveWithProvider("private-fixture", false, map[string]string{"gerrit.profiles.fixture.username": "fixture", "review.remove": "old"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConfigMutationProcess$")
	child.Env = append(os.Environ(), "MWCLI_TEST_METADATA_WRITER=1")
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	child.Stderr = &diagnostics
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill() }()
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatalf("child not ready: %s", diagnostics.String())
	}
	if err := cred.TransferTo(secrets.WindowsProvider); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk("review.preserve", "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintln(input, "continue"); err != nil {
		t.Fatal(err)
	}
	_ = input.Close()
	// Drain the test runner's final PASS before waiting for process exit.
	for scanner.Scan() {
	}
	if err := child.Wait(); err != nil {
		t.Fatalf("metadata writer: %v; %s", err, diagnostics.String())
	}
	unlock, err := lockCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if got, err := cred.Read(); err != nil || got != "private-fixture" {
		t.Fatalf("credential transfer was undone: %v", err)
	}
	disk := State().OnDiskKoanf
	if disk.String("review.preserve") != "new" || disk.Exists("review.remove") || disk.String("gerrit.default_profile") != "fixture" {
		t.Fatal("metadata writes lost a concurrent change")
	}
}

func TestTransferCanKeepPlaintextSource(t *testing.T) {
	for _, provider := range []string{"keyring", secrets.WindowsProvider} {
		for _, keep := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/keep=%t", provider, keep), func(t *testing.T) {
				isolatedConfig(t)
				mockWindowsStore(t)
				cred := seedCredential(t)
				if err := cred.SaveWithProvider("private-fixture", false, nil, "plaintext"); err != nil {
					t.Fatal(err)
				}
				if err := cred.TransferTo(provider, TransferOptions{KeepSource: keep}); err != nil {
					t.Fatal(err)
				}
				if got := State().OnDiskKoanf.String(cred.Key); (got == "private-fixture") != keep {
					t.Fatal("plaintext retention did not respect --keep-source")
				}
				if got, err := cred.Read(); err != nil || got != "private-fixture" {
					t.Fatal("destination is not usable", err)
				}
			})
		}
	}
}

func TestCredentialUpdatePersistsRotationAfterCancellation(t *testing.T) {
	isolatedConfig(t)
	cred := seedCredential(t)
	if err := cred.SaveWithProvider("old-fixture", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	value, err := cred.Update(ctx, func(current string) (string, error) {
		if current != "old-fixture" {
			t.Fatal("wrong source bundle")
		}
		cancel() // The token endpoint has already returned a successful rotation.
		return "rotated-fixture", nil
	})
	if err != nil || value != "rotated-fixture" {
		t.Fatal("successful rotation was discarded", err)
	}
	if saved, err := cred.Read(); err != nil || saved != "rotated-fixture" {
		t.Fatal("rotated bundle was not persisted", err)
	}
}
