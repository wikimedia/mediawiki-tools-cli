package keepassxc

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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/nacl/box"
)

// Reference contains only public identifiers. Access keys live separately.
type Reference struct {
	DatabaseHash string `koanf:"database_hash" json:"database_hash"`
	EntryID      string `koanf:"entry_id" json:"entry_id"`
	Login        string `koanf:"login" json:"login"`
}

// DeleteReference asks KeePassXC to recycle an explicitly selected entry. Its
// GUI supplies the final approval; a denial or unsupported action is an error.
func (c *Client) DeleteReference(ctx context.Context, ref Reference) error {
	if !validBrowserID(ref.DatabaseHash, 32) || !validBrowserID(ref.EntryID, 16) {
		return errors.New("invalid KeePassXC deletion reference")
	}
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	s, p, err := c.pairedBrowser(ctx, ref.DatabaseHash)
	if err != nil {
		return err
	}
	defer s.close()
	entries, err := browserEntries(s, p, "keepassxc://by-uuid/"+ref.EntryID)
	if err != nil {
		return err
	}
	if len(entries) != 1 || entries[0].UUID != ref.EntryID || (ref.Login != "" && entries[0].Login != ref.Login) {
		return errors.New("KeePassXC entry identity changed; deletion refused")
	}
	_, err = s.request("delete-entry", map[string]any{"id": p.ID, "uuid": ref.EntryID})
	return err
}

// EntryMetadata is the browser protocol's metadata-only database listing. It
// has no password field. Enumeration requires an explicit KeePassXC setting.
type EntryMetadata struct {
	UUID  string `json:"uuid"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func (c *Client) ListEntryMetadata(ctx context.Context) (string, []EntryMetadata, error) {
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	s, p, err := c.pairedBrowser(ctx, "")
	if err != nil {
		return "", nil, err
	}
	defer s.close()
	reply, err := s.request("get-database-entries", map[string]any{"id": p.ID})
	if err != nil {
		return "", nil, fmt.Errorf("KeePassXC entry metadata access is unavailable: %w. In KeePassXC Settings > Browser Integration > Advanced, enable \"Allow limited access to all entries in connected database\" (wording may vary by version). This allows integrations to list titles, URLs and UUIDs for all entries in connected databases; this metadata listing does not include passwords. You can enable it temporarily, rerun discovery and cleanup, then turn it off again when finished. If the setting is absent, your KeePassXC version may not support metadata scans", err)
	}
	var entries []EntryMetadata
	if json.Unmarshal(reply.Entries, &entries) != nil {
		return "", nil, errors.New("invalid KeePassXC metadata response")
	}
	current, err := s.request("get-databasehash", map[string]any{})
	if err != nil || current.Hash != p.DatabaseHash {
		return "", nil, errors.New("KeePassXC database changed during metadata enumeration")
	}
	for _, entry := range entries {
		if !validBrowserID(entry.UUID, 16) {
			return "", nil, errors.New("invalid KeePassXC metadata entry identifier")
		}
	}
	return p.DatabaseHash, entries, nil
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
	Action        string          `json:"action"`
	Success       string          `json:"success"`
	Nonce         string          `json:"nonce"`
	PublicKey     string          `json:"publicKey"`
	Message       string          `json:"message"`
	ErrorCode     json.RawMessage `json:"errorCode"`
	Hash          string          `json:"hash"`
	ID            string          `json:"id"`
	UUID          string          `json:"uuid"`
	Name          string          `json:"name"`
	Entries       json.RawMessage `json:"entries"`
	TriggerUnlock string          `json:"triggerUnlock"`
}

type browserSession struct {
	ctx                     context.Context
	cmd                     *exec.Cmd
	input                   io.WriteCloser
	output                  io.ReadCloser
	public, private, server *[32]byte
	clientID                string
	stopIO                  func() bool
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
		return "", errors.New("KeePassXC proxy not found; configure its absolute path (WSL can use keepassxc-proxy.exe)")
	}
	return filepath.Abs(path)
}

// Only connection setup is retried. No credential reads or writes have been
// issued yet, so reconnecting cannot duplicate a save or an approval request.
func openBrowser(ctx context.Context, proxy string) (*browserSession, error) {
	launched := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("KeePassXC did not become available; open KeePassXC, enable Browser Integration and unlock the paired database: %w", err)
		}
		session, err := openBrowserOnce(ctx, proxy)
		if !errors.Is(err, errBrowserUnavailable) {
			return session, err
		}
		if !launched {
			launchBrowserGUI(proxy)
			launched = true
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

// Use only the known GUI executable next to the explicitly paired proxy.
// In WSL this also handles a Windows installation reached through /mnt/c/.
func launchBrowserGUI(proxy string) {
	name := filepath.Base(proxy)
	gui := "keepassxc"
	switch strings.ToLower(name) {
	case "keepassxc-proxy":
	case "keepassxc-proxy.exe":
		gui += ".exe"
	default:
		return
	}
	path := filepath.Join(filepath.Dir(proxy), gui)
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return
	}
	cmd := exec.Command(path) // #nosec G204 -- fixed sibling GUI of the explicitly paired proxy, no shell or secret arguments.
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if cmd.Start() == nil {
		go func() { _ = cmd.Wait() }()
	}
}

func openBrowserOnce(ctx context.Context, proxy string) (*browserSession, error) {
	if !filepath.IsAbs(proxy) {
		return nil, errors.New("invalid KeePassXC proxy path; pair again")
	}
	s := &browserSession{ctx: ctx}
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
		return nil, errors.New("could not start KeePassXC proxy; check its configured executable path")
	}
	// A proxy started while the GUI was closed can remain disconnected even
	// after the GUI opens. Bound that handshake and create a fresh proxy.
	interrupt := func() {
		_ = s.input.Close()
		_ = s.output.Close()
		_ = s.cmd.Process.Kill()
	}
	s.stopIO = context.AfterFunc(ctx, interrupt)
	handshakeTimer := time.AfterFunc(3*time.Second, interrupt)
	defer handshakeTimer.Stop()

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
	if s.stopIO != nil {
		s.stopIO()
	}
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
		return browserMessage{}, fmt.Errorf("%w: could not send request", errBrowserUnavailable)
	}
	for notifications := 0; notifications < 16; notifications++ {
		if _, err = io.ReadFull(s.output, header[:]); err != nil {
			return browserMessage{}, fmt.Errorf("%w: proxy did not respond", errBrowserUnavailable)
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

var (
	errBrowserUnavailable  = errors.New("KeePassXC is not available")
	errBrowserEntryMissing = errors.New("no KeePassXC entry found")
	errBrowserLocked       = errors.New("KeePassXC database is locked or closed; unlock the paired database in KeePassXC and retry")
)

func browserError(raw json.RawMessage) error {
	switch strings.Trim(string(raw), "\"") {
	case "1":
		return errBrowserLocked
	case "6":
		return errors.New("KeePassXC access was cancelled or denied")
	case "8":
		return errors.New("KeePassXC pairing was revoked or does not match; disconnect and pair again")
	case "15":
		return fmt.Errorf("%w; check its path and access permissions", errBrowserEntryMissing)
	case "19":
		return errors.New("KeePassXC access to entries was denied")
	default:
		return errors.New("KeePassXC refused the request; check its GUI and pairing")
	}
}

func (s *browserSession) request(action string, fields map[string]any) (browserMessage, error) {
	for {
		reply, err := s.requestOnce(action, fields)
		if !errors.Is(err, errBrowserLocked) || action != "get-databasehash" {
			return reply, err
		}
		// KeePassXC raises its unlock dialog but returns a locked response
		// immediately. Wait during database verification, so identity and
		// pairing are checked after unlocking. Never replay a credential write.
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return browserMessage{}, fmt.Errorf("%w; timed out or cancelled waiting for unlock", errBrowserLocked)
		case <-timer.C:
		}
	}
}

func (s *browserSession) requestOnce(action string, fields map[string]any) (browserMessage, error) {
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
	reply, err := s.exchange(map[string]string{"action": action, "message": base64.StdEncoding.EncodeToString(cipher), "nonce": base64.StdEncoding.EncodeToString(nonce[:]), "clientID": s.clientID, "triggerUnlock": "true"})
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

// PairReference is the only operation that requests a new GUI association.
// It never reads a database file or requests its master password.
func (c *Client) Pair(ctx context.Context, proxy string) error {
	if err := c.checkNewBrowserPairing(); err != nil {
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
	if err := c.saveBrowserPairing(browserPairing{Proxy: proxy, ID: reply.ID, Key: key, DatabaseHash: reply.Hash}); err != nil {
		return fmt.Errorf("GUI pairing was approved but could not be saved; remove that connection in KeePassXC before retrying: %w", err)
	}
	return nil
}

func (c *Client) pairedBrowser(ctx context.Context, expectedHash string) (*browserSession, browserPairing, error) {
	p, err := c.loadBrowserPairing()
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

// Multiple accounts may share a hostname and title in an application group.
// Select only an exact account match, and refuse ambiguous or denied results.
func browserEntries(s *browserSession, p browserPairing, url string) ([]BrowserEntry, error) {
	reply, err := s.request("get-logins", map[string]any{"id": p.ID, "url": url, "keys": []map[string]string{{"id": p.ID, "key": p.Key}}})
	if err != nil {
		return nil, err
	}
	if reply.Hash != p.DatabaseHash {
		return nil, errors.New("KeePassXC database changed during credential lookup")
	}
	// KeePassXC's set-login acknowledgement may encode "entries" as a
	// numeric placeholder. Only get-logins has an entry-list payload.
	var entries []BrowserEntry
	if err := json.Unmarshal(reply.Entries, &entries); err != nil {
		return nil, errors.New("invalid KeePassXC entry-list response")
	}
	if len(entries) == 0 {
		return nil, errors.New("KeePassXC did not permit access to an entry; check its GUI")
	}
	return entries, nil
}

func browserLookupAccount(s *browserSession, p browserPairing, url, account string) (BrowserEntry, error) {
	entries, err := browserEntries(s, p, url)
	if err != nil {
		return BrowserEntry{}, err
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

// FindReferenceEntry selects an entry once; subsequent reads pin its UUID.
func (c *Client) Find(ctx context.Context, path string) (Reference, string, error) {
	path = strings.TrimPrefix(path, "/")
	if !validEntry(path) || len("keepassxc://by-path/"+path) > 256 {
		return Reference{}, "", errors.New("provide a full KeePassXC entry path of at most 237 bytes")
	}
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	s, p, err := c.pairedBrowser(ctx, "")
	if err != nil {
		return Reference{}, "", err
	}
	defer s.close()
	entry, err := browserLookup(s, p, "keepassxc://by-path/"+path)
	if err != nil {
		return Reference{}, "", err
	}
	return Reference{DatabaseHash: p.DatabaseHash, EntryID: entry.UUID, Login: entry.Login}, entry.Password, nil
}

func (c *Client) Read(ctx context.Context, k Reference) (string, error) {
	if !validBrowserID(k.DatabaseHash, 32) || !validBrowserID(k.EntryID, 16) {
		return "", errors.New("invalid KeePassXC GUI entry reference")
	}
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	s, p, err := c.pairedBrowser(ctx, k.DatabaseHash)
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

// WriteReference saves through the GUI protocol and verifies the exact
// account and value before allowing configuration plaintext to be removed.
func (c *Client) Write(ctx context.Context, existing *Reference, key, account, endpoint, value string) (Reference, error) {
	if !validEntry(c.Group) {
		return Reference{}, errors.New("invalid KeePassXC application group")
	}
	if value == "" {
		return Reference{}, errors.New("refusing to save an empty KeePassXC credential")
	}
	endpoint, err := normalizeEndpoint(endpoint)
	if err != nil {
		return Reference{}, err
	}
	entryURL := endpoint
	if existing == nil && c.EntryURL != "" {
		entryURL = c.EntryURL
	}
	parsed, err := url.Parse(entryURL)
	if err != nil || parsed.Hostname() == "" || parsed.Scheme == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || len(entryURL) > 256 {
		return Reference{}, errors.New("invalid credential endpoint")
	}
	group := c.Group
	path := group + "/" + parsed.Hostname()
	legacyPath := group + "/" + key + "/" + parsed.Hostname()
	if !validEntry(path) || len("keepassxc://by-path/"+path) > 256 || (c.EntryURL == "" && (!validEntry(legacyPath) || len("keepassxc://by-path/"+legacyPath) > 256)) {
		return Reference{}, errors.New("credential key is too long or invalid for KeePassXC")
	}
	expectedHash := ""
	if existing != nil {
		expectedHash = existing.DatabaseHash
	}
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	s, p, err := c.pairedBrowser(ctx, expectedHash)
	if err != nil {
		return Reference{}, err
	}
	defer s.close()
	var entry BrowserEntry
	if existing != nil {
		if !validBrowserID(existing.EntryID, 16) || existing.Login != account {
			return Reference{}, errors.New("invalid saved KeePassXC identity")
		}
		entry, err = browserLookup(s, p, "keepassxc://by-uuid/"+existing.EntryID)
		if err != nil {
			return Reference{}, err
		}
		if entry.UUID != existing.EntryID || entry.Login != account {
			return Reference{}, errors.New("KeePassXC entry identity changed")
		}
		if entry.Password != value {
			if _, err = s.request("set-login", map[string]any{"id": p.ID, "uuid": entry.UUID, "url": endpoint, "login": account, "password": value}); err != nil {
				return Reference{}, err
			}
		}
	} else {
		entry, err = browserLookupAccount(s, p, "keepassxc://by-path/"+path, account)
		// Recover an interrupted migration made with the previous nested layout.
		// Existing UUID links continue to use their original entry in any location.
		if c.EntryURL == "" && errors.Is(err, errBrowserEntryMissing) {
			legacy, legacyErr := browserLookupAccount(s, p, "keepassxc://by-path/"+legacyPath, account)
			if legacyErr == nil {
				entry, err = legacy, nil
			} else if !errors.Is(legacyErr, errBrowserEntryMissing) {
				return Reference{}, legacyErr
			}
		}
		if err == nil {
			if entry.Login != account || entry.Password != value {
				return Reference{}, errors.New("existing KeePassXC migration entry differs; nothing was overwritten")
			}
		} else {
			if !errors.Is(err, errBrowserEntryMissing) {
				return Reference{}, err
			}
			groupReply, err := s.request("create-new-group", map[string]any{"groupName": group})
			if err != nil {
				return Reference{}, err
			}
			if !validBrowserID(groupReply.UUID, 16) {
				return Reference{}, errors.New("could not verify the KeePassXC destination group")
			}
			current, err := s.request("get-databasehash", map[string]any{})
			if err != nil {
				return Reference{}, err
			}
			if current.Hash != p.DatabaseHash {
				return Reference{}, errors.New("KeePassXC database changed before saving")
			}
			if _, err = s.request("set-login", map[string]any{"id": p.ID, "url": entryURL, "login": account, "password": value, "group": group, "groupUuid": groupReply.UUID, "downloadFavicon": "false"}); err != nil {
				return Reference{}, err
			}
			entry, err = browserLookupAccount(s, p, "keepassxc://by-path/"+path, account)
			if err != nil {
				return Reference{}, errors.New("KeePassXC entry was not verified; original configuration retained")
			}
		}
	}
	verified, err := browserLookup(s, p, "keepassxc://by-uuid/"+entry.UUID)
	if err != nil || verified.UUID != entry.UUID || verified.Login != account || verified.Password != value {
		return Reference{}, errors.New("KeePassXC saved credential did not verify; original configuration retained")
	}
	return Reference{DatabaseHash: p.DatabaseHash, EntryID: verified.UUID, Login: verified.Login}, nil
}
