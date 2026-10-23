# Add and manage a custom service

This walkthrough adds a small internal container to an existing
[development environment](setup.md), inspects it, and removes it. Use the same
[context](contexts.md) for every command.

For file conventions, Compose override and web-routing examples, dashboard
discovery, and authentication behavior, read `mw dev custom --help`.

## Define a service

Find the Compose file selected by the example service-set name:

```sh
mw dev custom --name custom-example where
```

Create the file at the printed path with the following content. If that file
already exists, choose another `custom-` name and adjust the commands and
service name below. Do not overwrite an existing definition.

```yaml
services:
  custom-example:
    image: busybox:1.37
    command: ["sleep", "86400"]
    security_opt:
      - label:disable
    dns:
      - ${NETWORK_SUBNET_PREFIX}.10
    networks:
      - dps
```

This container runs for a day and publishes no web endpoint. Matching its
service name to the service-set name lets `custom exec` select it directly.

## Start and inspect it

```sh
mw dev custom --name custom-example create
mw dev custom --name custom-example exec -- uname -a
```

Check the command output and find `custom-example` on the environment dashboard.
Allow its five-second status cache to refresh. The dashboard lists internal
services too, so a service can appear without a web UI link.

To adapt this example into a web service, follow the configuration examples in
`mw dev custom --help` and the [gateway guide](gateway.md). They explain local
hostname access and authenticated public routes.

## Stop, restart, and remove it

```sh
mw dev custom --name custom-example stop
mw dev custom --name custom-example start
mw dev custom --name custom-example exec -- uname -a
```

Remove the example container when finished:

```sh
mw dev custom --name custom-example destroy
```

Then delete only the example Compose file you created. Refresh dashboard
metadata with `mw dev where` and reload after the status cache expires.
