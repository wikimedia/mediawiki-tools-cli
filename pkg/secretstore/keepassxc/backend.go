package keepassxc

import (
	"context"
	"encoding/base64"
	"strings"
	"unicode/utf8"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

type BrowserClient interface {
	Read(context.Context, Reference) (string, error)
	Write(context.Context, *Reference, string, string, string, string) (Reference, error)
}
type Backend struct{ Client BrowserClient }

func New(client BrowserClient) *Backend { return &Backend{Client: client} }
func (*Backend) Name() string           { return "keepassxc" }

// The browser protocol cannot delete a database entry. Use Store.Unlink to
// forget an application reference without claiming the external entry is gone.
func (*Backend) Capabilities() secretstore.Capabilities {
	return secretstore.Capabilities{MayPrompt: true, VerifiesWrites: true}
}
func (b *Backend) Get(ctx context.Context, reference secretstore.Reference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.Client == nil {
		return nil, secretstore.ErrInvalid
	}
	ref, err := decodeReference(reference)
	if err != nil {
		return nil, err
	}
	value, err := b.Client.Read(ctx, ref)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, safeError("get", err)
	}
	if reference.Fields["encoding"] == "base64-v1" {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, secretstore.ErrInvalid
		}
		return decoded, nil
	}
	if encoding := reference.Fields["encoding"]; encoding != "" && encoding != "text" {
		return nil, secretstore.ErrInvalid
	}
	return []byte(value), nil
}
func (b *Backend) Set(ctx context.Context, request secretstore.WriteRequest, value []byte) (secretstore.Reference, error) {
	if b.Client == nil {
		return secretstore.Reference{}, secretstore.ErrInvalid
	}
	var existing *Reference
	if request.Existing != nil {
		reference, err := decodeReference(*request.Existing)
		if err != nil {
			return secretstore.Reference{}, err
		}
		existing = &reference
	}
	account := request.Metadata["account"]
	if account == "" {
		account = base64.RawURLEncoding.EncodeToString([]byte(request.Namespace)) + "/" + base64.RawURLEncoding.EncodeToString([]byte(request.Key))
	}
	endpoint := request.Metadata["endpoint"]
	if endpoint == "" {
		endpoint = "https://secretstore.invalid"
	}
	encoded := string(value)
	encoding := "text"
	if !utf8.Valid(value) {
		encoded = base64.StdEncoding.EncodeToString(value)
		encoding = "base64-v1"
	}
	reference, err := b.Client.Write(ctx, existing, request.Key, account, endpoint, encoded)
	if ctx.Err() != nil {
		return secretstore.Reference{}, ctx.Err()
	}
	if err != nil {
		return secretstore.Reference{}, safeError("set", err)
	}
	if reference.Login != account {
		return secretstore.Reference{}, secretstore.ErrVerification
	}
	owned := true
	if request.Existing != nil {
		owned = request.Existing.Owned
	}
	return secretstore.Reference{Provider: b.Name(), Owned: owned, Fields: map[string]string{"database_hash": reference.DatabaseHash, "entry_id": reference.EntryID, "login": reference.Login, "encoding": encoding}}, nil
}
func (*Backend) Delete(context.Context, secretstore.Reference) error {
	return secretstore.ErrUnsupported
}
func decodeReference(reference secretstore.Reference) (Reference, error) {
	r := Reference{DatabaseHash: reference.Fields["database_hash"], EntryID: reference.Fields["entry_id"], Login: reference.Fields["login"]}
	if reference.Provider != "keepassxc" || !validBrowserID(r.DatabaseHash, 32) || !validBrowserID(r.EntryID, 16) || r.Login == "" {
		return r, secretstore.ErrInvalid
	}
	return r, nil
}
func safeError(operation string, err error) error {
	if err == nil {
		return nil
	}
	kind := secretstore.ErrUnavailable
	message := err.Error()
	switch {
	case strings.Contains(message, "differs; nothing was overwritten"):
		kind = secretstore.ErrConflict
	case strings.Contains(message, "no KeePassXC GUI pairing"):
		kind = secretstore.ErrInteractionRequired
	case strings.Contains(message, "locked or closed"):
		kind = secretstore.ErrLocked
	case strings.Contains(message, "did not permit access"), strings.Contains(message, "denied"):
		kind = secretstore.ErrDenied
	case strings.Contains(message, "did not verify"), strings.Contains(message, "was not verified"):
		kind = secretstore.ErrVerification
	}
	return secretstore.SafeError(operation, "keepassxc", kind)
}
