package keyring

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

//go:embed windows_credentials.ps1
var windowsCredentialScript string

// NewPowerShellBridge uses Windows PowerShell's native API support without an
// installed helper executable or module. Only fixed code and the public namespace
// enter the command line; credential requests are sent over anonymous pipes.
func NewPowerShellBridge(executable, namespace string) (*BridgeClient, error) {
	if executable == "" || namespace == "" || !utf8.ValidString(namespace) || strings.ContainsAny(namespace, "\x00\r\n") {
		return nil, secretstore.ErrInvalid
	}
	script := strings.ReplaceAll(windowsCredentialScript, "__NAMESPACE_BASE64__", base64.StdEncoding.EncodeToString([]byte(namespace)))
	words := utf16.Encode([]rune(script))
	encoded := make([]byte, len(words)*2)
	for i, word := range words {
		encoded[2*i], encoded[2*i+1] = byte(word), byte(word>>8)
	}
	command := base64.StdEncoding.EncodeToString(encoded)
	// Windows command lines are limited to 32,767 UTF-16 code units.
	if len(command)+len(utf16.Encode([]rune(executable)))+128 > 32767 {
		return nil, fmt.Errorf("PowerShell credential bridge command is too large")
	}
	return &BridgeClient{
		Executable:   executable,
		Args:         []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", command},
		ProbeService: namespace + ":credential-store-probe",
	}, nil
}
