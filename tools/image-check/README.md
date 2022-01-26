# image-check

This tool is intended to make using up to date container images in mwcli easier by providing a mwcli developer a quikc way to check all images used.

When run, it will output all tags that appear via SEMVER to be newer than the currently used tag

Usage:

```
go run ./tools/image-check/main.go
```

Output:

```
$ go run ./tools/image-check/main.go
-------------------------------------------------------
Checking adminer.yml service adminer using image adminer
Current tag: 4
Human URL: https://hub.docker.com/_/adminer?tab=tags
Available tags: [4.2 4.2-fastcgi 4.2-standalone 4.2.5 4.2.5-fastcgi 4.2.5-standalone 4.3 4.3-fastcgi 4.3-standalone 4.3.0 4.3.0-fastcgi 4.3.0-standalone 4.3.1 4.3.1-fastcgi 4.3.1-standalone 4.4 4.4-fastcgi 4.4-standalone 4.4.0 4.4.0-fastcgi 4.4.0-standalone 4.5 4.5-fastcgi 4.5-standalone 4.5.0 4.5.0-fastcgi 4.5.0-standalone 4.6 4.6-fastcgi 4.6-standalone 4.6.0 4.6.0-fastcgi 4.6.0-standalone 4.6.1 4.6.1-fastcgi 4.6.1-standalone 4.6.2 4.6.2-fastcgi 4.6.2-standalone 4.6.3 4.6.3-fastcgi 4.6.3-standalone 4.7 4.7-fastcgi 4.7-standalone 4.7.0 4.7.0-fastcgi 4.7.0-standalone 4.7.1 4.7.1-fastcgi 4.7.1-standalone 4.7.2 4.7.2-fastcgi 4.7.2-standalone 4.7.3 4.7.3-fastcgi 4.7.3-standalone 4.7.4 4.7.4-fastcgi 4.7.4-standalone 4.7.5 4.7.5-fastcgi 4.7.5-standalone 4.7.6 4.7.6-fastcgi 4.7.6-standalone 4.7.7 4.7.7-fastcgi 4.7.7-standalone 4.7.8 4.7.8-fastcgi 4.7.8-standalone 4.7.9 4.7.9-fastcgi 4.7.9-standalone 4.8.0 4.8.0-fastcgi 4.8.0-standalone 4.8.1 4.8.1-fastcgi 4.8.1-standalone]
-------------------------------------------------------
Checking base.yml service dps using image defreitas/dns-proxy-server
Current tag: 2.19.0
Human URL: https://hub.docker.com/r/defreitas/dns-proxy-server?tab=tags
Available tags: []
```

**Gotchas**

- Some tags are not semver
- Some Wikimedia images use totally different names when bumping php versions etc
- Some Wikimedia images get "security" bumps, with a suffix such as `-s1`, `-s2` etc, and per semver these are not newer?
- The output is a little hard to read?
- You still have to manually bump the images currently