# Wiki authentication

`wikimedia.txt` is an exact API endpoint allowlist, not a domain-suffix rule.
It is the intersection of Wikimedia's Action API site matrix and the production
`dblists/sul.dblist` shared-login membership list, excluding private wikis. This
avoids treating independently authenticated Wikimedia sites as shared accounts.

Refresh the list with `python3 tools/update-wikimedia-realms.py`, review the diff,
and run `go test ./internal/wikiauth`. List updates ship with CLI releases; no
discovery request is made when selecting credentials.

The temporary default OAuth registration is Addshore's approved public test
client, `50f06966290c605f175ec7ea7a4caabc`. It grants basic access only and requires
the exact `http://localhost/auth/login` callback. The client ID is public. There
is no client secret. Users can supply another client ID and registered loopback
callback; a production registration and its grants remain separate work.

OAuth token bundles are stored through the existing credential-provider API.
Only public identity and client metadata belong in ordinary configuration.
Bearer credentials attach only to the selected exact API endpoint, not asset
downloads or redirected requests. Refresh uses the same saved provider and
must persist any rotated refresh token before allowing the command to proceed.
