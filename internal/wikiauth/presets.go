package wikiauth

const (
	WikibaseWorldRealm    = "wikibase-world"
	WikibaseWorldAPI      = "https://wikibase.world/w/api.php"
	WikibaseWorldClientID = "8546b3108b91bd9ec7292c0aab4bb158"
)

// Preset binds a public client registration to its exact wiki authority.
type Preset struct{ Realm, API, ClientID string }

func BuiltinPreset(realm string) (Preset, bool) {
	switch realm {
	case "wikimedia":
		return Preset{Realm: realm, API: WikimediaAuthority + "/w/api.php", ClientID: DefaultClientID}, true
	case WikibaseWorldRealm:
		return Preset{Realm: realm, API: WikibaseWorldAPI, ClientID: WikibaseWorldClientID}, true
	default:
		return Preset{}, false
	}
}

func BuiltinRealm(endpoint string) string {
	if IsWikimedia(endpoint) {
		return "wikimedia"
	}
	if endpoint == WikibaseWorldAPI {
		return WikibaseWorldRealm
	}
	return ""
}
