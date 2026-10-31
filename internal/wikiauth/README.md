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

The CLI requests basic access by default. Interactive logins for every client use
one shared catalogue in `commands/wiki/grant_catalogue.go`: MediaWiki 1.46.2 core,
plus the OAuth and Echo extension definitions from REL1_46. The catalogue does not
claim these grants are registered for a particular consumer; the server validates
requested scopes. There is no live discovery or cache.

Sources for catalogue updates:

- https://github.com/wikimedia/mediawiki/blob/1.46.2/includes/MainConfigSchema.php
  (`GrantPermissions`, `GrantPermissionGroups`, `GrantRiskGroups`).
- https://github.com/wikimedia/mediawiki/blob/1.46.2/languages/i18n/userrights/en.json
  (grant and group labels).
- https://github.com/wikimedia/mediawiki-extensions-OAuth/blob/REL1_46/extension.json
  and `i18n/en.json` (`oauthmanageownclient`, internal).
- https://github.com/wikimedia/mediawiki-extensions-Echo/blob/REL1_46/extension.json
  and `i18n/en.json` (`echo-read-notifications`, low).

Preserve missing metadata as unknown. In 1.46.2, the core grouping/risk maps still
name `editviewmywatchlist`, while the actual grant is `editmywatchlist`. The latter
therefore has the upstream fallback group (`other`) and unknown risk; do not
silently infer a low risk rating. New explicit/saved scopes absent from the
catalogue also remain unknown.

The picker retains upstream functional groups and uses low/vandalism/security
risk levels for cumulative Right/Left presets. Internal/unknown permissions are
manual-only in the picker and are preserved across preset changes if already
selected. Explicit `--all-scopes` means every entry in the bundled catalogue,
including internal/unknown grants. It always sends and saves explicit scope names.
Explicit `--scope` values are validated by the request builder and wiki, not by a
per-consumer allowlist. Requested scopes remain attached to profiles/token bundles
on refresh; a consumer's registered grants must not be presented as the effective
token scope.

OAuth token bundles are stored through the existing credential-provider API.
Only public identity and client metadata belong in ordinary configuration.
Bearer credentials attach only to the selected exact API endpoint, not asset
downloads or redirected requests. Refresh uses the same saved provider and
must persist any rotated refresh token before allowing the command to proceed.

The `wikibase-world` preset uses public consumer version 1.1,
`8546b3108b91bd9ec7292c0aab4bb158`, with the same loopback callback. It authorizes
only `https://wikibase.world/w/api.php`. Built-in realm inference and completion
are local and cannot broaden the endpoint allowlist. Custom public clients must
be explicitly selected; OAuth REST endpoints follow their API script path.

Public-client refresh without a client secret needs MediaWiki 1.46 or a backport.
wikibase.world reported 1.43.11 at registration, so initial authorization and
refresh must be tested separately. A failed refresh preserves the saved bundle
and provides reauthorization instructions; no client secret is embedded.
