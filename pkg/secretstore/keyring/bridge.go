package keyring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	oskeyring "github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

const WindowsProvider = "windows-credential-manager"

const (
	bridgeLimit       = 16 * 1024
	bridgeOutputLimit = 1024 * 1024
)

// BridgeClient calls an explicitly selected native helper. Secrets travel only
// through anonymous pipes; the helper must implement ServeBridge's protocol.
type BridgeClient struct {
	Executable   string
	Args         []string
	ProbeService string
}

type bridgeRequest struct {
	Version int    `json:"version"`
	Action  string `json:"action"`
	Service string `json:"service,omitempty"`
	Account string `json:"account,omitempty"`
	Value   []byte `json:"value,omitempty"`
}

// CredentialMetadata contains public identifiers only, never credential blobs.
type CredentialMetadata struct {
	Service string `json:"service"`
	Account string `json:"account"`
	Target  string `json:"target"`
}

type MetadataLister interface {
	ListMetadata() ([]CredentialMetadata, error)
}

type bridgeResponse struct {
	Version int                  `json:"version"`
	Value   []byte               `json:"value,omitempty"`
	Error   string               `json:"error,omitempty"`
	Entries []CredentialMetadata `json:"entries,omitempty"`
}

func (c *BridgeClient) Get(service, account string) (string, error) {
	return c.call(context.Background(), bridgeRequest{Action: "get", Service: service, Account: account})
}

func (c *BridgeClient) Set(service, account, value string) error {
	_, err := c.call(context.Background(), bridgeRequest{Action: "set", Service: service, Account: account, Value: []byte(value)})
	return err
}

func (c *BridgeClient) Delete(service, account string) error {
	_, err := c.call(context.Background(), bridgeRequest{Action: "delete", Service: service, Account: account})
	return err
}

func (c *BridgeClient) Probe(ctx context.Context) error {
	_, err := c.call(ctx, bridgeRequest{Action: "probe", Service: c.ProbeService, Account: "availability"})
	return err
}

// boundedOutput prevents a misbehaving helper from returning unbounded data.
type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > bridgeOutputLimit {
		return 0, secretstore.ErrTooLarge
	}
	return b.Buffer.Write(p)
}

func (c *BridgeClient) call(ctx context.Context, request bridgeRequest) (string, error) {
	reply, err := c.invoke(ctx, request)
	return string(reply.Value), err
}

func (c *BridgeClient) ListMetadata() ([]CredentialMetadata, error) {
	reply, err := c.invoke(context.Background(), bridgeRequest{Action: "list"})
	return reply.Entries, err
}

func (c *BridgeClient) invoke(ctx context.Context, request bridgeRequest) (bridgeResponse, error) {
	if c.Executable == "" {
		return bridgeResponse{}, secretstore.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request.Version = 1
	input, err := json.Marshal(request)
	if err != nil || len(input) > bridgeLimit {
		return bridgeResponse{}, secretstore.ErrTooLarge
	}
	cmd := exec.CommandContext(ctx, c.Executable, c.Args...) // #nosec G204 -- explicitly selected helper; secret data is stdin, never arguments.
	cmd.Stdin = bytes.NewReader(input)
	var output boundedOutput
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return bridgeResponse{}, ctx.Err()
		}
		return bridgeResponse{}, secretstore.ErrUnavailable
	}
	var reply bridgeResponse
	if json.Unmarshal(output.Bytes(), &reply) != nil || reply.Version != 1 {
		return bridgeResponse{}, secretstore.ErrUnavailable
	}
	switch reply.Error {
	case "":
		return reply, nil
	case "not-found":
		return bridgeResponse{}, oskeyring.ErrNotFound
	case "too-large":
		return bridgeResponse{}, secretstore.ErrTooLarge
	case "invalid":
		return bridgeResponse{}, secretstore.ErrInvalid
	default:
		return bridgeResponse{}, secretstore.ErrUnavailable
	}
}

// ServeBridge handles one bounded request, returning sanitized errors only.
// allowService restricts the helper to application-owned namespaces.
func ServeBridge(input io.Reader, output io.Writer, client Client, allowService func(string) bool) error {
	reply := bridgeResponse{Version: 1}
	data, err := io.ReadAll(io.LimitReader(input, bridgeLimit+1))
	var request bridgeRequest
	if err != nil || len(data) > bridgeLimit || json.Unmarshal(data, &request) != nil || request.Version != 1 || client == nil {
		reply.Error = "invalid"
	} else if request.Action == "list" {
		if lister, ok := client.(MetadataLister); ok && allowService != nil {
			var entries []CredentialMetadata
			entries, err = lister.ListMetadata()
			for _, entry := range entries {
				if allowService(entry.Service) && entry.Account != "" && !strings.ContainsAny(entry.Service+entry.Account, "\x00\r\n") && entry.Target == entry.Service+":"+entry.Account {
					reply.Entries = append(reply.Entries, entry)
				}
			}
		} else {
			reply.Error = "unavailable"
		}
	} else if request.Action == "probe" {
		if allowService == nil || !allowService(request.Service) {
			reply.Error = "invalid"
		} else {
			_, err = client.Get(request.Service, "availability")
		}
	} else if allowService == nil || !allowService(request.Service) || request.Account == "" || strings.ContainsAny(request.Service+request.Account, "\x00\r\n") {
		reply.Error = "invalid"
	} else {
		switch request.Action {
		case "get":
			var value string
			value, err = client.Get(request.Service, request.Account)
			reply.Value = []byte(value)
		case "set":
			if len(request.Value) > 2560 {
				err = oskeyring.ErrSetDataTooBig
			} else {
				err = client.Set(request.Service, request.Account, string(request.Value))
			}
		case "delete":
			err = client.Delete(request.Service, request.Account)
		default:
			reply.Error = "invalid"
		}
	}
	if err != nil {
		reply.Value = nil
		reply.Entries = nil
		switch {
		case errors.Is(err, oskeyring.ErrNotFound):
			if request.Action != "probe" {
				reply.Error = "not-found"
			}
		case errors.Is(err, oskeyring.ErrSetDataTooBig), errors.Is(err, secretstore.ErrTooLarge):
			reply.Error = "too-large"
		default:
			reply.Error = "unavailable"
		}
	}
	return json.NewEncoder(output).Encode(reply)
}
