# Typed disk cache for public API data

`Cache[T]` stores a JSON value and freshness deadline. Reuse it for small public inventories shared by commands and shell completion. Credentials and authenticated responses do not belong here.

```go
cache := diskcache.New[[]string](path, 24*time.Hour)
names, err := cache.Get(ctx, fetchNames, func() {
    cache.StartRefresh("--no-interaction", "some-command", "_refresh-cache")
})
```

- A missing, incomplete, or corrupt entry is fetched synchronously using the caller's context.
- A fresh entry returns without network I/O.
- A stale entry returns immediately and invokes the refresh callback.
- `Refresh(ctx, fetch)` forces a synchronous update and reports errors. Failed updates preserve old data and defer automatic retries for one minute.
- Writes use a temporary file and rename. Cache directory/write failures do not turn a successful fetch into an error.
- Entries are limited to 4 MiB when read; use a different mechanism for larger datasets.

For CLI background work, register a hidden command that calls `Refresh` with a bounded context and defers `FinishRefresh`. `StartRefresh` launches the current executable with the supplied arguments and null terminal streams; it does not use a shell. A per-cache lock prevents multiple workers and expires after one minute after a crash. Keep the worker timeout below one minute. The supplied arguments must be constant command paths/flags, never credentials. Normal goroutines would stop when the parent CLI exits.

Callers choose the cache path (normally under `xdg.CacheHome/mwcli`), freshness lifetime, fetch/validation logic, and refresh command. The package knows nothing about Toolviews or Cobra. Existing APIs may opt in individually; this does not automatically cache all CLI requests.
