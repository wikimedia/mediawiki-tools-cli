// Package keyring adapts native OS credential storage to secretstore.
package keyring

import (
	"context"
	"encoding/base64"
	"errors"
	"runtime"

	oskeyring "github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

type Client interface {
	Get(string, string) (string, error)
	Set(string, string, string) error
	Delete(string, string) error
}
type nativeClient struct{}

func (nativeClient) Get(service, account string) (string, error) {
	return oskeyring.Get(service, account)
}

func (nativeClient) Set(service, account, value string) error {
	return oskeyring.Set(service, account, value)
}

func (nativeClient) Delete(service, account string) error { return oskeyring.Delete(service, account) }

type Backend struct {
	Client        Client
	Provider      string
	MaxValueBytes int
}

func New() *Backend { return &Backend{Client: nativeClient{}} }

func (b *Backend) Name() string {
	if b.Provider != "" {
		return b.Provider
	}
	return "keyring"
}

func (b *Backend) Capabilities() secretstore.Capabilities {
	limit := b.MaxValueBytes
	if runtime.GOOS == "windows" {
		limit = 2560
	}
	return secretstore.Capabilities{CanDelete: true, MaxValueBytes: limit, MayPrompt: true}
}

func (b *Backend) Get(ctx context.Context, reference secretstore.Reference) ([]byte, error) {
	if b.Client == nil {
		return nil, secretstore.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	service, account, err := b.identity(reference)
	if err != nil {
		return nil, err
	}
	// Native keyring libraries may block in a desktop unlock prompt and have no
	// cancellation API. Only the read runs in this goroutine: cancellation must
	// never leave a transformation or credential write running in the background.
	type result struct {
		value string
		err   error
	}
	ready := make(chan result, 1)
	go func() {
		if client, ok := b.Client.(interface {
			GetContext(context.Context, string, string) (string, error)
		}); ok {
			value, err := client.GetContext(ctx, service, account)
			ready <- result{value, err}
			return
		}
		value, err := b.Client.Get(service, account)
		ready <- result{value, err}
	}()
	var value string
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-ready:
		value, err = response.value, response.err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if errors.Is(err, oskeyring.ErrNotFound) {
		err = secretstore.ErrNotFound
	}
	if err != nil {
		return nil, secretstore.SafeError("get", b.Name(), err)
	}
	return []byte(value), nil
}

func (b *Backend) Set(ctx context.Context, request secretstore.WriteRequest, value []byte) (secretstore.Reference, error) {
	if b.Client == nil {
		return secretstore.Reference{}, secretstore.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return secretstore.Reference{}, err
	}
	// Encoding both components prevents delimiter collisions on platforms that
	// flatten service/account into a single credential target name.
	reference := secretstore.Reference{Provider: b.Name(), Owned: true, Fields: map[string]string{"service": "secretstore.v1." + base64.RawURLEncoding.EncodeToString([]byte(request.Namespace)), "account": base64.RawURLEncoding.EncodeToString([]byte(request.Key))}}
	if request.Existing != nil {
		reference = *request.Existing
	}
	service, account, err := b.identity(reference)
	if err != nil {
		return reference, err
	}
	if request.Existing == nil {
		previous, err := b.Client.Get(service, account)
		if err == nil && previous != string(value) {
			return reference, secretstore.ErrConflict
		}
		if err != nil && !errors.Is(err, oskeyring.ErrNotFound) && !errors.Is(err, secretstore.ErrNotFound) {
			return reference, secretstore.SafeError("get", b.Name(), err)
		}
	}
	if err := b.Client.Set(service, account, string(value)); err != nil {
		if errors.Is(err, oskeyring.ErrSetDataTooBig) {
			err = secretstore.ErrTooLarge
		}
		return reference, secretstore.SafeError("set", b.Name(), err)
	}
	return reference, nil
}

func (b *Backend) Delete(ctx context.Context, reference secretstore.Reference) error {
	if b.Client == nil {
		return secretstore.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	service, account, err := b.identity(reference)
	if err != nil {
		return err
	}
	err = b.Client.Delete(service, account)
	if errors.Is(err, oskeyring.ErrNotFound) {
		return secretstore.ErrNotFound
	}
	return secretstore.SafeError("delete", b.Name(), err)
}

func (b *Backend) identity(reference secretstore.Reference) (string, string, error) {
	service, account := reference.Fields["service"], reference.Fields["account"]
	if reference.Provider != b.Name() || service == "" || account == "" {
		return "", "", secretstore.ErrInvalid
	}
	return service, account, nil
}
