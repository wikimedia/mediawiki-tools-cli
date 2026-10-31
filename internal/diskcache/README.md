# Typed disk cache for public API data

`Cache[T]` stores a JSON value and freshness deadline. Reuse it for small public inventories shared by commands and shell completion. Credentials and authenticated responses do not belong here.

```go
cache := diskcache.New[[]string](path, 24*time.Hour)
names, err := cache.Get(ctx, fetchNames, func() {
    cache.StartRefresh("--no-interaction", "some-command", "_refresh-cache")
})
```

- A missing, incomplete, or corrupt entry is fetched synchronously using the caller's context. Failed cold fetches persist a one-minute retry deadline, so repeated completion requests do not repeatedly wait on an unavailable API.
- A fresh entry returns without network I/O.
- A stale entry returns immediately and invokes the refresh callback.
- `Refresh(ctx, fetch)` forces a synchronous update and reports errors. Refreshes are serialized across processes so an older request cannot overwrite a later refresh. Failed updates preserve old data and defer automatic retries for one minute.
- Writes use a temporary file and rename. Cache directory/write failures do not turn a successful fetch into an error.
- Entries are limited to 4 MiB when read; use a different mechanism for larger datasets.

For CLI background work, register a hidden command that calls `RefreshIfStale` with a bounded context. It rechecks freshness after acquiring the shared refresh lock, so redundant workers skip an already completed update. `StartRefresh` launches the current executable with the supplied arguments and null terminal streams; it does not use a shell. Launches are throttled for one minute under the same lock. OS-managed file locks release automatically on process exit or crash; lock files stay in place and must not be removed while the CLI is running. There is no worker cleanup method or expiring lock ownership. The supplied arguments must be constant command paths/flags, never credentials. A background worker survives its parent exiting; when the parent stays alive it reaps the child.

Callers choose the cache path (normally under `xdg.CacheHome/mwcli`), freshness lifetime, fetch/validation logic, and refresh command. The package knows nothing about Toolviews or Cobra. Existing APIs may opt in individually; this does not automatically cache all CLI requests.

Cache entries distinguish a cached value from a failed fetch explicitly, including when the successful value is a nil slice, map, or pointer. Existing non-null cache entries remain readable.
