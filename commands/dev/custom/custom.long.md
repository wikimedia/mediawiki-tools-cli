# Custom docker-compose services

You can define one or more custom sets of `docker compose` services or files.
The default service and file would be found in `custom.yml`,
with additional service sets and files being prefixed with `custom-` such as `custom-two.yml`.
Names prefixed with `custom.` (for example `custom.local.yml`) are also accepted.
These files should be created in the location returned by the `mw dev custom where` command.
That directory is managed by the CLI, but `custom*.yml` and `custom*.yaml` files are preserved and will not be cleaned up by sync.
You can then create these services by running the `mw dev custom` command,
optionally with a `--name` flag, such as `mw dev custom --name custom-two create`.

## Example override

You can use these files to override any default docker compose attributes.

Such as defining a custom environment variable for a service.

```yaml
services:
  mysql:
    environment:
      - CUSTOM=yes
```

## Example internal service

This service will be accessible within the `docker compose` network to other services.

```yaml
services:
  <service-name>:
    image: <service-image>
    security_opt:
      - label:disable
    dns:
      - ${NETWORK_SUBNET_PREFIX}.10
    networks:
      - dps
```

## Example web service

This services will be accessible on your host machine via the virtual host specified.

```yaml
services:
  <service-name>:
    image: <service-image>
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.<service-name>-host.rule=Host(`<service-name>.local.wmftest.net`) && !PathPrefix(`/proxy/`)"
      - "traefik.http.routers.<service-name>-host.entrypoints=web"
      - "traefik.http.routers.<service-name>-host.service=<service-name>"
      - "traefik.http.routers.<service-name>.rule=Path(`/proxy/<service-name>`) || PathPrefix(`/proxy/<service-name>/`)"
      - "traefik.http.routers.<service-name>.entrypoints=web"
      - "traefik.http.routers.<service-name>.middlewares=${TRAEFIK_AUTH_MIDDLEWARE:-gateway-auth@file},strip-<service-name>@docker,forwarded-https@file"
      - "traefik.http.middlewares.strip-<service-name>.stripprefix.prefixes=/proxy/<service-name>"
      - "traefik.http.services.<service-name>.loadbalancer.server.port=<service-port>"
    depends_on:
      - traefik
    security_opt:
      - label:disable
    dns:
      - ${NETWORK_SUBNET_PREFIX}.10
    networks:
      - dps
```

Note: If you use the docker hosts file integration, you may need to manually add this host to gain access.


## Dashboard discovery

Custom Compose services are listed automatically after a mwcli command prepares
the environment (including `custom create`). Both web and non-web services are
included; status uses the same internal DNS check as bundled services. A
`VIRTUAL_HOST` value supplies the web UI link, using the first hostname when it
lists aliases. Map and list environment syntax and standard variable/default
substitution are supported. The dashboard reads generated name/host metadata
from its existing read-only data mount; it does not receive the Docker socket or
other service environment values. Reload after its five-second status cache
expires. If you edit custom YAML while the dashboard is running, run a mwcli
command such as `mw dev where` to refresh the metadata.

The host-based route keeps local development access open, like the previous nginx proxy. The `/proxy/<service-name>/...` route is for the public gateway and requires the generated Traefik login by default. `TRAEFIK_AUTH_MIDDLEWARE` can override the middleware used by gateway routes. Add the local hostname with `mw dev hosts add --host <service-name>.local.wmftest.net` when needed; `VIRTUAL_HOST` and `VIRTUAL_PORT` are not required for Traefik or local DNS.
