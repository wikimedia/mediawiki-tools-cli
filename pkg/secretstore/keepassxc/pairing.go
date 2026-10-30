package keepassxc

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

func (c *Client) pairingDirectory(create bool) error {
	if !filepath.IsAbs(c.PairingFile) {
		return errors.New("KeePassXC pairing path must be absolute")
	}
	dir := filepath.Dir(c.PairingFile)
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

func (c *Client) checkNewBrowserPairing() error {
	_, err := os.Lstat(c.PairingFile)
	if err == nil {
		return errors.New("a KeePassXC GUI pairing already exists; inspect or disconnect the existing pairing first")
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := c.pairingDirectory(false); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (c *Client) saveBrowserPairing(p browserPairing) error {
	if err := c.pairingDirectory(true); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return errors.New("could not encode KeePassXC pairing")
	}
	f, err := os.CreateTemp(filepath.Dir(c.PairingFile), ".pairing-*")
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
	return os.Link(f.Name(), c.PairingFile)
}

func (c *Client) loadBrowserPairing() (browserPairing, error) {
	var p browserPairing
	if err := c.pairingDirectory(false); err != nil {
		if os.IsNotExist(err) {
			return p, errors.New("no KeePassXC GUI pairing; pair with KeePassXC before accessing credentials")
		}
		return p, err
	}
	info, err := os.Lstat(c.PairingFile)
	if os.IsNotExist(err) {
		return p, errors.New("no KeePassXC GUI pairing; pair with KeePassXC before accessing credentials")
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
	f, err := os.Open(c.PairingFile)
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
func (c *Client) Status() (string, error) {
	p, err := c.loadBrowserPairing()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Pairing file: %s\nProxy: %s\nDatabase ID: %s\n", c.PairingFile, p.Proxy, p.DatabaseHash), nil
}

// ForgetKeePassXCBrowser does not revoke the connection inside the database.
func (c *Client) Disconnect() error {
	if err := c.pairingDirectory(false); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	info, err := os.Lstat(c.PairingFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("refusing to remove a linked KeePassXC pairing file")
	}
	return os.Remove(c.PairingFile)
}
