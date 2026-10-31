package config

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/credentialcheck"
)

func TestInventoryValidatesEachCopyIndependentlyAndCachesVaultRead(t *testing.T) {
	store := isolatedConfig(t)
	credential := seedCredential(t)
	if err := credential.SaveWithProvider("old-vault-private", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk(credential.Key, "new-plaintext-private"); err != nil {
		t.Fatal(err)
	}
	old := checkServiceCredential
	t.Cleanup(func() { checkServiceCredential = old })
	values := map[string]int{}
	checkServiceCredential = func(_ context.Context, input credentialcheck.Request) credentialcheck.Result {
		values[input.Value]++
		if input.Service != "gerrit" || input.Account != credential.Account || input.Endpoint != credential.Endpoint {
			t.Fatal("wrong credential identity checked")
		}
		if input.Value == "new-plaintext-private" {
			return credentialcheck.Result{Status: "valid", Identity: input.Account}
		}
		return credentialcheck.Result{Status: "invalid"}
	}
	before, _ := os.ReadFile(Path())
	reads := store.reads
	entries := SecretInventory()
	InspectSecretInventory(t.Context(), entries, true, true)
	if store.reads != reads+1 {
		t.Fatal("vault was read repeatedly for comparison and validation")
	}
	if values["old-vault-private"] != 1 || values["new-plaintext-private"] != 1 {
		t.Fatal("both source values were not checked independently")
	}
	for _, entry := range entries {
		if entry.Comparison != "different" {
			t.Fatal("comparison missing")
		}
		want := "invalid"
		if entry.Source == "plaintext-config" {
			want = "valid"
		}
		if entry.Validation != want {
			t.Fatalf("source %s status = %s, want %s", entry.Source, entry.Validation, want)
		}
	}
	after, _ := os.ReadFile(Path())
	if !bytes.Equal(before, after) {
		t.Fatal("verification changed configuration")
	}
	encoded, _ := json.Marshal(entries)
	if strings.Contains(string(encoded), "old-vault-private") || strings.Contains(string(encoded), "new-plaintext-private") {
		t.Fatal("validation exposed values")
	}
}

func TestUnavailableVaultDoesNotPreventPlaintextValidation(t *testing.T) {
	store := isolatedConfig(t)
	credential := seedCredential(t)
	if err := credential.SaveWithProvider("old-vault-private", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk(credential.Key, "new-plaintext-private"); err != nil {
		t.Fatal(err)
	}
	store.failGet = true
	old := checkServiceCredential
	t.Cleanup(func() { checkServiceCredential = old })
	checks := 0
	checkServiceCredential = func(_ context.Context, input credentialcheck.Request) credentialcheck.Result {
		checks++
		if input.Value != "new-plaintext-private" {
			t.Fatal("wrong value")
		}
		return credentialcheck.Result{Status: "valid"}
	}
	entries := SecretInventory()
	InspectSecretInventory(t.Context(), entries, false, true)
	if checks != 1 {
		t.Fatal("locked vault prevented checking plaintext")
	}
	for _, entry := range entries {
		want := "unavailable"
		if entry.Source == "plaintext-config" {
			want = "valid"
		}
		if entry.Validation != want {
			t.Fatal("incorrect per-source status")
		}
	}
}

func TestInventoryExplainsWrongAccountWithoutChangingConfig(t *testing.T) {
	isolatedConfig(t)
	credential := seedCredential(t)
	if err := PutKeyValueOnDisk(credential.Key, "private-fixture"); err != nil {
		t.Fatal(err)
	}
	old := checkServiceCredential
	t.Cleanup(func() { checkServiceCredential = old })
	checkServiceCredential = func(context.Context, credentialcheck.Request) credentialcheck.Result {
		return credentialcheck.Result{Status: "wrong-account", Identity: "Addbot"}
	}
	before, _ := os.ReadFile(Path())
	entries := SecretInventory()
	InspectSecretInventory(t.Context(), entries, false, true)
	found := false
	for _, entry := range entries {
		if entry.Key != credential.Key {
			continue
		}
		found = true
		if entry.AuthenticatedAs != "Addbot" || !strings.Contains(entry.Status, "authenticated as Addbot; expected "+credential.Account) {
			t.Fatalf("missing account diagnostic: %+v", entry)
		}
	}
	if !found {
		t.Fatal("credential missing")
	}
	after, _ := os.ReadFile(Path())
	if !bytes.Equal(before, after) {
		t.Fatal("verification changed configuration")
	}
	encoded, _ := json.Marshal(entries)
	if strings.Contains(string(encoded), "private-fixture") {
		t.Fatal("verification exposed secret")
	}
}

func TestInventoryChecksCloudCopiesWithTheirOwnCredentialIDs(t *testing.T) {
	isolatedConfig(t)
	key := "cloud_vps.projects.demo.credential.secret"
	idKey := "cloud_vps.projects.demo.credential.id"
	if err := PutKeyValueOnDisk(idKey, "old-id"); err != nil {
		t.Fatal(err)
	}
	credential, err := CredentialForKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := credential.SaveWithProvider("old-private", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk(idKey, "new-id"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk(key, "new-private"); err != nil {
		t.Fatal(err)
	}
	current, _ := CredentialForKey(key)
	if _, err := current.Read(); err == nil {
		t.Fatal("normal reads must still reject mismatched account")
	}
	old := checkServiceCredential
	t.Cleanup(func() { checkServiceCredential = old })
	checks := map[string]string{}
	checkServiceCredential = func(_ context.Context, input credentialcheck.Request) credentialcheck.Result {
		checks[input.Account] = input.Value
		return credentialcheck.Result{Status: "valid"}
	}
	before, _ := os.ReadFile(Path())
	entries := SecretInventory()
	InspectSecretInventory(t.Context(), entries, true, true)
	if checks["demo:old-id"] != "old-private" || checks["demo:new-id"] != "new-private" || len(checks) != 2 {
		t.Fatal("copies were not authenticated with their own IDs")
	}
	for _, entry := range entries {
		if entry.Key == key && (entry.Comparison != "different" || entry.Validation != "valid") {
			t.Fatalf("comparison or authentication unavailable: %+v", entry)
		}
	}
	after, _ := os.ReadFile(Path())
	if !bytes.Equal(before, after) {
		t.Fatal("inspection changed configuration")
	}
}
