package config

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSecretComparisonReportsEqualityWithoutExposingOrChangingValues(t *testing.T) {
	for _, result := range []string{"match", "different", "unavailable"} {
		t.Run(result, func(t *testing.T) {
			store := isolatedConfig(t)
			credential := seedCredential(t)
			if err := credential.SaveWithProvider("vault-private-fixture", false, nil, "keyring"); err != nil {
				t.Fatal(err)
			}
			plaintext := "plaintext-private-fixture"
			if result == "match" {
				plaintext = "vault-private-fixture"
			}
			if err := PutKeyValueOnDisk(credential.Key, plaintext); err != nil {
				t.Fatal(err)
			}
			if result == "unavailable" {
				store.failGet = true
			}
			before, _ := os.ReadFile(Path())
			reads := store.reads
			entries := SecretInventory()
			for _, entry := range entries {
				if entry.Comparison != "not-checked" {
					t.Fatal("metadata listing checked provider")
				}
			}
			if store.reads != reads {
				t.Fatal("metadata listing opened provider")
			}
			CheckSecretCopies(entries)
			if len(entries) != 2 || store.reads != reads+1 {
				t.Fatal("comparison should read each paired credential once")
			}
			for _, entry := range entries {
				if entry.Comparison != result {
					t.Fatalf("comparison = %s, want %s", entry.Comparison, result)
				}
				if strings.Contains(entry.Status, "not compared") || (result != "unavailable" && strings.Contains(entry.Status, "not checked")) {
					t.Fatal("comparison left contradictory metadata status")
				}
			}
			after, _ := os.ReadFile(Path())
			if !bytes.Equal(before, after) {
				t.Fatal("comparison modified config")
			}
			encoded, _ := json.Marshal(entries)
			if strings.Contains(string(encoded), "vault-private-fixture") || strings.Contains(string(encoded), "plaintext-private-fixture") {
				t.Fatal("comparison exposed credential values")
			}
		})
	}
}

func TestSecretComparisonDoesNotReadCredentialsWithoutPlaintextCopies(t *testing.T) {
	store := isolatedConfig(t)
	credential := seedCredential(t)
	if err := credential.SaveWithProvider("vault-private-fixture", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	reads := store.reads
	entries := SecretInventory()
	CheckSecretCopies(entries)
	if store.reads != reads || entries[0].Comparison != "" {
		t.Fatal("unpaired credential was read")
	}
}
