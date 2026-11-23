# Adapted from Wikimedia Foundation wmf-claude, commit
# 003fa611cd670d030a0063986a876960f2ba7214 (MIT; see LICENSE).
# Swift development service: the media storage of production (ms-fe and ms-be),
# as one container.
#
# The packages are those of Debian 13, which production's puppet supports
# for its Swift hosts next to Debian 11 (modules/swift/manifests/proxy.pp
# puts the middlewares in python3.13 there). Debian 11 is past its LTS:
# its security archive no longer serves the packages that its indexes
# name, so an image of it does not build. The three WMF middlewares (the
# URL rewrite of upload.wikimedia.org with its request to Thumbor, CORS,
# and the Cache-Control default) come from a pinned commit of
# operations/puppet; puppet keeps them in files/python3.9 for each release.
# rewrite.py uses urllib.request but imports only urllib.parse, so it
# loads only when another module imported urllib.request before it. The
# build adds the import, unless the file of puppet has it.
#
# Differences from production:
# - One host with one device and one replica. No replicator, auditor,
#   updater or expirer: an object with X-Delete-At stops to exist for a
#   reader at that time, but stays on the disk.
# - One cluster. Production writes to two (FileBackendMultiWrite).
# - No statsd, and the keys are for development only.
FROM debian:trixie
ARG PUPPET_COMMIT=4600c31b5e06319731514f3d2ccba51b94fd504c
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      swift swift-proxy swift-account swift-container swift-object \
      python3-swiftclient memcached ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*
RUN set -e; d=/usr/local/lib/python3.13/dist-packages/wmf; mkdir -p "$d"; \
    for f in __init__.py rewrite.py cors.py ensure_max_age.py; do \
      curl -fsS "https://gerrit.wikimedia.org/r/plugins/gitiles/operations/puppet/+/${PUPPET_COMMIT}/modules/swift/files/python3.9/SwiftMedia/wmf/${f}?format=TEXT" \
        | base64 -d > "$d/$f"; \
    done; \
    grep -qx 'import urllib.request' "$d/rewrite.py" \
      || sed -i 's/^import urllib\.parse$/&\nimport urllib.request/' "$d/rewrite.py"; \
    python3 -c 'import wmf.rewrite, wmf.cors, wmf.ensure_max_age'
COPY swift.conf account-server.conf container-server.conf object-server.conf /etc/swift/
COPY proxy-server.conf /etc/swift/proxy-server.conf.template
COPY start.sh healthcheck.sh configure.py /usr/local/bin/
COPY LICENSE /usr/share/doc/mwcli-swift/LICENSE
# One device, one replica, 2^8 partitions.
RUN set -e; cd /etc/swift; \
    for r in object:6200 container:6201 account:6202; do \
      swift-ring-builder "${r%:*}.builder" create 8 1 1; \
      swift-ring-builder "${r%:*}.builder" add --region 1 --zone 1 --ip 127.0.0.1 --port "${r#*:}" --device d1 --weight 1; \
      swift-ring-builder "${r%:*}.builder" rebalance; \
    done; \
    chown -R swift:swift /etc/swift; chmod +x /usr/local/bin/start.sh /usr/local/bin/healthcheck.sh
EXPOSE 8080
CMD ["/usr/local/bin/start.sh"]
