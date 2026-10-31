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
	fail   bool
}

func (s *bridgeFixture) Get(service, account string) (string, error) {
	if s.fail {
		return "", errors.New("private-fixture-secret")
	}
	value, ok := s.values[service+account]
	if !ok {
		return "", oskeyring.ErrNotFound
	}
	return value, nil
}

func (s *bridgeFixture) Set(service, account, value string) error {
	if s.fail {
		return errors.New("private-fixture-secret")
	}
	s.values[service+account] = value
	return nil
}

func (s *bridgeFixture) Delete(service, account string) error {
	delete(s.values, service+account)
	return nil
}

func TestBridgeProtocol(t *testing.T) {
	client := &bridgeFixture{values: map[string]string{}}
	call := func(request bridgeRequest) bridgeResponse {
		t.Helper()
		input, _ := json.Marshal(request)
		var output bytes.Buffer
		if err := ServeBridge(bytes.NewReader(input), &output, client, func(service string) bool { return strings.HasPrefix(service, "a-wikimedia-cli:") }); err != nil {
			t.Fatal(err)
		}
		var reply bridgeResponse
		if json.Unmarshal(output.Bytes(), &reply) != nil || reply.Version != 1 {
			t.Fatal("invalid response")
		}
		return reply
	}
	for _, action := range []string{"set", "get", "delete"} {
		request := bridgeRequest{Version: 1, Action: action, Service: "a-wikimedia-cli:gerrit", Account: "fixture"}
		if action == "set" {
			request.Value = []byte("private-fixture-secret")
		}
		reply := call(request)
		if reply.Error != "" {
			t.Fatal(reply.Error)
		}
		if action == "get" && string(reply.Value) != "private-fixture-secret" {
			t.Fatal("value changed")
		}
	}
	if reply := call(bridgeRequest{Version: 1, Action: "get", Service: "a-wikimedia-cli:gerrit", Account: "fixture"}); reply.Error != "not-found" {
		t.Fatal("missing item not reported")
	}
	for _, request := range []bridgeRequest{
		{Version: 1, Action: "get", Service: "unrelated", Account: "fixture"},
		{Version: 2, Action: "get", Service: "a-wikimedia-cli:gerrit", Account: "fixture"},
		{Version: 1, Action: "enumerate", Service: "a-wikimedia-cli:gerrit", Account: "fixture"},
		{Version: 1, Action: "get", Service: "a-wikimedia-cli:gerrit", Account: "fixture\x00"},
	} {
		if reply := call(request); reply.Error != "invalid" || len(reply.Value) != 0 {
			t.Fatal("accepted invalid request")
		}
	}
	if reply := call(bridgeRequest{Version: 1, Action: "set", Service: "a-wikimedia-cli:gerrit", Account: "fixture", Value: []byte(strings.Repeat("x", 2561))}); reply.Error != "too-large" {
		t.Fatal("accepted oversized value")
	}
	client.fail = true
	if reply := call(bridgeRequest{Version: 1, Action: "get", Service: "a-wikimedia-cli:gerrit", Account: "fixture"}); reply.Error != "unavailable" || strings.Contains(string(reply.Value)+reply.Error, "private-fixture-secret") {
		t.Fatal("provider diagnostic leaked")
	}
}

func TestBridgeHelperProcess(t *testing.T) {
	if os.Getenv("MWCLI_BRIDGE_FIXTURE") != "1" {
		return
	}
	client := &bridgeFixture{values: map[string]string{"a-wikimedia-cli:gerritfixture": "private-fixture-secret"}}
	if err := ServeBridge(os.Stdin, os.Stdout, client, func(s string) bool { return strings.HasPrefix(s, "a-wikimedia-cli:") }); err != nil {
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

func (s *bridgeFixture) ListMetadata() ([]CredentialMetadata, error) {
	return []CredentialMetadata{{Service: "a-wikimedia-cli:gerrit", Account: "fixture", Target: "a-wikimedia-cli:gerrit:fixture"}}, nil
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
