package secrets

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

const (
	testBrowserHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testBrowserUUID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("MWCLI_KEEPASSXC_BROWSER_TEST_HELPER"); mode != "" {
		runBrowserHelper(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runBrowserHelper(mode string) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		os.Exit(20)
	}
	var peer [32]byte
	created := false
	password := "fixture-service-secret"
	for {
		var header [4]byte
		if _, err := io.ReadFull(os.Stdin, header[:]); err != nil {
			return
		}
		size := binary.LittleEndian.Uint32(header[:])
		if size == 0 || size > 1024*1024 {
			os.Exit(21)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(os.Stdin, data); err != nil {
			os.Exit(22)
		}
		var req browserMessage
		if json.Unmarshal(data, &req) != nil {
			os.Exit(23)
		}
		if name := os.Getenv("WM_BROWSER_ACTION_LOG"); name != "" {
			f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				os.Exit(24)
			}
			fmt.Fprintln(f, req.Action)
			f.Close()
		}
		send := func(reply map[string]any) {
			data, _ := json.Marshal(reply)
			binary.LittleEndian.PutUint32(header[:], uint32(len(data)))
			os.Stdout.Write(header[:])
			os.Stdout.Write(data)
		}
		if mode == "oversized" {
			binary.LittleEndian.PutUint32(header[:], 2*1024*1024)
			os.Stdout.Write(header[:])
			return
		}
		nbytes, err := base64.StdEncoding.DecodeString(req.Nonce)
		if err != nil || len(nbytes) != 24 {
			os.Exit(25)
		}
		var nonce [24]byte
		copy(nonce[:], nbytes)
		responseNonce := nextBrowserNonce(nonce)
		if req.Action == "change-public-keys" {
			key, err := base64.StdEncoding.DecodeString(req.PublicKey)
			if err != nil || len(key) != 32 {
				os.Exit(26)
			}
			copy(peer[:], key)
			send(map[string]any{"action": req.Action, "success": "true", "nonce": base64.StdEncoding.EncodeToString(responseNonce[:]), "publicKey": base64.StdEncoding.EncodeToString(pub[:])})
			continue
		}
		data, err = base64.StdEncoding.DecodeString(req.Message)
		if err != nil {
			os.Exit(27)
		}
		data, ok := box.Open(nil, data, &nonce, &peer, priv)
		if !ok {
			os.Exit(28)
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(data, &fields) != nil {
			os.Exit(29)
		}
		if req.Action == "get-databasehash" && mode == "locked" {
			send(map[string]any{"action": req.Action, "errorCode": "1", "error": "sensitive-error-text"})
			continue
		}
		if req.Action == "test-associate" && mode == "revoked" {
			send(map[string]any{"action": req.Action, "errorCode": "8"})
			continue
		}
		hash := testBrowserHash
		if mode == "wrong-database" {
			hash = strings.Repeat("c", 64)
		}
		payload := map[string]any{"action": req.Action, "success": "true", "nonce": base64.StdEncoding.EncodeToString(responseNonce[:]), "hash": hash}
		switch req.Action {
		case "get-databasehash":
		case "associate":
			payload["id"] = "wm-test"
		case "test-associate":
			var id, key string
			json.Unmarshal(fields["id"], &id)
			json.Unmarshal(fields["key"], &key)
			if id != "wm-test" || key == "" {
				os.Exit(30)
			}
			payload["id"] = "wm-test"
		case "get-logins":
			var url string
			json.Unmarshal(fields["url"], &url)
			if url != "keepassxc://by-path/wm/gerrit.wikimedia.org" && url != "keepassxc://by-path/wm/gerrit.password" && url != "keepassxc://by-uuid/"+testBrowserUUID && url != "keepassxc://by-path/wm/gerrit.password/gerrit.wikimedia.org" {
				os.Exit(31)
			}
			if mode == "legacy-recovery" && url == "keepassxc://by-path/wm/gerrit.wikimedia.org" {
				send(map[string]any{"action": req.Action, "errorCode": "15"})
				continue
			}
			if strings.HasPrefix(mode, "create") && ((!created && url != "keepassxc://by-uuid/"+testBrowserUUID && !(mode == "create-other-account" && url == "keepassxc://by-path/wm/gerrit.wikimedia.org")) || url == "keepassxc://by-path/wm/gerrit.password/gerrit.wikimedia.org") {
				send(map[string]any{"action": req.Action, "errorCode": "15"})
				continue
			}
			entry := BrowserEntry{UUID: testBrowserUUID, Login: "fixture", Password: password}
			switch mode {
			case "wrong-entry":
				entry.UUID = strings.Repeat("d", 32)
			case "wrong-login":
				entry.Login = "someone-else"
			case "expired":
				entry.Expired = "true"
			case "empty":
				entry.Password = ""
			}
			entries := []BrowserEntry{entry}
			if mode == "create-other-account" && !created {
				entries = []BrowserEntry{{UUID: strings.Repeat("c", 32), Login: "other-account", Password: "other-fixture-secret"}}
			}
			if mode == "denied" {
				entries = []BrowserEntry{}
			}
			if mode == "multiple-accounts" && strings.Contains(url, "by-path") {
				entries = append(entries, BrowserEntry{UUID: strings.Repeat("c", 32), Login: "other-account", Password: "other-fixture-secret"})
			}
			if mode == "duplicate-account" && strings.Contains(url, "by-path") {
				entries = append(entries, entry)
			}
			if mode == "multiple" {
				entries = append(entries, entry)
			}
			payload["entries"] = entries
			if mode == "numeric-entries" {
				payload["entries"] = 0
			}
		case "create-new-group":
			var name string
			json.Unmarshal(fields["groupName"], &name)
			if name != "wm" {
				os.Exit(33)
			}
			if mode == "create-denied" {
				send(map[string]any{"action": req.Action, "errorCode": "6"})
				continue
			}
			payload["uuid"] = strings.Repeat("e", 32)
		case "set-login":
			// BrowserAction passes QJsonValue::Null through QVariant. KeePassXC
			// versions can serialize the enum as 0 rather than a JSON array/null.
			payload["entries"] = 0
			payload["count"] = 0
			payload["error"] = "success"
			var login, value, endpoint, uuid, group string
			json.Unmarshal(fields["login"], &login)
			json.Unmarshal(fields["password"], &value)
			json.Unmarshal(fields["url"], &endpoint)
			json.Unmarshal(fields["uuid"], &uuid)
			json.Unmarshal(fields["groupUuid"], &group)
			if login != "fixture" || value != "updated-fixture-secret" || endpoint != "https://gerrit.wikimedia.org" {
				os.Exit(34)
			}
			if strings.HasPrefix(mode, "create") && (uuid != "" || group != strings.Repeat("e", 32)) {
				os.Exit(35)
			}
			if mode == "update" && uuid != testBrowserUUID {
				os.Exit(36)
			}
			if mode != "create-unverified" && mode != "update-unverified" {
				password = value
			}
			created = true
		default:
			os.Exit(32)
		}
		data, _ = json.Marshal(payload)
		cipher := box.Seal(nil, data, &responseNonce, &peer, priv)
		if mode == "tampered" {
			cipher[0] ^= 1
		}
		if mode == "bad-nonce" {
			responseNonce[0] ^= 1
		}
		send(map[string]any{"action": req.Action, "nonce": base64.StdEncoding.EncodeToString(responseNonce[:]), "message": base64.StdEncoding.EncodeToString(cipher)})
	}
}

func browserFixture(t *testing.T, mode string) browserPairing {
	t.Helper()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	t.Setenv("MWCLI_KEEPASSXC_BROWSER_TEST_HELPER", mode)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return browserPairing{Proxy: executable, ID: "wm-test", Key: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))), DatabaseHash: testBrowserHash}
}

func TestBrowserPairLookupAndRead(t *testing.T) {
	p := browserFixture(t, "success")
	// Both pairing and retrieval use only framed, encrypted protocol messages.
	if err := PairKeePassXCBrowser(context.Background(), p.Proxy); err != nil {
		t.Fatal(err)
	}
	saved, err := loadBrowserPairing()
	if err != nil || saved.ID != "wm-test" {
		t.Fatalf("pairing was not saved: %v", err)
	}
	status, err := KeePassXCBrowserStatus()
	if err != nil || strings.Contains(status, saved.Key) {
		t.Fatal("status exposed pairing key or failed")
	}
	ref, password, err := FindKeePassXCBrowserEntry(context.Background(), "wm/gerrit.password")
	if err != nil || password != "fixture-service-secret" || ref.EntryID != testBrowserUUID {
		t.Fatalf("GUI lookup failed: %v", err)
	}
	value, err := ref.Read()
	if err != nil || value != password {
		t.Fatalf("fresh-session read failed: %v", err)
	}
	if err := PairKeePassXCBrowser(context.Background(), p.Proxy); err == nil {
		t.Fatal("silently replaced pairing")
	}
	info, err := os.Stat(KeePassXCPairingPath())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatal("pairing file is not private")
	}
	if err := ForgetKeePassXCBrowser(); err != nil {
		t.Fatal(err)
	}
	if _, err := ref.Read(); err == nil {
		t.Fatal("read succeeded after forgetting pairing")
	}
}

func TestBrowserFailureNeverReturnsCredential(t *testing.T) {
	for _, mode := range []string{"locked", "revoked", "wrong-database", "wrong-entry", "wrong-login", "expired", "empty", "denied", "multiple", "tampered", "bad-nonce", "oversized", "numeric-entries"} {
		t.Run(mode, func(t *testing.T) {
			p := browserFixture(t, mode)
			if err := saveBrowserPairing(p); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(t.TempDir(), "actions")
			t.Setenv("WM_BROWSER_ACTION_LOG", log)
			value, err := (KeePassXCBrowser{DatabaseHash: testBrowserHash, EntryID: testBrowserUUID, Login: "fixture"}).Read()
			if err == nil || value != "" {
				t.Fatal("failed GUI access returned a credential")
			}
			if strings.Contains(err.Error(), p.Key) || strings.Contains(err.Error(), "sensitive-error-text") || strings.Contains(err.Error(), "fixture-service-secret") {
				t.Fatal("error exposed secret data")
			}
			data, _ := os.ReadFile(log)
			if (mode == "locked" || mode == "revoked" || mode == "wrong-database") && strings.Contains(string(data), "get-logins") {
				t.Fatal("looked up credential before verifying database and pairing")
			}
		})
	}
}

func TestBrowserPairingStorageRejectsUnsafePaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions and symlinks")
	}
	t.Run("directory symlink", func(t *testing.T) {
		p := browserFixture(t, "success")
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Dir(KeePassXCPairingPath())); err != nil {
			t.Fatal(err)
		}
		if err := saveBrowserPairing(p); err == nil {
			t.Fatal("followed pairing directory symlink")
		}
	})
	t.Run("file symlink", func(t *testing.T) {
		p := browserFixture(t, "success")
		if err := pairingDirectory(true); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "unrelated")
		if err := os.WriteFile(target, []byte("unchanged"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, KeePassXCPairingPath()); err != nil {
			t.Fatal(err)
		}
		if err := saveBrowserPairing(p); err == nil {
			t.Fatal("replaced linked pairing")
		}
		if _, err := loadBrowserPairing(); err == nil {
			t.Fatal("read linked pairing")
		}
		if err := ForgetKeePassXCBrowser(); err == nil {
			t.Fatal("removed linked pairing")
		}
		data, _ := os.ReadFile(target)
		if string(data) != "unchanged" {
			t.Fatal("changed symlink target")
		}
	})
	t.Run("public permissions", func(t *testing.T) {
		p := browserFixture(t, "success")
		if err := saveBrowserPairing(p); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(KeePassXCPairingPath(), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadBrowserPairing(); err == nil {
			t.Fatal("accepted publicly readable pairing")
		}
	})
}

func TestBrowserWritesAreVerified(t *testing.T) {
	for _, mode := range []string{"create", "create-other-account", "update", "create-denied", "create-unverified", "update-unverified", "wrong-login", "denied", "locked", "revoked", "wrong-database"} {
		t.Run(mode, func(t *testing.T) {
			p := browserFixture(t, mode)
			if err := saveBrowserPairing(p); err != nil {
				t.Fatal(err)
			}
			var existing *KeePassXCBrowser
			if !strings.HasPrefix(mode, "create") {
				existing = &KeePassXCBrowser{DatabaseHash: testBrowserHash, EntryID: testBrowserUUID, Login: "fixture"}
			}
			ref, err := WriteKeePassXCBrowser(context.Background(), existing, "gerrit.password", "fixture", "https://gerrit.wikimedia.org", "updated-fixture-secret")
			if mode == "create" || mode == "create-other-account" || mode == "update" {
				if err != nil || ref.Login != "fixture" || ref.EntryID != testBrowserUUID {
					t.Fatalf("save failed: %v", err)
				}
			} else {
				if err == nil || ref.EntryID != "" {
					t.Fatal("unverified save returned reference")
				}
				if strings.Contains(err.Error(), "updated-fixture-secret") || strings.Contains(err.Error(), "fixture-service-secret") {
					t.Fatal("save error leaked credential")
				}
			}
		})
	}
}

func TestBrowserNewCredentialDoesNotOverwriteExistingEntry(t *testing.T) {
	p := browserFixture(t, "success")
	if err := saveBrowserPairing(p); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "actions")
	t.Setenv("WM_BROWSER_ACTION_LOG", log)
	if _, err := WriteKeePassXCBrowser(context.Background(), nil, "gerrit.password", "fixture", "https://gerrit.wikimedia.org", "updated-fixture-secret"); err == nil {
		t.Fatal("overwrote unrelated existing entry")
	}
	data, _ := os.ReadFile(log)
	if strings.Contains(string(data), "set-login") {
		t.Fatal("wrote an existing entry without saved reference")
	}
}

func TestBrowserFlatGroupReusesMatchingAccount(t *testing.T) {
	for _, mode := range []string{"multiple-accounts", "duplicate-account", "legacy-recovery"} {
		t.Run(mode, func(t *testing.T) {
			p := browserFixture(t, mode)
			if err := saveBrowserPairing(p); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(t.TempDir(), "actions")
			t.Setenv("WM_BROWSER_ACTION_LOG", log)
			ref, err := WriteKeePassXCBrowser(context.Background(), nil, "gerrit.password", "fixture", "https://gerrit.wikimedia.org", "fixture-service-secret")
			if mode == "duplicate-account" {
				if err == nil || ref.EntryID != "" {
					t.Fatal("accepted ambiguous account")
				}
			} else if err != nil || ref.Login != "fixture" || ref.EntryID != testBrowserUUID {
				t.Fatalf("could not reuse matching entry: %v", err)
			}
			data, _ := os.ReadFile(log)
			if strings.Contains(string(data), "set-login") || strings.Contains(string(data), "create-new-group") {
				t.Fatal("created a duplicate or overwrote an existing entry")
			}
		})
	}
}
