package secrets

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/nacl/box"
)

// KeePassXCBrowser contains only public identifiers. Access keys live separately.
type KeePassXCBrowser struct {
	DatabaseHash string `koanf:"database_hash" json:"database_hash"`
	EntryID      string `koanf:"entry_id" json:"entry_id"`
	Login        string `koanf:"login" json:"login"`
}

type BrowserEntry struct {
	UUID     string `json:"uuid"`
	Login    string `json:"login"`
	Password string `json:"password"`
	Expired  string `json:"expired"`
}

type browserPairing struct {
	Proxy        string `json:"proxy"`
	ID           string `json:"id"`
	Key          string `json:"key"`
	DatabaseHash string `json:"database_hash"`
}

type browserMessage struct {
	Action    string          `json:"action"`
	Success   string          `json:"success"`
	Nonce     string          `json:"nonce"`
	PublicKey string          `json:"publicKey"`
	Message   string          `json:"message"`
	ErrorCode json.RawMessage `json:"errorCode"`
	Hash      string          `json:"hash"`
	ID        string          `json:"id"`
	UUID      string          `json:"uuid"`
	Name      string          `json:"name"`
	Entries   json.RawMessage `json:"entries"`
}

type browserSession struct {
	cmd                     *exec.Cmd
	input                   io.WriteCloser
	output                  io.ReadCloser
	public, private, server *[32]byte
	clientID                string
}

const browserTimeout = 2 * time.Minute

func browserProxy(name string) (string, error) {
	if name == "" {
		name = "keepassxc-proxy"
		if _, err := exec.LookPath(name); err != nil {
			name = "keepassxc-proxy.exe"
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", errors.New("KeePassXC proxy not found; use config secrets keepassxc connect --proxy PATH (WSL can use keepassxc-proxy.exe)")
	}
	return filepath.Abs(path)
}

func openBrowser(ctx context.Context, proxy string) (*browserSession, error) {
	if !filepath.IsAbs(proxy) {
		return nil, errors.New("invalid KeePassXC proxy path; pair again")
	}
	s := &browserSession{}
	var err error
	s.public, s.private, err = box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	var id [24]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, err
	}
	s.clientID = base64.StdEncoding.EncodeToString(id[:])
	s.cmd = exec.CommandContext(ctx, proxy) // #nosec G204 -- absolute proxy selected during explicit pairing, no shell.
	s.input, err = s.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	s.output, err = s.cmd.StdoutPipe()
	if err != nil {
		s.input.Close()
		return nil, err
	}
	// Never forward proxy output or diagnostics, which can contain credentials.
	s.cmd.Stderr = io.Discard
	if err = s.cmd.Start(); err != nil {
		s.input.Close()
		s.output.Close()
		return nil, errors.New("could not start KeePassXC proxy")
	}
	nonce, err := browserNonce()
	if err != nil {
		s.close()
		return nil, err
	}
	reply, err := s.exchange(map[string]string{"action": "change-public-keys", "publicKey": base64.StdEncoding.EncodeToString(s.public[:]), "nonce": base64.StdEncoding.EncodeToString(nonce[:]), "clientID": s.clientID})
	if err != nil {
		s.close()
		return nil, err
	}
	expected := nextBrowserNonce(nonce)
	key, decodeErr := base64.StdEncoding.DecodeString(reply.PublicKey)
	if decodeErr != nil || len(key) != 32 || reply.Success != "true" || reply.Nonce != base64.StdEncoding.EncodeToString(expected[:]) {
		s.close()
		return nil, errors.New("invalid KeePassXC public-key handshake")
	}
	s.server = new([32]byte)
	copy(s.server[:], key)
	return s, nil
}

func (s *browserSession) close() {
	s.input.Close()
	s.output.Close()
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.cmd.Wait()
}

func browserNonce() ([24]byte, error) {
	var nonce [24]byte
	_, err := rand.Read(nonce[:])
	return nonce, err
}

func nextBrowserNonce(nonce [24]byte) [24]byte {
	for i := range nonce {
		nonce[i]++
		if nonce[i] != 0 {
			break
		}
	}
	return nonce
}

func (s *browserSession) exchange(request map[string]string) (browserMessage, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return browserMessage{}, err
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(data)))
	if _, err = io.Copy(s.input, bytes.NewReader(append(header[:], data...))); err != nil {
		return browserMessage{}, errors.New("could not send KeePassXC request")
	}
	for notifications := 0; notifications < 16; notifications++ {
		if _, err = io.ReadFull(s.output, header[:]); err != nil {
			return browserMessage{}, errors.New("KeePassXC did not respond; open its GUI, enable Browser Integration, and approve the request before the timeout")
		}
		size := binary.LittleEndian.Uint32(header[:])
		if size == 0 || size > 1024*1024 {
			return browserMessage{}, errors.New("invalid KeePassXC message size")
		}
		data = make([]byte, size)
		if _, err = io.ReadFull(s.output, data); err != nil {
			return browserMessage{}, errors.New("incomplete KeePassXC response")
		}
		// The proxy may combine a notification and response in one native frame.
		decoder := json.NewDecoder(bytes.NewReader(data))
		for {
			var reply browserMessage
			if err = decoder.Decode(&reply); err == io.EOF {
				break
			} else if err != nil {
				return browserMessage{}, errors.New("invalid KeePassXC message")
			}
			if reply.Action == "database-locked" || reply.Action == "database-unlocked" {
				continue
			}
			if reply.Action != request["action"] {
				return browserMessage{}, errors.New("unexpected KeePassXC response")
			}
			if len(reply.ErrorCode) > 0 {
				return browserMessage{}, browserError(reply.ErrorCode)
			}
			return reply, nil
		}
	}
	return browserMessage{}, errors.New("too many KeePassXC notifications")
}

var errBrowserEntryMissing = errors.New("no KeePassXC entry found")

func browserError(raw json.RawMessage) error {
	switch strings.Trim(string(raw), "\"") {
	case "1":
		return errors.New("KeePassXC database is locked or closed; unlock the paired database in KeePassXC and retry")
	case "6":
		return errors.New("KeePassXC access was cancelled or denied")
	case "8":
		return errors.New("KeePassXC pairing was revoked or does not match; run config secrets keepassxc disconnect, then config secrets keepassxc connect")
	case "15":
		return fmt.Errorf("%w; check its path and access permissions", errBrowserEntryMissing)
	case "19":
		return errors.New("KeePassXC access to entries was denied")
	default:
		return errors.New("KeePassXC refused the request; check its GUI and pairing")
	}
}

func (s *browserSession) request(action string, fields map[string]any) (browserMessage, error) {
	nonce, err := browserNonce()
	if err != nil {
		return browserMessage{}, err
	}
	fields["action"] = action
	data, err := json.Marshal(fields)
	if err != nil {
		return browserMessage{}, err
	}
	cipher := box.Seal(nil, data, &nonce, s.server, s.private)
	reply, err := s.exchange(map[string]string{"action": action, "message": base64.StdEncoding.EncodeToString(cipher), "nonce": base64.StdEncoding.EncodeToString(nonce[:]), "clientID": s.clientID, "triggerUnlock": "false"})
	if err != nil {
		return browserMessage{}, err
	}
	expected := nextBrowserNonce(nonce)
	if reply.Nonce != base64.StdEncoding.EncodeToString(expected[:]) {
		return browserMessage{}, errors.New("KeePassXC response nonce did not match")
	}
	cipher, err = base64.StdEncoding.DecodeString(reply.Message)
	if err != nil {
		return browserMessage{}, errors.New("invalid KeePassXC encrypted response")
	}
	data, ok := box.Open(nil, cipher, &expected, s.server, s.private)
	if !ok {
		return browserMessage{}, errors.New("KeePassXC response authentication failed")
	}
	var decoded browserMessage
	if json.Unmarshal(data, &decoded) != nil || decoded.Success != "true" || decoded.Nonce != reply.Nonce {
		return browserMessage{}, fmt.Errorf("invalid authenticated KeePassXC %s response", action)
	}
	return decoded, nil
}

func validBrowserID(value string, length int) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == length
}

// PairKeePassXCBrowser is the only operation that requests a new GUI association.
// It never reads a database file or requests its master password.
func PairKeePassXCBrowser(ctx context.Context, proxy string) error {
	if err := checkNewBrowserPairing(); err != nil {
		return err
	}
	proxy, err := browserProxy(proxy)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	s, err := openBrowser(ctx, proxy)
	if err != nil {
		return err
	}
	defer s.close()
	database, err := s.request("get-databasehash", map[string]any{})
	if err != nil {
		return err
	}
	if !validBrowserID(database.Hash, 32) {
		return errors.New("invalid KeePassXC database identifier")
	}
	identity, _, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	key := base64.StdEncoding.EncodeToString(identity[:])
	reply, err := s.request("associate", map[string]any{"key": base64.StdEncoding.EncodeToString(s.public[:]), "idKey": key})
	if err != nil {
		return err
	}
	if reply.ID == "" || reply.Hash != database.Hash {
		return errors.New("KeePassXC database changed during pairing")
	}
	if err := saveBrowserPairing(browserPairing{Proxy: proxy, ID: reply.ID, Key: key, DatabaseHash: reply.Hash}); err != nil {
		return fmt.Errorf("GUI pairing was approved but could not be saved; remove that connection in KeePassXC before retrying: %w", err)
	}
	return nil
}

func pairedBrowser(ctx context.Context, expectedHash string) (*browserSession, browserPairing, error) {
	p, err := loadBrowserPairing()
	if err != nil {
		return nil, p, err
	}
	if expectedHash != "" && p.DatabaseHash != expectedHash {
		return nil, p, errors.New("saved pairing belongs to a different KeePassXC database")
	}
	s, err := openBrowser(ctx, p.Proxy)
	if err != nil {
		return nil, p, err
	}
	database, err := s.request("get-databasehash", map[string]any{})
	if err != nil {
		s.close()
		return nil, p, err
	}
	if database.Hash != p.DatabaseHash {
		s.close()
		return nil, p, errors.New("wrong KeePassXC database is active; select the paired database and retry")
	}
	reply, err := s.request("test-associate", map[string]any{"id": p.ID, "key": p.Key})
	if err != nil {
		s.close()
		return nil, p, err
	}
	if reply.ID != p.ID || reply.Hash != p.DatabaseHash {
		s.close()
		return nil, p, errors.New("KeePassXC pairing identity did not match")
	}
	return s, p, nil
}

func browserLookup(s *browserSession, p browserPairing, url string) (BrowserEntry, error) {
	return browserLookupAccount(s, p, url, "")
}

// Multiple accounts may share a hostname and title in the flat wm group.
// Select only an exact account match, and refuse ambiguous or denied results.
func browserLookupAccount(s *browserSession, p browserPairing, url, account string) (BrowserEntry, error) {
	reply, err := s.request("get-logins", map[string]any{"id": p.ID, "url": url, "keys": []map[string]string{{"id": p.ID, "key": p.Key}}})
	if err != nil {
		return BrowserEntry{}, err
	}
	if reply.Hash != p.DatabaseHash {
		return BrowserEntry{}, errors.New("KeePassXC database changed during credential lookup")
	}
	// KeePassXC's set-login acknowledgement may encode "entries" as a
	// numeric placeholder. Only get-logins has an entry-list payload.
	var entries []BrowserEntry
	if err := json.Unmarshal(reply.Entries, &entries); err != nil {
		return BrowserEntry{}, errors.New("invalid KeePassXC entry-list response")
	}
	if len(entries) == 0 {
		return BrowserEntry{}, errors.New("KeePassXC did not permit access to an entry; check its GUI")
	}
	if account != "" {
		matching := make([]BrowserEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.Login == account {
				matching = append(matching, entry)
			}
		}
		entries = matching
		if len(entries) == 0 {
			return BrowserEntry{}, errBrowserEntryMissing
		}
	}
	if len(entries) != 1 {
		return BrowserEntry{}, errors.New("KeePassXC did not return exactly one permitted entry; check its GUI and entry path")
	}
	entry := entries[0]
	if !validBrowserID(entry.UUID, 16) || entry.Password == "" || entry.Expired == "true" {
		return BrowserEntry{}, errors.New("KeePassXC returned an invalid, empty or expired credential")
	}
	return entry, nil
}

// FindKeePassXCBrowserEntry selects an entry once; subsequent reads pin its UUID.
func FindKeePassXCBrowserEntry(ctx context.Context, path string) (KeePassXCBrowser, string, error) {
	path = strings.TrimPrefix(path, "/")
	if !validEntry(path) || len("keepassxc://by-path/"+path) > 256 {
		return KeePassXCBrowser{}, "", errors.New("provide a full KeePassXC entry path of at most 237 bytes")
	}
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	s, p, err := pairedBrowser(ctx, "")
	if err != nil {
		return KeePassXCBrowser{}, "", err
	}
	defer s.close()
	entry, err := browserLookup(s, p, "keepassxc://by-path/"+path)
	if err != nil {
		return KeePassXCBrowser{}, "", err
	}
	return KeePassXCBrowser{DatabaseHash: p.DatabaseHash, EntryID: entry.UUID, Login: entry.Login}, entry.Password, nil
}

func (k KeePassXCBrowser) Read() (string, error) {
	if !validBrowserID(k.DatabaseHash, 32) || !validBrowserID(k.EntryID, 16) {
		return "", errors.New("invalid KeePassXC GUI entry reference")
	}
	ctx, cancel := context.WithTimeout(context.Background(), browserTimeout)
	defer cancel()
	s, p, err := pairedBrowser(ctx, k.DatabaseHash)
	if err != nil {
		return "", err
	}
	defer s.close()
	entry, err := browserLookup(s, p, "keepassxc://by-uuid/"+k.EntryID)
	if err != nil {
		return "", err
	}
	if entry.UUID != k.EntryID || entry.Login != k.Login {
		return "", errors.New("KeePassXC entry identity changed; link the credential again")
	}
	return entry.Password, nil
}

func validEntry(entry string) bool {
	if entry == "" || strings.ContainsAny(entry, "\r\n") {
		return false
	}
	for _, part := range strings.Split(entry, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// WriteKeePassXCBrowser saves through the GUI protocol and verifies the exact
// account and value before allowing configuration plaintext to be removed.
func WriteKeePassXCBrowser(ctx context.Context, existing *KeePassXCBrowser, key, account, endpoint, value string) (KeePassXCBrowser, error) {
	if value == "" {
		return KeePassXCBrowser{}, errors.New("refusing to save an empty KeePassXC credential")
	}
	endpoint, err := Endpoint(endpoint)
	if err != nil {
		return KeePassXCBrowser{}, err
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return KeePassXCBrowser{}, errors.New("invalid credential endpoint")
	}
	group := "wm"
	path := group + "/" + parsed.Hostname()
	legacyPath := group + "/" + key + "/" + parsed.Hostname()
	if !validEntry(legacyPath) || len("keepassxc://by-path/"+legacyPath) > 256 {
		return KeePassXCBrowser{}, errors.New("credential key is too long or invalid for KeePassXC")
	}
	expectedHash := ""
	if existing != nil {
		expectedHash = existing.DatabaseHash
	}
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	s, p, err := pairedBrowser(ctx, expectedHash)
	if err != nil {
		return KeePassXCBrowser{}, err
	}
	defer s.close()
	var entry BrowserEntry
	if existing != nil {
		if !validBrowserID(existing.EntryID, 16) || existing.Login != account {
			return KeePassXCBrowser{}, errors.New("invalid saved KeePassXC identity")
		}
		entry, err = browserLookup(s, p, "keepassxc://by-uuid/"+existing.EntryID)
		if err != nil {
			return KeePassXCBrowser{}, err
		}
		if entry.UUID != existing.EntryID || entry.Login != account {
			return KeePassXCBrowser{}, errors.New("KeePassXC entry identity changed")
		}
		if entry.Password != value {
			if _, err = s.request("set-login", map[string]any{"id": p.ID, "uuid": entry.UUID, "url": endpoint, "login": account, "password": value}); err != nil {
				return KeePassXCBrowser{}, err
			}
		}
	} else {
		entry, err = browserLookupAccount(s, p, "keepassxc://by-path/"+path, account)
		// Recover an interrupted migration made with the previous nested layout.
		// Existing UUID links continue to use their original entry in any location.
		if errors.Is(err, errBrowserEntryMissing) {
			legacy, legacyErr := browserLookupAccount(s, p, "keepassxc://by-path/"+legacyPath, account)
			if legacyErr == nil {
				entry, err = legacy, nil
			} else if !errors.Is(legacyErr, errBrowserEntryMissing) {
				return KeePassXCBrowser{}, legacyErr
			}
		}
		if err == nil {
			if entry.Login != account || entry.Password != value {
				return KeePassXCBrowser{}, errors.New("existing KeePassXC migration entry differs; nothing was overwritten")
			}
		} else {
			if !errors.Is(err, errBrowserEntryMissing) {
				return KeePassXCBrowser{}, err
			}
			groupReply, err := s.request("create-new-group", map[string]any{"groupName": group})
			if err != nil {
				return KeePassXCBrowser{}, err
			}
			if !validBrowserID(groupReply.UUID, 16) {
				return KeePassXCBrowser{}, errors.New("could not verify the KeePassXC destination group")
			}
			current, err := s.request("get-databasehash", map[string]any{})
			if err != nil {
				return KeePassXCBrowser{}, err
			}
			if current.Hash != p.DatabaseHash {
				return KeePassXCBrowser{}, errors.New("KeePassXC database changed before saving")
			}
			if _, err = s.request("set-login", map[string]any{"id": p.ID, "url": endpoint, "login": account, "password": value, "group": group, "groupUuid": groupReply.UUID, "downloadFavicon": "false"}); err != nil {
				return KeePassXCBrowser{}, err
			}
			entry, err = browserLookupAccount(s, p, "keepassxc://by-path/"+path, account)
			if err != nil {
				return KeePassXCBrowser{}, errors.New("KeePassXC entry was not verified; original configuration retained")
			}
		}
	}
	verified, err := browserLookup(s, p, "keepassxc://by-uuid/"+entry.UUID)
	if err != nil || verified.UUID != entry.UUID || verified.Login != account || verified.Password != value {
		return KeePassXCBrowser{}, errors.New("KeePassXC saved credential did not verify; original configuration retained")
	}
	return KeePassXCBrowser{DatabaseHash: p.DatabaseHash, EntryID: verified.UUID, Login: verified.Login}, nil
}
