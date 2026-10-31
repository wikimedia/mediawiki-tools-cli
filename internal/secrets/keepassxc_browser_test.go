package secrets

import "testing"

func TestPhabricatorProfilesHaveSeparateKeePassXCLocations(t *testing.T) {
	personal := browserClientForCredential("phabricator.profiles.personal.key")
	bot := browserClientForCredential("phabricator.profiles.bot.key")
	if personal.Group == bot.Group || personal.Group == "wm" {
		t.Fatal("Phabricator profiles share a vault location")
	}
	if personal.PairingFile != bot.PairingFile {
		t.Fatal("profiles changed database pairing")
	}
}
