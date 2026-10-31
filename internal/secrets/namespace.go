package secrets

import (
	"encoding/base64"
	"strings"
)

// CredentialNamespace names the CLI's password-manager entries independently
// of its executable name and configuration directory.
const CredentialNamespace = "a-wikimedia-cli"

const CredentialServicePrefix = CredentialNamespace + ":"

func CredentialTargetPrefixes() []string {
	encoded := "secretstore.v1." + base64.RawURLEncoding.EncodeToString([]byte(CredentialNamespace))
	return []string{CredentialServicePrefix, encoded + ":"}
}

func OwnsCredentialService(service string) bool {
	prefixes := CredentialTargetPrefixes()
	return strings.HasPrefix(service, prefixes[0]) || service == strings.TrimSuffix(prefixes[1], ":")
}
