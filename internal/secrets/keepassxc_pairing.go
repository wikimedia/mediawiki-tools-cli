package secrets

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

// KeePassXCPairingPath is intentionally separate from ordinary config and output.
// The protocol's identification key acts as a bearer credential even though the
// protocol calls it a public key. Never include it in config show/get or logs.
func KeePassXCPairingPath() string {
	return filepath.Join(cli.UserDirectoryPath(), "secrets", "keepassxc-pairing.json")
}

func pairingDirectory(create bool) error {
	dir := filepath.Dir(KeePassXCPairingPath())
	if create {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("KeePassXC pairing directory must not be a symlink")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return errors.New("KeePassXC pairing directory is accessible to other users; set its permissions to 0700")
	}
	return nil
}

func checkNewBrowserPairing() error {
	_, err := os.Lstat(KeePassXCPairingPath())
	if err == nil {
		return errors.New("a KeePassXC GUI pairing already exists; use config secrets keepassxc status or disconnect first")
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := pairingDirectory(false); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func saveBrowserPairing(p browserPairing) error {
	if err := pairingDirectory(true); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return errors.New("could not encode KeePassXC pairing")
	}
	f, err := os.CreateTemp(filepath.Dir(KeePassXCPairingPath()), ".pairing-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Linking is atomic and refuses to overwrite a concurrent pairing or symlink.
	return os.Link(f.Name(), KeePassXCPairingPath())
}

func loadBrowserPairing() (browserPairing, error) {
	var p browserPairing
	if err := pairingDirectory(false); err != nil {
		if os.IsNotExist(err) {
			return p, errors.New("no KeePassXC GUI pairing; run config secrets keepassxc connect")
		}
		return p, err
	}
	info, err := os.Lstat(KeePassXCPairingPath())
	if os.IsNotExist(err) {
		return p, errors.New("no KeePassXC GUI pairing; run config secrets keepassxc connect")
	}
	if err != nil {
		return p, err
	}
	if !info.Mode().IsRegular() {
		return p, errors.New("KeePassXC pairing must be a regular file, not a symlink")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return p, errors.New("KeePassXC pairing file is accessible to other users; set its permissions to 0600")
	}
	f, err := os.Open(KeePassXCPairingPath())
	if err != nil {
		return p, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil || len(data) > 64*1024 {
		return p, errors.New("could not read KeePassXC pairing")
	}
	if json.Unmarshal(data, &p) != nil {
		return p, errors.New("invalid KeePassXC pairing file")
	}
	key, err := base64.StdEncoding.DecodeString(p.Key)
	if err != nil || len(key) != 32 || p.ID == "" || !validBrowserID(p.DatabaseHash, 32) || !filepath.IsAbs(p.Proxy) {
		return browserPairing{}, errors.New("invalid KeePassXC pairing; disconnect and pair again")
	}
	return p, nil
}

// KeePassXCBrowserStatus describes the connection without loading a credential
// from the GUI or exposing the pairing key. It does not verify GUI availability.
func KeePassXCBrowserStatus() (string, error) {
	p, err := loadBrowserPairing()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Pairing file: %s\nProxy: %s\nDatabase ID: %s\n", KeePassXCPairingPath(), p.Proxy, p.DatabaseHash), nil
}

// ForgetKeePassXCBrowser does not revoke the connection inside the database.
func ForgetKeePassXCBrowser() error {
	if err := pairingDirectory(false); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	info, err := os.Lstat(KeePassXCPairingPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("refusing to remove a linked KeePassXC pairing file")
	}
	return os.Remove(KeePassXCPairingPath())
}
