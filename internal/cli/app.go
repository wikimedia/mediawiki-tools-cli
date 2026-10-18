package cli

import (
	"path/filepath"
	"strings"
	"unicode"
)

const DisplayName = "a Wikimedia CLI"

const Description = "Explore Wikimedia data, work with services, and develop MediaWiki."

// DefaultCommandName is used for generated documentation and unnamed invocations.
const DefaultCommandName = "mw"

// APPNAME is the stable configuration identity, independent of branding or argv[0].
// TODO: migrate the mwcli configuration directory separately, preserving existing
// credentials and supporting users who run old and new executables side by side.
const APPNAME string = "mwcli"

// InvocationName preserves a renamed executable or symlink's name. Do not use
// os.Executable here: resolving a symlink loses the name typed by the user.
func InvocationName(arg0 string) string {
	name := filepath.Base(strings.ReplaceAll(arg0, "\\", "/"))
	if strings.EqualFold(filepath.Ext(name), ".exe") {
		name = name[:len(name)-4]
	}
	if name == "" || name == "." || name == "/" || strings.ContainsFunc(name, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return DefaultCommandName
	}
	return name
}

// BinaryNames returns supported release/archive names, with the default first.
// The installed executable's arbitrary local filename is not a release identity.
func BinaryNames() []string {
	if DefaultCommandName == "wm" {
		return []string{"wm", "mw"}
	}
	return []string{"mw", "wm"}
}
