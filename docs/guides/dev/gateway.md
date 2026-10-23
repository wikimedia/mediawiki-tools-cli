# Use the development gateway

Traefik serves your local development services on one port, usually 8080. Open `http://localhost:8080/proxy/dashboard/` to see services and installed wikis.

## Sign in

```sh
mw dev traefik credentials
```

This prints the saved gateway credentials and a single-use login link. The link expires after five minutes; the browser session lasts twelve hours. A fresh environment uses username `wm`; upgraded environments keep their existing credentials.

The gateway login is separate from MediaWiki, Keycloak or other application logins. Sign out using the dashboard link or `http://localhost:8080/_wm/logout`.

## Choose a service URL

Dashboard links offer:

- **Local:** the service's hostname, such as `default.mediawiki.local.wmftest.net`.
- **Proxy:** a path on the gateway, such as `/proxy/default/wiki/Main_Page`.
- **Tunnel:** the corresponding public HTTPS address when a Cloudflare Quick Tunnel is running.

Use HTTP for the local gateway unless you have configured an HTTPS proxy in front of it. Application-specific setup may be needed for services under a proxy path.

## Connect from another computer

For a temporary public HTTPS address:

```sh
mw dev tunnel start
mw dev tunnel status
mw dev tunnel url
mw dev traefik credentials
```

Open the printed tunnel login link. Gateway authentication remains enabled, including for public Quick Tunnels. The tunnel URL is temporary and can change after a restart. Stop the tunnel with `mw dev tunnel stop` when finished; `mw dev tunnel logs --follow` helps diagnose connection problems.

For a private connection instead:

```sh
mw dev tunnel start --provider holesail
```

Run the printed Holesail client command on the connecting computer, then open its local gateway address. Keep the connection string private. Generate a login link for the client address with `mw dev traefik credentials --url http://127.0.0.1:8082` when that is the port used by your client.

Use `mw dev tunnel status`, `url` and `logs --follow` to inspect the selected provider.
