# secretstore

An application-scoped Go API for credentials in OS keyrings and KeePassXC.
Storage and provider selection are independent of service authentication.

```go
import (
    "context"
    "path/filepath"

    "gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
    "gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keyring"
)

// appConfigDir is your application's private configuration directory.
registry, err := secretstore.NewFileRegistry(
    filepath.Join(appConfigDir, "secrets", "registry.json"),
)
if err != nil { return err }

store, err := secretstore.Open(secretstore.Config{
    Namespace: "my-app",
    Provider:  "keyring",
    Registry:  registry,
    Backends:  []secretstore.Backend{keyring.New()},
})
if err != nil { return err }

ctx := context.Background()
if err := store.Set(ctx, "accounts/personal/token", tokenBytes); err != nil {
    return err
}
value, err := store.Get(ctx, "accounts/personal/token")
// Use value without printing or logging it.
```

The package is currently part of this Go module. It has no dependency on CLI
configuration, Cobra, Wikimedia services, or the internal packages. It is not
yet published as a separate module.
It currently uses the repository's GPL-3.0 license; no separate license has been
introduced for the extracted code.

## API and behavior

- `Get(ctx, key)` reads the provider recorded for that application key.
- `Set(ctx, key, value)` verifies a write before committing its reference.
- `SetWithMetadata` accepts public labels, such as an account or endpoint.
- `Delete` removes app-owned entries where the provider supports deletion. For
  explicitly linked external entries it only removes the app's reference.
- `Unlink` always forgets the reference without deleting an external entry.
- `List` reads only registry metadata. It does not read credentials or unlock a
  provider. Returned metadata is copied so callers cannot mutate the registry.
- `Link` imports a caller-selected reference without accessing its value. It
  refuses to silently replace a key already in the registry.
- `Transfer` verifies another provider before changing a key's route, retaining
  the source entry. It never silently changes storage because a provider fails.

Changing `Config.Provider` changes the default for **new** keys. Existing keys
continue using their recorded provider. The namespace is a stable application
identifier, independent of executable names and branding. It groups records;
it is not a security boundary against other software running as the same user.

File registries contain public references, ownership flags and metadata, never
values. Their parent directory must be private on Unix. Files are written as
`0600`, replaced atomically, and protected by OS locks across processes. Locks
are released by the OS when a process exits. Registry files and lock paths must
be regular files. Windows uses the profile's filesystem ACLs.

Use `NewMemoryRegistry()` for tests or temporary references. Applications may
implement `Registry` to persist references in an existing configuration store.
`Registry.Lock` must serialize mutations to the same key, including provider
writes. Metadata supplied to a registry must not contain secrets.

## Providers

### OS keyring

The keyring adapter uses the existing `zalando/go-keyring` native integration:
Secret Service on Linux, Keychain on macOS, and Credential Manager on Windows.
WSL uses Linux Secret Service; it does not automatically use the Windows host's
Credential Manager. No file or plaintext fallback is enabled.

New native item identifiers encode both namespace and key to avoid collisions
when an OS backend combines its service and account fields. Imported references
retain their identifiers. Windows values are limited to 2560 bytes. Unknown
platform limits are reported by the provider rather than silently truncating or
splitting a value. OS calls are checked for cancellation before/after access,
but an in-flight native call may not itself support cancellation.

### KeePassXC

```go
browser, err := keepassxc.NewClient(
    filepath.Join(appConfigDir, "secrets", "keepassxc-pairing.json"),
    "my-app", // Dedicated application group in the database.
)
if err != nil { return err }

// During explicit first-time setup only:
// err = browser.Pair(ctx, "/absolute/path/to/keepassxc-proxy")

backend := keepassxc.New(browser)
```

Import `pkg/secretstore/keepassxc` and register that backend in `Config.Backends`.
The default may then be `"keepassxc"`. Keep the namespace and group stable and
use a dedicated group for each app. Public metadata `account` and `endpoint`
can supply a readable login and service URL; otherwise app-scoped defaults are
used. Text values remain text in the vault. Non-UTF-8 values are encoded and the
reference records how to decode them, preserving arbitrary byte values.

Pairing, status, discovery and disconnect are explicit `Client` operations.
Pairing credentials live in a separate private file; the registry holds only
the database ID, entry UUID, login and encoding. Keep that pairing file private:
it grants access while the paired database is unlocked and permissions allow.
`Detect` does not access credentials or launch the GUI. WSL can select Windows
`keepassxc-proxy.exe` using its absolute `/mnt/c/...` path.

Reads use pinned database and entry IDs. Writes read back the exact saved value
before reporting success. The browser protocol does not support deleting an
entry, so `Delete` on an app-owned KeePassXC reference returns `ErrUnsupported`;
use `Unlink` to forget the key while retaining the entry in KeePassXC.

## Errors and interaction

Use `errors.Is` with `ErrNotFound`, `ErrLocked`, `ErrDenied`, `ErrUnavailable`,
`ErrUnsupported`, `ErrTooLarge`, `ErrInvalid`, `ErrVerification`, `ErrConflict`
and `ErrInteractionRequired`, or with context cancellation/deadline errors.
Unknown provider failures are conservatively unavailable. Public operation
errors do not copy provider stdout, stderr, values or raw diagnostic bodies.

Set `Interaction: secretstore.NoInteraction` to reject operations on providers
that may prompt. This is conservative: a backend with `MayPrompt` is rejected
even if its vault happens to be unlocked. Metadata listing remains available.
`Open` never pairs, discovers providers, or reads a credential.

Registry updates follow verified writes. There is no transaction spanning an
external vault and a local file: a write may have reached the provider even if
verification or saving the reference fails. `ErrVerification` therefore does
not mean that the provider's old value is unchanged. Retain plaintext/source
copies until the full operation succeeds; never delete the old item first.

## CLI plaintext migration

The CLI uses its config as an application-owned metadata registry and routes
keyring and KeePassXC reads/writes through this package. Service/account binding,
interactive provider choice and remote authentication checks remain in the CLI.
It retains the explicit migration path from `main`'s plaintext configuration:

```sh
./bin/mw config secrets migrate --dry-run --provider keepassxc
./bin/mw config secrets migrate --provider keepassxc
```

If a linked destination already differs, the ordinary command preserves both
copies. To explicitly replace one linked credential with its plaintext value:

```sh
./bin/mw config secrets migrate phabricator.sites.wikimedia.key --replace
```

Plaintext is removed only after a verified save and a successful config write.
These commands are never run automatically by the library or during upgrades.

## Validation

```sh
GOFLAGS=-mod=mod go test -race ./pkg/secretstore/...
```

Tests use fake native stores and encrypted, framed KeePassXC proxy fixtures.
They cover provider routing, byte preservation, private/atomic registries,
concurrent writers, cancellation, source retention, ownership, and protocol
failures without touching a user's real keyring or vault.
