# Wiki authentication

`wikimedia.txt` is an exact API endpoint allowlist, not a domain-suffix rule.
It is the intersection of Wikimedia's Action API site matrix and the production
`dblists/sul.dblist` shared-login membership list, excluding private wikis. This
avoids treating independently authenticated Wikimedia sites as shared accounts.

Refresh the list with `python3 tools/update-wikimedia-realms.py`, review the diff,
and run `go test ./internal/wikiauth`. List updates ship with CLI releases; no
discovery request is made when selecting credentials.

The default OAuth registration is the public a-wikimedia-cli consumer,
`b54c7ed6be6d2dc967a2d5101436f4a1`, with the exact
`http://localhost:8765/auth/login` callback. It uses authorization code + PKCE
without a client secret. The registration was proposed on 2026-10-31; the
proposer can test before approval. Users may supply another public client ID
and its registered loopback callback.

The CLI requests basic access by default and offers an interactive opt-in
chooser for this consumer's registered grants. Explicit --scope values bypass
the chooser; --all-scopes explicitly requests all registered permissions.
The display catalog in commands/wiki/scopes.go must track this consumer version.
Requested scopes are stored with profiles/token bundles and retained on refresh;
the client-grants resource must not be presented as the effective token scope.

OAuth token bundles are stored through the existing credential-provider API.
Only public identity and client metadata belong in ordinary configuration.
Bearer credentials attach only to the selected exact API endpoint, not asset
downloads or redirected requests. Refresh uses the same saved provider and
must persist any rotated refresh token before allowing the command to proceed.
