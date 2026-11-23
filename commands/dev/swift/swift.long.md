Run a local OpenStack Swift object store with Wikimedia upload URL handling.

The first create builds a Debian-based development image. Objects persist when
stopping or rebuilding the service; destroy removes its data. MediaWiki storage
must be enabled explicitly for each wiki. Swift also works without Thumbor.
