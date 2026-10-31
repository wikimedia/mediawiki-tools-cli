package config

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestStructuredCredentialMetadataCommitsWithVerifiedReference(t *testing.T) {
	for _, provider := range []string{"keyring", "keepassxc", "plaintext"} {
		t.Run(provider, func(t *testing.T) {
			isolatedConfig(t)
			previous := writeKeePassXCBrowser
			writeKeePassXCBrowser = func(context.Context, *secrets.KeePassXCBrowser, string, string, string, string) (secrets.KeePassXCBrowser, error) {
				return secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: "Fixture"}, nil
			}
			t.Cleanup(func() { writeKeePassXCBrowser = previous })
			a := WikiProfile{Realm: "wikimedia", Name: "personal", Username: "Fixture", Identity: "Fixture", Method: "oauth", Endpoint: wikiauth.WikimediaAuthority, Wiki: wikiauth.WikimediaAuthority + "/w/api.php", ClientID: wikiauth.DefaultClientID}
			metadata := map[string]any{}
			for key, value := range a.Metadata() {
				metadata[key] = value
			}
			sites := []string{a.Wiki, "https://test.wikipedia.org/w/api.php"}
			metadata["wiki.profiles.wikimedia.personal.sites"] = sites
			metadata["wiki.default_profile"] = "wikimedia/personal"
			if err := a.Credential().SaveWithProviderMetadata("private-metadata-fixture", false, metadata, provider); err != nil {
				t.Fatal(err)
			}
			saved, err := LoadWikiProfile("wikimedia", "personal")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(saved.Sites, sites) {
				t.Fatal("site list lost", saved.Sites)
			}
			if !State().OnDiskKoanf.Exists(credentialReferenceKey(a.Credential().Key)) {
				t.Fatal("metadata saved without reference")
			}
			if provider != "plaintext" && State().OnDiskKoanf.Exists(a.Credential().Key) {
				t.Fatal("secure save retained plaintext")
			}
		})
	}
}

func TestStructuredCredentialMetadataFailureDoesNotChangeDefaults(t *testing.T) {
	store := isolatedConfig(t)
	_ = PutKeyValueOnDisk("wiki.default_profile", "wikimedia/existing")
	c := Credential{Key: "wiki.profiles.wikimedia.new.token", Service: "wiki", Endpoint: wikiauth.WikimediaAuthority, Account: "Fixture"}
	for _, metadata := range []map[string]any{
		{"wiki.default_profile": "wikimedia/new", "private.password": []string{"secret"}},
		{"wiki.default_profile": "wikimedia/new", "sites": map[string]string{"token": "secret"}},
	} {
		if err := c.SaveWithProviderMetadata("fixture", false, metadata, "keyring"); err == nil {
			t.Fatal("invalid structured metadata accepted")
		}
	}
	store.failSet = true
	if err := c.SaveWithProviderMetadata("fixture", false, map[string]any{"wiki.default_profile": "wikimedia/new", "wiki.profiles.wikimedia.new.sites": []string{wikiauth.WikimediaAuthority + "/w/api.php"}}, "keyring"); err == nil {
		t.Fatal("failed provider write accepted")
	}
	if State().OnDiskKoanf.String("wiki.default_profile") != "wikimedia/existing" || State().OnDiskKoanf.Exists("wiki.profiles.wikimedia.new") {
		t.Fatal("failed save changed public profile metadata")
	}
}
