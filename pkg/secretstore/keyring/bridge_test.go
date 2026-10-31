package keyring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	oskeyring "github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

type bridgeFixture struct {
	values map[string]string
}

func (s *bridgeFixture) Get(service, account string) (string, error) {
	value, ok := s.values[service+account]
	if !ok {
		return "", oskeyring.ErrNotFound
	}
	return value, nil
}

func (s *bridgeFixture) Set(service, account, value string) error {
	s.values[service+account] = value
	return nil
}

func (s *bridgeFixture) Delete(service, account string) error {
	delete(s.values, service+account)
	return nil
}

func TestBridgeHelperProcess(t *testing.T) {
	if os.Getenv("MWCLI_BRIDGE_FIXTURE") != "1" {
		return
	}
	var request bridgeRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		os.Exit(2)
	}
	reply := bridgeResponse{Version: 1}
	switch {
	case request.Action == "list":
		reply.Entries = []CredentialMetadata{{Service: "a-wikimedia-cli:gerrit", Account: "fixture", Target: "a-wikimedia-cli:gerrit:fixture"}}
	case !strings.HasPrefix(request.Service, "a-wikimedia-cli:"):
		reply.Error = "invalid"
	case request.Action == "probe":
	case request.Account == "fixture":
		reply.Value = []byte("private-fixture-secret")
	default:
		reply.Error = "not-found"
	}
	if err := json.NewEncoder(os.Stdout).Encode(reply); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestBridgeClientUsesPipesAndPreservesErrors(t *testing.T) {
	t.Setenv("MWCLI_BRIDGE_FIXTURE", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	client := &BridgeClient{Executable: executable, Args: []string{"-test.run=^TestBridgeHelperProcess$"}, ProbeService: "a-wikimedia-cli:credential-store-probe"}
	value, err := client.Get("a-wikimedia-cli:gerrit", "fixture")
	if err != nil || value != "private-fixture-secret" {
		t.Fatalf("read failed: %v", err)
	}
	if _, err := client.Get("a-wikimedia-cli:gerrit", "missing"); !errors.Is(err, oskeyring.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := client.Get("other", "fixture"); !errors.Is(err, secretstore.ErrInvalid) {
		t.Fatalf("namespace restriction: %v", err)
	}
	if err := client.Probe(context.Background()); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if strings.Contains(strings.Join(client.Args, " "), value) {
		t.Fatal("secret in command arguments")
	}
}

func TestWindowsBackendIdentityAndLimit(t *testing.T) {
	client := &bridgeFixture{values: map[string]string{}}
	backend := &Backend{Client: client, Provider: WindowsProvider, MaxValueBytes: 2560}
	if backend.Name() != WindowsProvider || backend.Capabilities().MaxValueBytes != 2560 {
		t.Fatal("wrong capabilities")
	}
	ref, err := backend.Set(t.Context(), secretstore.WriteRequest{Namespace: "a-wikimedia-cli", Key: "fixture"}, []byte("private-fixture-secret"))
	if err != nil || ref.Provider != WindowsProvider {
		t.Fatalf("save: %v", err)
	}
	got, err := backend.Get(t.Context(), ref)
	if err != nil || string(got) != "private-fixture-secret" {
		t.Fatalf("read: %v", err)
	}
	ref.Provider = "keyring"
	if _, err := backend.Get(t.Context(), ref); err == nil {
		t.Fatal("cross-provider reference accepted")
	}
}

func TestBridgeMetadataListingOmitsCredentialValues(t *testing.T) {
	t.Setenv("MWCLI_BRIDGE_FIXTURE", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	client := &BridgeClient{Executable: executable, Args: []string{"-test.run=^TestBridgeHelperProcess$"}, ProbeService: "a-wikimedia-cli:credential-store-probe"}
	entries, err := client.ListMetadata()
	if err != nil || len(entries) != 1 || entries[0].Target != "a-wikimedia-cli:gerrit:fixture" {
		t.Fatalf("metadata listing: %+v, %v", entries, err)
	}
	data, _ := json.Marshal(entries)
	if bytes.Contains(data, []byte("private-fixture-secret")) {
		t.Fatal("credential value crossed metadata transport")
	}
}
