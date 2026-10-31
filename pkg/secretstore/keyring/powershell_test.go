package keyring

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	oskeyring "github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

func TestPowerShellBridgeCommand(t *testing.T) {
	client, err := NewPowerShellBridge("powershell.exe", "test-'\";namespace")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(client.Args[:4], " ") != "-NoLogo -NoProfile -NonInteractive -EncodedCommand" {
		t.Fatal("PowerShell must not load profiles or prompt")
	}
	encoded, err := base64.StdEncoding.DecodeString(client.Args[4])
	if err != nil {
		t.Fatal(err)
	}
	words := make([]uint16, len(encoded)/2)
	for i := range words {
		words[i] = binary.LittleEndian.Uint16(encoded[i*2:])
	}
	script := string(utf16.Decode(words))
	if strings.Contains(script, "__NAMESPACE_BASE64__") || strings.Contains(script, "test-'\";namespace") {
		t.Fatal("namespace was not safely encoded")
	}
	for _, namespace := range []string{"", "invalid\x00namespace", "invalid\nnamespace", "invalid\xffnamespace"} {
		if _, err := NewPowerShellBridge("powershell.exe", namespace); !errors.Is(err, secretstore.ErrInvalid) {
			t.Fatalf("invalid namespace accepted: %v", err)
		}
	}
	if _, err := NewPowerShellBridge("powershell.exe", strings.Repeat("n", 20000)); err == nil {
		t.Fatal("oversized Windows command line accepted")
	}
}

// Opt in on Windows or WSL. All items use a random test namespace, never saved
// application credentials, and are deleted even if an assertion fails.
func TestPowerShellBridgeCredentialManager(t *testing.T) {
	if os.Getenv("MWCLI_TEST_WINDOWS_CREDENTIALS") != "1" {
		t.Skip("set MWCLI_TEST_WINDOWS_CREDENTIALS=1 to exercise Windows Credential Manager")
	}
	path, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	namespace := "a-wikimedia-cli-test-" + hex.EncodeToString(id)
	client, err := NewPowerShellBridge(path, namespace)
	if err != nil {
		t.Fatal(err)
	}
	service := "secretstore.v1." + base64.RawURLEncoding.EncodeToString([]byte(namespace))
	account := "temporary:account-\u00e9"
	t.Cleanup(func() {
		if err := client.Delete(service, account); err != nil && !errors.Is(err, oskeyring.ErrNotFound) {
			t.Errorf("temporary credential cleanup failed: %v", err)
		}
	})
	legacyService := namespace + ":gerrit"
	t.Cleanup(func() {
		if err := client.Delete(legacyService, account); err != nil && !errors.Is(err, oskeyring.ErrNotFound) {
			t.Errorf("temporary legacy credential cleanup failed: %v", err)
		}
	})
	if err := client.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(service, account); !errors.Is(err, oskeyring.ErrNotFound) {
		t.Fatalf("missing credential: %v", err)
	}
	for _, value := range []string{"fixture secret \"$();\r\n\u00e9\U0001f511", "", strings.Repeat("x", 2560)} {
		if err := client.Set(service, account, value); err != nil {
			t.Fatal(err)
		}
		got, err := client.Get(service, account)
		if err != nil || got != value {
			t.Fatalf("credential did not round trip: %v", err)
		}
	}
	if err := client.Set(legacyService, account, "legacy-slot-fixture"); err != nil {
		t.Fatal(err)
	}
	if got, err := client.Get(legacyService, account); err != nil || got != "legacy-slot-fixture" {
		t.Fatalf("legacy target did not round trip: %v", err)
	}
	entries, err := client.ListMetadata()
	if err != nil || len(entries) != 2 {
		t.Fatalf("metadata scan: %v, %v", entries, err)
	}
	for _, entry := range entries {
		if entry.Account != account || (entry.Service != service && entry.Service != legacyService) || entry.Target != entry.Service+":"+account {
			t.Fatal("unexpected credential metadata")
		}
	}
	data, _ := json.Marshal(entries)
	if bytes.Contains(data, []byte(strings.Repeat("x", 2560))) {
		t.Fatal("metadata exposed secret")
	}
	if err := client.Set(service, account, strings.Repeat("x", 2561)); !errors.Is(err, secretstore.ErrTooLarge) {
		t.Fatalf("oversized credential accepted: %v", err)
	}
	for _, foreign := range []string{"other-application-" + hex.EncodeToString(id), namespace + "-other:service", strings.ToUpper(namespace) + ":service"} {
		if err := client.Set(foreign, account, "fixture"); !errors.Is(err, secretstore.ErrInvalid) {
			t.Fatalf("foreign namespace accepted: %v", err)
		}
	}
	if err := client.Delete(service, account); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(service, account); !errors.Is(err, oskeyring.ErrNotFound) {
		t.Fatalf("deleted credential still present: %v", err)
	}
	for _, input := range []string{`{"version":2,"action":"list"}`, `{"version":"1","action":"list"}`, `{"version":1,"action":"unknown"}`, `[{"version":1,"action":"list"}]`, `{`, strings.Repeat(" ", bridgeLimit+1)} {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		cmd := exec.CommandContext(ctx, client.Executable, client.Args...)
		cmd.Stdin = strings.NewReader(input)
		output, err := cmd.Output()
		cancel()
		var reply bridgeResponse
		if err != nil || json.Unmarshal(output, &reply) != nil || reply.Error != "invalid" {
			t.Fatalf("malformed bridge request: error = %v, response error = %q", err, reply.Error)
		}
	}
}
