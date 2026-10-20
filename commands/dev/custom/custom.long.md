# Custom Compose services

Define custom Compose services or override bundled service settings. The default
service set uses `custom.yml`. Additional sets use names such as
`custom-two.yml` or `custom.local.yml`; select their basename with `--name`.

`mw dev custom where` prints the selected file's path. Create additional files
in that same directory. The CLI manages the directory but preserves
`custom*.yml` and `custom*.yaml` files during sync.

`mw dev custom --name custom-two create` creates the services defined in that
file. Its `stop`, `start`, and `destroy` commands act on the same service set.
`exec` targets the service whose name matches the selected set's basename.

## Example override

Custom files can override existing Compose attributes, for example adding an
environment variable to MySQL:

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

This service will be accessible on your host machine via the virtual host specified.

```yaml
services:
  <service-name>:
    image: <service-image>
    environment:
      - VIRTUAL_HOST=<service-name>.local.wmftest.net
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

## Dashboard discovery

Custom Compose services are listed automatically after a CLI command prepares
the environment (including `custom create`). Both web and non-web services are
included; status uses the same internal DNS check as bundled services. A
`VIRTUAL_HOST` value supplies the web UI link, using the first hostname when it
lists aliases. Map and list environment syntax and standard variable/default
substitution are supported. The dashboard reads generated name/host metadata
from its existing read-only data mount; it does not receive the Docker socket or
other service environment values. Reload after its five-second status cache
expires. If you edit custom YAML while the dashboard is running, run a CLI
command such as `mw dev where` to refresh the metadata.

## Routing and authentication

The host-based route keeps local development access open. The `/proxy/<service-name>/...` route is for the public gateway and requires the generated Traefik login by default. `TRAEFIK_AUTH_MIDDLEWARE` can override the middleware used by gateway routes. Add the local hostname with `mw dev hosts add --host <service-name>.local.wmftest.net` when needed; `VIRTUAL_HOST` and `VIRTUAL_PORT` are not required for Traefik or local DNS.

## Learn more

For a walkthrough creating, inspecting, stopping and removing a custom service,
see `mw help guides dev custom-services`.
