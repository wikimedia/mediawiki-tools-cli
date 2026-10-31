// Package secrets separates credential storage from ordinary configuration.
package secrets

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	GerritEndpoint      = "https://gerrit.wikimedia.org/r"
	CloudEndpoint       = "https://openstack.eqiad1.wikimediacloud.org:25000/v3"
	PhabricatorEndpoint = "https://phabricator.wikimedia.org"
)

var ErrNotFound = keyring.ErrNotFound

// Store permits other credential providers without coupling service commands to them.
type Store interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
	Delete(service, account string) error
}

type Keyring struct{}

func (Keyring) Get(service, account string) (string, error) { return keyring.Get(service, account) }
func (Keyring) Set(service, account, value string) error    { return keyring.Set(service, account, value) }
func (Keyring) Delete(service, account string) error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// Endpoint binds credentials to an HTTPS destination. HTTP is only allowed for
// loopback development/test services, never arbitrary remote endpoints.
func Endpoint(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("credential endpoint must be an HTTPS URL without user information, query or fragment")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := strings.EqualFold(u.Hostname(), "localhost") || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return "", fmt.Errorf("credential endpoint must use HTTPS (HTTP is only allowed on loopback)")
	}
	u.Host = strings.ToLower(u.Host)
	return strings.TrimRight(u.String(), "/"), nil
}

// ReadInput accepts a single secret from a pipe, including password-manager CLIs.
// Bound input and do not include provider output in errors.
func ReadInput(reader io.Reader) (string, error) {
	const limit = 64 * 1024
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return "", fmt.Errorf("could not read secret from stdin")
	}
	if len(data) > limit {
		return "", fmt.Errorf("secret input exceeds 64 KiB")
	}
	value := strings.TrimRight(string(data), "\r\n")
	if value == "" {
		return "", fmt.Errorf("secret input is empty")
	}
	return value, nil
}
