package config

import (
	"os"
	"strings"
	"testing"
)

func TestEnterpriseCredentialInventoryTransferAndRedaction(t *testing.T) {
	isolatedConfig(t)
	const key = "enterprise.profiles.bot.token"
	// The bundle is opaque to config; its provider and profile identity have the
	// same lifecycle as credentials for the other services.
	cred := Credential{Key: key, Service: "enterprise", Endpoint: "https://api.enterprise.wikimedia.com", Account: "fixture"}
	if err := cred.SaveWithProvider("enterprise-bundle-secret", false, map[string]string{"enterprise.profiles.bot.username": "fixture"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	resolved, err := CredentialForKey(key)
	if err != nil || resolved != cred {
		t.Fatal("Enterprise binding missing", resolved, err)
	}
	if entries := SecretInventory(); len(entries) != 1 || entries[0].Service != "enterprise" {
		t.Fatalf("missing Enterprise inventory entry: %+v", entries)
	}
	if profiles := ProfileInventory(); len(profiles) != 1 || profiles[0].Service != "enterprise" || profiles[0].SecretKey != key || profiles[0].Account != "fixture" {
		t.Fatalf("missing Enterprise profile metadata: %+v", profiles)
	}
	if err := cred.TransferTo("plaintext"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(Redacted(State().OnDiskKoanf).Sprint(), "enterprise-bundle-secret") {
		t.Fatal("bundle not redacted")
	}
	if err := cred.TransferTo("keyring"); err != nil {
		t.Fatal(err)
	}
	disk, err := os.ReadFile(Path())
	if err != nil || strings.Contains(string(disk), "enterprise-bundle-secret") {
		t.Fatal("transfer left plaintext behind", err)
	}
	if value, err := cred.Read(); err != nil || value != "enterprise-bundle-secret" {
		t.Fatal("bundle lost during transfer", err)
	}
	if err := cred.Delete(); err != nil {
		t.Fatal(err)
	}
	if HasCredential(key) || len(SecretInventory()) != 0 {
		t.Fatal("profile removal retained active credential")
	}
}
