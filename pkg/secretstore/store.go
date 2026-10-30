// Package secretstore routes application keys to explicit credential backends.
// Registries contain references and public metadata only, never secret values.
package secretstore

import (
	"context"
	"crypto/subtle"
	"errors"
	"sort"
	"strings"
)

var (
	ErrNotFound            = errors.New("secretstore: not found")
	ErrLocked              = errors.New("secretstore: locked")
	ErrDenied              = errors.New("secretstore: access denied")
	ErrUnavailable         = errors.New("secretstore: unavailable")
	ErrUnsupported         = errors.New("secretstore: operation unsupported")
	ErrTooLarge            = errors.New("secretstore: value too large")
	ErrInvalid             = errors.New("secretstore: invalid configuration or value")
	ErrVerification        = errors.New("secretstore: write could not be verified; previous reference retained")
	ErrConflict            = errors.New("secretstore: reference already exists")
	ErrInteractionRequired = errors.New("secretstore: interaction required")
)

// Error deliberately excludes backend diagnostics, which can contain secrets.
type Error struct {
	Operation, Provider string
	Kind                error
}

func (e *Error) Error() string {
	return "secretstore: " + e.Operation + " (" + e.Provider + "): " + e.Kind.Error()
}
func (e *Error) Unwrap() error { return e.Kind }

func SafeError(operation, provider string, err error) error {
	if err == nil {
		return nil
	}
	kind := ErrUnavailable
	for _, candidate := range []error{context.Canceled, context.DeadlineExceeded, ErrNotFound, ErrLocked, ErrDenied, ErrUnsupported, ErrTooLarge, ErrInvalid, ErrVerification, ErrConflict, ErrInteractionRequired} {
		if errors.Is(err, candidate) {
			kind = candidate
			break
		}
	}
	return &Error{Operation: operation, Provider: provider, Kind: kind}
}

type Capabilities struct {
	CanDelete bool
	// VerifiesWrites means Set already reads back and compares the exact value.
	// The store avoids a redundant provider read (and extra GUI prompts).
	VerifiesWrites bool
	// MaxValueBytes is zero when the limit is unknown or not fixed.
	MaxValueBytes int
	MayPrompt     bool
}

// Reference fields must be public identifiers, never pairing keys or tokens.
// Owned=false means Delete unlinks an external item without deleting it.
type Reference struct {
	Provider string            `json:"provider"`
	Fields   map[string]string `json:"fields"`
	Owned    bool              `json:"owned"`
}

type Entry struct {
	Namespace string            `json:"namespace"`
	Key       string            `json:"key"`
	Reference Reference         `json:"reference"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type WriteRequest struct {
	Namespace, Key string
	Metadata       map[string]string
	Existing       *Reference
}

// Backends must not remove the old item before attempting an update. A failed
// verification can still mean a provider changed; callers retain source copies.
// With Existing=nil, refuse differing unregistered items with ErrConflict.
type Backend interface {
	Name() string
	Capabilities() Capabilities
	Get(context.Context, Reference) ([]byte, error)
	Set(context.Context, WriteRequest, []byte) (Reference, error)
	Delete(context.Context, Reference) error
}

type Registry interface {
	Get(context.Context, string, string) (Entry, error)
	Put(context.Context, Entry) error
	Delete(context.Context, string, string) error
	List(context.Context, string) ([]Entry, error)
	// Lock serializes mutations to a key, including provider writes.
	Lock(context.Context, string, string) (func(), error)
}

type InteractionPolicy uint8

const (
	AllowInteraction InteractionPolicy = iota
	NoInteraction
)

type Config struct {
	Namespace, Provider string
	Registry            Registry
	Backends            []Backend
	Interaction         InteractionPolicy
}
type Store struct {
	namespace, provider string
	registry            Registry
	backends            map[string]Backend
	interaction         InteractionPolicy
}

// Open performs no provider discovery, pairing or credential access.
func Open(config Config) (*Store, error) {
	if !valid(config.Namespace) || config.Registry == nil || config.Interaction > NoInteraction {
		return nil, ErrInvalid
	}
	s := &Store{namespace: config.Namespace, provider: config.Provider, registry: config.Registry, backends: map[string]Backend{}, interaction: config.Interaction}
	for _, backend := range config.Backends {
		if backend == nil || !valid(backend.Name()) {
			return nil, ErrInvalid
		}
		if _, exists := s.backends[backend.Name()]; exists {
			return nil, ErrInvalid
		}
		s.backends[backend.Name()] = backend
	}
	if _, exists := s.backends[s.provider]; !exists {
		return nil, ErrUnsupported
	}
	return s, nil
}

func valid(value string) bool { return value != "" && !strings.ContainsAny(value, "\x00\r\n") }

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if !valid(key) {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entry, err := s.registry.Get(ctx, s.namespace, key)
	if err != nil {
		return nil, SafeError("lookup", "registry", err)
	}
	backend, ok := s.backends[entry.Reference.Provider]
	if !ok {
		return nil, ErrUnsupported
	}
	if s.interaction == NoInteraction && backend.Capabilities().MayPrompt {
		return nil, ErrInteractionRequired
	}
	value, err := backend.Get(ctx, cloneReference(entry.Reference))
	if err != nil {
		return nil, SafeError("get", backend.Name(), err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]byte(nil), value...), nil
}

func (s *Store) Set(ctx context.Context, key string, value []byte) error {
	return s.SetWithMetadata(ctx, key, value, nil)
}
func (s *Store) SetWithMetadata(ctx context.Context, key string, value []byte, metadata map[string]string) error {
	if !valid(key) || len(value) == 0 {
		return ErrInvalid
	}
	unlock, err := s.registry.Lock(ctx, s.namespace, key)
	if err != nil {
		return SafeError("lock", "registry", err)
	}
	defer unlock()
	entry, err := s.registry.Get(ctx, s.namespace, key)
	var existing *Reference
	provider := s.provider
	if err == nil {
		provider = entry.Reference.Provider
		reference := cloneReference(entry.Reference)
		existing = &reference
	} else if !errors.Is(err, ErrNotFound) {
		return SafeError("lookup", "registry", err)
	}
	backend, ok := s.backends[provider]
	if !ok {
		return ErrUnsupported
	}
	if s.interaction == NoInteraction && backend.Capabilities().MayPrompt {
		return ErrInteractionRequired
	}
	if maximum := backend.Capabilities().MaxValueBytes; maximum > 0 && len(value) > maximum {
		return ErrTooLarge
	}
	if metadata == nil && existing != nil {
		metadata = entry.Metadata
	}
	reference, err := backend.Set(ctx, WriteRequest{Namespace: s.namespace, Key: key, Metadata: cloneMap(metadata), Existing: existing}, append([]byte(nil), value...))
	if err != nil {
		return SafeError("set", provider, err)
	}
	if reference.Provider != provider {
		return SafeError("set", provider, ErrVerification)
	}
	if !backend.Capabilities().VerifiesWrites {
		verified, err := backend.Get(ctx, cloneReference(reference))
		if err != nil || subtle.ConstantTimeCompare(verified, value) != 1 {
			return SafeError("set", provider, ErrVerification)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return SafeError("save reference", "registry", s.registry.Put(ctx, Entry{Namespace: s.namespace, Key: key, Reference: cloneReference(reference), Metadata: cloneMap(metadata)}))
}

// Link records a caller-selected existing item without reading or modifying it.
// Linking cannot silently replace a previously registered key.
func (s *Store) Link(ctx context.Context, key string, reference Reference, metadata map[string]string) error {
	if !valid(key) {
		return ErrInvalid
	}
	if _, ok := s.backends[reference.Provider]; !ok {
		return ErrUnsupported
	}
	unlock, err := s.registry.Lock(ctx, s.namespace, key)
	if err != nil {
		return SafeError("lock", "registry", err)
	}
	defer unlock()
	if _, err := s.registry.Get(ctx, s.namespace, key); err == nil {
		return ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return SafeError("lookup", "registry", err)
	}
	return SafeError("link", "registry", s.registry.Put(ctx, Entry{Namespace: s.namespace, Key: key, Reference: cloneReference(reference), Metadata: cloneMap(metadata)}))
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if !valid(key) {
		return ErrInvalid
	}
	unlock, err := s.registry.Lock(ctx, s.namespace, key)
	if err != nil {
		return SafeError("lock", "registry", err)
	}
	defer unlock()
	entry, err := s.registry.Get(ctx, s.namespace, key)
	if err != nil {
		return SafeError("lookup", "registry", err)
	}
	if entry.Reference.Owned {
		backend, ok := s.backends[entry.Reference.Provider]
		if !ok || !backend.Capabilities().CanDelete {
			return ErrUnsupported
		}
		if s.interaction == NoInteraction && backend.Capabilities().MayPrompt {
			return ErrInteractionRequired
		}
		if err := backend.Delete(ctx, cloneReference(entry.Reference)); err != nil && !errors.Is(err, ErrNotFound) {
			return SafeError("delete", backend.Name(), err)
		}
	}
	return SafeError("unlink", "registry", s.registry.Delete(ctx, s.namespace, key))
}

// Unlink forgets the app's reference while retaining the external item.
func (s *Store) Unlink(ctx context.Context, key string) error {
	if !valid(key) {
		return ErrInvalid
	}
	unlock, err := s.registry.Lock(ctx, s.namespace, key)
	if err != nil {
		return SafeError("lock", "registry", err)
	}
	defer unlock()
	return SafeError("unlink", "registry", s.registry.Delete(ctx, s.namespace, key))
}

// Transfer verifies the destination before changing the route. It retains the
// source entry; physical deletion is always a separate, explicit operation.
func (s *Store) Transfer(ctx context.Context, key, provider string) error {
	if !valid(key) {
		return ErrInvalid
	}
	destination, ok := s.backends[provider]
	if !ok {
		return ErrUnsupported
	}
	unlock, err := s.registry.Lock(ctx, s.namespace, key)
	if err != nil {
		return SafeError("lock", "registry", err)
	}
	defer unlock()
	entry, err := s.registry.Get(ctx, s.namespace, key)
	if err != nil {
		return SafeError("lookup", "registry", err)
	}
	if entry.Reference.Provider == provider {
		return ErrConflict
	}
	source, ok := s.backends[entry.Reference.Provider]
	if !ok {
		return ErrUnsupported
	}
	if s.interaction == NoInteraction && (source.Capabilities().MayPrompt || destination.Capabilities().MayPrompt) {
		return ErrInteractionRequired
	}
	value, err := source.Get(ctx, cloneReference(entry.Reference))
	if err != nil {
		return SafeError("get", source.Name(), err)
	}
	if len(value) == 0 {
		return ErrInvalid
	}
	if limit := destination.Capabilities().MaxValueBytes; limit > 0 && len(value) > limit {
		return ErrTooLarge
	}
	reference, err := destination.Set(ctx, WriteRequest{Namespace: s.namespace, Key: key, Metadata: cloneMap(entry.Metadata)}, append([]byte(nil), value...))
	if err != nil {
		return SafeError("transfer", provider, err)
	}
	if reference.Provider != provider {
		return ErrVerification
	}
	if !destination.Capabilities().VerifiesWrites {
		verified, err := destination.Get(ctx, cloneReference(reference))
		if err != nil || subtle.ConstantTimeCompare(value, verified) != 1 {
			return ErrVerification
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	entry.Reference = cloneReference(reference)
	return SafeError("save reference", "registry", s.registry.Put(ctx, entry))
}

// List reads only application registry metadata, never the backend or values.
func (s *Store) List(ctx context.Context) ([]Entry, error) {
	entries, err := s.registry.List(ctx, s.namespace)
	if err != nil {
		return nil, SafeError("list", "registry", err)
	}
	for i := range entries {
		entries[i] = cloneEntry(entries[i])
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries, nil
}

func cloneMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	copy := make(map[string]string, len(input))
	for k, v := range input {
		copy[k] = v
	}
	return copy
}
func cloneReference(r Reference) Reference { r.Fields = cloneMap(r.Fields); return r }
func cloneEntry(e Entry) Entry {
	e.Reference = cloneReference(e.Reference)
	e.Metadata = cloneMap(e.Metadata)
	return e
}
