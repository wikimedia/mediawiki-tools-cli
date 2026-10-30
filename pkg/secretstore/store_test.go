package secretstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type testBackend struct {
	name                   string
	mu                     sync.Mutex
	values                 map[string][]byte
	reads, writes, deletes int
	failGet, failSet       bool
	canDelete              bool
	limit                  int
}

func backend(name string) *testBackend {
	return &testBackend{name: name, values: map[string][]byte{}, canDelete: true}
}
func (b *testBackend) Name() string { return b.name }
func (b *testBackend) Capabilities() Capabilities {
	return Capabilities{CanDelete: b.canDelete, MaxValueBytes: b.limit}
}
func (b *testBackend) Get(ctx context.Context, ref Reference) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reads++
	if b.failGet {
		return nil, fmt.Errorf("raw backend diagnostics secret-fixture")
	}
	v, ok := b.values[ref.Fields["id"]]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}
func (b *testBackend) Set(ctx context.Context, request WriteRequest, value []byte) (Reference, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.writes++
	if b.failSet {
		return Reference{}, fmt.Errorf("raw backend diagnostics secret-fixture")
	}
	ref := Reference{Provider: b.name, Fields: map[string]string{"id": request.Namespace + "/" + request.Key}, Owned: true}
	if request.Existing != nil {
		ref = *request.Existing
	}
	b.values[ref.Fields["id"]] = append([]byte(nil), value...)
	return ref, nil
}
func (b *testBackend) Delete(ctx context.Context, ref Reference) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deletes++
	delete(b.values, ref.Fields["id"])
	return nil
}
func openTest(t *testing.T, namespace, provider string, r Registry, backends ...Backend) *Store {
	t.Helper()
	s, err := Open(Config{Namespace: namespace, Provider: provider, Registry: r, Backends: backends})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDurableRoutingAndNamespaces(t *testing.T) {
	r := NewMemoryRegistry()
	a, b := backend("a"), backend("b")
	s := openTest(t, "app", "a", r, a, b)
	if err := s.Set(t.Context(), "token", []byte("secret-fixture")); err != nil {
		t.Fatal(err)
	}
	// Reopening with another default must retain the original item's provider.
	s = openTest(t, "app", "b", r, a, b)
	if got, err := s.Get(t.Context(), "token"); err != nil || string(got) != "secret-fixture" {
		t.Fatal("default change lost credential")
	}
	if err := s.Set(t.Context(), "token", []byte("updated")); err != nil {
		t.Fatal(err)
	}
	if b.reads != 0 || b.writes != 0 {
		t.Fatal("default change silently switched an existing provider")
	}
	other := openTest(t, "other-app", "a", r, a, b)
	if _, err := other.Get(t.Context(), "token"); !errors.Is(err, ErrNotFound) {
		t.Fatal("namespace isolation failed")
	}
	if err := other.Set(t.Context(), "token", []byte("other")); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(t.Context())
	if err != nil || len(entries) != 1 || entries[0].Namespace != "app" {
		t.Fatal("list crossed namespaces")
	}
	entries[0].Reference.Fields["id"] = "tampered"
	if got, _ := s.Get(t.Context(), "token"); string(got) != "updated" {
		t.Fatal("metadata list leaked mutable internal references")
	}
}

func TestFailureNeverFallsBackOrRemovesReference(t *testing.T) {
	for _, mode := range []string{"write", "verify"} {
		t.Run(mode, func(t *testing.T) {
			r := NewMemoryRegistry()
			a, b := backend("a"), backend("b")
			s := openTest(t, "app", "a", r, a, b)
			if err := s.Set(t.Context(), "token", []byte("original")); err != nil {
				t.Fatal(err)
			}
			before, _ := r.Get(t.Context(), "app", "token")
			if mode == "write" {
				a.failSet = true
			} else {
				a.failGet = true
			}
			err := s.Set(t.Context(), "token", []byte("secret-fixture"))
			if err == nil || strings.Contains(err.Error(), "secret-fixture") {
				t.Fatal("failed operation exposed diagnostics or succeeded")
			}
			after, _ := r.Get(t.Context(), "app", "token")
			if after.Reference.Fields["id"] != before.Reference.Fields["id"] || b.writes != 0 {
				t.Fatal("failed write changed route or used another provider")
			}
		})
	}
}

func TestDeleteOwnershipAndUnsupportedDelete(t *testing.T) {
	r := NewMemoryRegistry()
	a := backend("a")
	s := openTest(t, "app", "a", r, a)
	ref := Reference{Provider: "a", Fields: map[string]string{"id": "external"}}
	if err := s.Link(t.Context(), "external", ref, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(t.Context(), "external"); err != nil || a.deletes != 0 {
		t.Fatal("external item was deleted instead of unlinked")
	}
	if err := s.Set(t.Context(), "owned", []byte("value")); err != nil {
		t.Fatal(err)
	}
	a.canDelete = false
	if err := s.Delete(t.Context(), "owned"); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported deletion was not reported")
	}
	if _, err := s.Get(t.Context(), "owned"); err != nil {
		t.Fatal("unsupported delete lost registry entry")
	}
	if err := s.Unlink(t.Context(), "owned"); err != nil || a.deletes != 0 {
		t.Fatal("unlink deleted provider item")
	}
}

func TestCanceledOperationsAndLimits(t *testing.T) {
	r := NewMemoryRegistry()
	a := backend("a")
	a.limit = 3
	s := openTest(t, "app", "a", r, a)
	if err := s.Set(t.Context(), "token", []byte("large")); !errors.Is(err, ErrTooLarge) || a.writes != 0 {
		t.Fatal("oversized value reached provider")
	}
	unlock, err := r.Lock(t.Context(), "app", "token")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if err := s.Set(ctx, "token", []byte("ok")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("blocked operation did not cancel")
	}
}

func TestTransferVerifiesBeforeRoutingAndRetainsSource(t *testing.T) {
	r := NewMemoryRegistry()
	a, b := backend("a"), backend("b")
	s := openTest(t, "app", "a", r, a, b)
	if err := s.Set(t.Context(), "token", []byte("secret-fixture")); err != nil {
		t.Fatal(err)
	}
	b.failGet = true
	if err := s.Transfer(t.Context(), "token", "b"); !errors.Is(err, ErrVerification) {
		t.Fatal("unverified destination accepted")
	}
	entry, _ := r.Get(t.Context(), "app", "token")
	if entry.Reference.Provider != "a" {
		t.Fatal("failed transfer switched provider")
	}
	b.failGet = false
	if err := s.Transfer(t.Context(), "token", "b"); err != nil {
		t.Fatal(err)
	}
	entry, _ = r.Get(t.Context(), "app", "token")
	if entry.Reference.Provider != "b" || a.deletes != 0 || string(a.values["app/token"]) != "secret-fixture" {
		t.Fatal("transfer deleted source or did not switch route")
	}
}

type promptingBackend struct{ *testBackend }

func (b promptingBackend) Capabilities() Capabilities {
	caps := b.testBackend.Capabilities()
	caps.MayPrompt = true
	return caps
}
func TestNoInteractionNeverCallsPromptingBackend(t *testing.T) {
	r := NewMemoryRegistry()
	base := backend("prompting")
	s, err := Open(Config{Namespace: "app", Provider: base.Name(), Registry: r, Backends: []Backend{promptingBackend{base}}, Interaction: NoInteraction})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set(t.Context(), "token", []byte("value")); !errors.Is(err, ErrInteractionRequired) || base.writes != 0 {
		t.Fatal("noninteractive operation invoked provider")
	}
	if _, err := s.List(t.Context()); err != nil || base.reads != 0 {
		t.Fatal("metadata listing invoked provider")
	}
}
