// Package keepassxc accesses the running KeePassXC application through its
// encrypted browser protocol. It never handles the database master password.
package keepassxc

import (
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Client has no global app paths. Pairing keys are kept separately from registry
// metadata in a caller-selected private file. Group selects app-owned entries.
type Client struct{ PairingFile, Group string }

func NewClient(pairingFile, group string) (*Client, error) {
	if !filepath.IsAbs(pairingFile) || !validEntry(group) {
		return nil, errors.New("invalid KeePassXC pairing path or group")
	}
	return &Client{PairingFile: pairingFile, Group: group}, nil
}

type Availability struct {
	Available    bool
	Detail       string
	NeedsPairing bool
}

func (c *Client) Detect() Availability {
	if _, err := os.Lstat(c.PairingFile); err == nil {
		if _, err := c.loadBrowserPairing(); err != nil {
			return Availability{Detail: "pairing needs repair; reconnect with KeePassXC"}
		}
		return Availability{Available: true, Detail: "paired; the database must be unlocked"}
	}
	if _, err := browserProxy(""); err != nil {
		return Availability{Detail: "not detected; install KeePassXC or configure its proxy"}
	}
	return Availability{Available: true, Detail: "detected; GUI pairing required", NeedsPairing: true}
}

func normalizeEndpoint(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid credential endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := strings.EqualFold(u.Hostname(), "localhost") || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return "", errors.New("credential endpoint must use HTTPS")
	}
	u.Host = strings.ToLower(u.Host)
	return strings.TrimRight(u.String(), "/"), nil
}
