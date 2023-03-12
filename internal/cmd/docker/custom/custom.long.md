# Custom docker-compose services

You can define one or more custom sets of `docker-compose` services.
The default service set would be found in `custom.yml`,
with additional service sets being prefixed with `custom-` such as `custom-two.yml`.
These files should be created in the location returned by the `mw docker custom where` command.

## Example internal service

This service will be accessible within the `docker-compose` network to other services.

```yaml
version: '3.7'
services:
  <service-name>:
    image: <service-image>
    dns:
      - ${NETWORK_SUBNET_PREFIX}.10
    networks:
      - dps
```

## Example web service

This services will be accessible on your host machine via the virtual host specified.

```yaml
version: '3.7'
services:
  <service-name>:
    image: <service-image>
    environment:
      - VIRTUAL_HOST=<service-name>.mwdd.localhost,<service-name>.mwdd
      - VIRTUAL_PORT=<service-port>
    depends_on:
      - nginx-proxy
    dns:
      - ${NETWORK_SUBNET_PREFIX}.10
    networks:
      - dps
```

Note: If you use the docker hosts file integration, you may need to manually add this host to gain access.
