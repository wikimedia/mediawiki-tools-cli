Run Wikimedia's Thumbor thumbnail renderer with the local Swift storage service.

`wm dev thumbor create` starts Thumbor, Swift and memcached. MediaWiki must be
configured to use Swift and its thumbnail proxy before it sends rendering
requests to Thumbor. The internal endpoint is `http://thumbor:8800`.

Use `wm dev thumbor image` to select an image and `wm dev thumbor exec` to run
commands in the container. Container lists for sharded and private Swift storage
can be set with the comma-separated `SWIFT_SHARDED_CONTAINERS` and
`SWIFT_PRIVATE_CONTAINERS` context variables.

`MEDIAWIKI_SWIFT_PRIVATE_WIKIS` also accepts a comma-separated list of private
wiki database names, such as `officewiki`. Thumbor derives their private
containers automatically and combines them with `SWIFT_PRIVATE_CONTAINERS`.
