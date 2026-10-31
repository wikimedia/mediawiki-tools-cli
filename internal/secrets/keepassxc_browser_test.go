package secrets

import "testing"

func TestCredentialsUseFlatKeePassXCGroupAndDistinctURLs(t *testing.T) {
	keys := []string{
		"wiki.profiles.wikimedia.personal.token",
		"wiki.profiles.wikimedia.edit-test.token",
		"wiki.profiles.wikimedia.personal.password",
		"phabricator.profiles.personal.key",
		"phabricator.profiles.bot.key",
		"gerrit.password",
		"gerrit.profiles.Personal.password",
		"gerrit.profiles.personal.password",
	}
	seen := map[string]bool{}
	for _, key := range keys {
		client := browserClientForCredential(key)
		if client.Group != KeePassXCGroup || client.PairingFile != browserClient().PairingFile {
			t.Fatalf("credential %s changed the group or pairing", key)
		}
		if seen[client.EntryURL] {
			t.Fatalf("credential %s shares a storage URL", key)
		}
		seen[client.EntryURL] = true
	}
	if got := browserClientForCredential(keys[0]).EntryURL; got != "a-wikimedia-cli://"+keys[0] {
		t.Fatalf("unexpected storage URL: %s", got)
	}
}
