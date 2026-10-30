package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestSecretInventoryIncludesLinkedPlaintextOrphanAndPairingWithoutReads(t *testing.T) {
	store := isolatedConfig(t)
	credential := seedCredential(t)
	if err := credential.SaveWithProvider("stored-secret-fixture", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	// The inventory must detect leftover plaintext even for a linked credential.
	if err := PutKeyValueOnDisk("gerrit.password", "leftover-secret-fixture"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk("custom.api_token", "custom-secret-fixture"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk("custom.empty_password", ""); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk("phabricator.sites.bot.username", "Bot"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk("phabricator.sites.bot.url", secrets.PhabricatorEndpoint); err != nil {
		t.Fatal(err)
	}
	phab, err := CredentialForKey("phabricator.sites.bot.key")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := phab.reference()
	if err != nil {
		t.Fatal(err)
	}
	disk := State().OnDiskKoanf.Copy()
	if err := disk.Set(credentialReferenceKey(phab.Key), map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": "keepassxc-browser", "browser": map[string]any{"database_hash": "database-fixture", "entry_id": "entry-fixture", "login": "Bot"}}); err != nil {
		t.Fatal(err)
	}
	if err := disk.Set("credential_storage.orphan", map[string]string{"service": "mwcli:future:https://example.org", "endpoint": "https://example.org", "identity": "Someone", "provider": "keyring"}); err != nil {
		t.Fatal(err)
	}
	if err := disk.Set("credential_storage.malformed", 123); err != nil {
		t.Fatal(err)
	}
	if err := PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	pairing := secrets.KeePassXCPairingPath()
	if err := os.MkdirAll(filepath.Dir(pairing), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pairing, []byte("pairing-secret-fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := store.reads
	previous := readKeePassXCBrowser
	readKeePassXCBrowser = func(secrets.KeePassXCBrowser) (string, error) { t.Fatal("inventory read KeePassXC"); return "", nil }
	t.Cleanup(func() { readKeePassXCBrowser = previous })
	entries := SecretInventory()
	if store.reads != reads {
		t.Fatal("inventory opened the keyring")
	}
	byKey := map[string]SecretInfo{}
	var plaintextGerrit *SecretInfo
	for _, entry := range entries {
		if entry.Key == "gerrit.password" && entry.Source == "plaintext-config" {
			copy := entry
			plaintextGerrit = &copy
			continue
		}
		byKey[entry.Key] = entry
	}
	if len(entries) != 7 {
		t.Fatalf("got %d inventory entries", len(entries))
	}
	if gerrit := byKey["gerrit.password"]; gerrit.Provider != "keyring" || gerrit.Account != "fixture" || !gerrit.PlaintextPresent || !strings.Contains(gerrit.Status, "plaintext copy") {
		t.Fatalf("incomplete linked credential metadata: %+v", gerrit)
	}
	if plaintextGerrit == nil || plaintextGerrit.Provider != "plaintext" || plaintextGerrit.Location != Path() || plaintextGerrit.Reference != "" || !strings.Contains(plaintextGerrit.Status, "not compared") {
		t.Fatal("additional plaintext copy was not listed separately")
	}
	if plain := byKey["custom.api_token"]; plain.Provider != "plaintext" || !strings.Contains(plain.Status, "no credential provider mapping") {
		t.Fatal("custom plaintext credential omitted")
	}
	if phab := byKey["phabricator.sites.bot.key"]; phab.EntryID != "entry-fixture" || phab.DatabaseID != "database-fixture" || phab.Provider != "keepassxc" {
		t.Fatal("KeePassXC identifiers omitted")
	}
	if byKey["credential_storage.orphan"].Status != "unmapped reference (not checked)" {
		t.Fatal("orphan reference not reported")
	}
	if byKey["credential_storage.malformed"].Status != "invalid reference" {
		t.Fatal("malformed reference not reported")
	}
	if byKey["keepassxc-pairing"].Location != pairing {
		t.Fatal("pairing-file location omitted")
	}
	encoded, _ := json.Marshal(entries)
	for _, secret := range []string{"stored-secret-fixture", "leftover-secret-fixture", "custom-secret-fixture", "pairing-secret-fixture"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("inventory exposed a secret value")
		}
	}
}

func TestSecretInventoryFindsWikiTokensAfterPlaintextRemoved(t *testing.T) {
	isolatedConfig(t)
	a := WikiAccount{Realm: "wikimedia", Name: "personal", Username: "Fixture", Method: "oauth", Endpoint: "https://meta.wikimedia.org", Wiki: "https://meta.wikimedia.org/w/api.php", ClientID: "public-client"}
	if err := a.Credential().SaveWithProvider("oauth-token-fixture", false, a.Metadata(), "keyring"); err != nil {
		t.Fatal(err)
	}
	entries := SecretInventory()
	if len(entries) != 1 || entries[0].Key != a.Credential().Key || entries[0].Service != "wiki" || entries[0].Account != "Fixture" {
		t.Fatalf("saved OAuth reference not identified: %+v", entries)
	}
}
