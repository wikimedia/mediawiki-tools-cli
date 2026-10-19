# Keycloak service

[Keycloak](https://www.keycloak.org/) is an open source identity manager that
supports OpenID Connect and SAML single sign-on.

Use the service commands to create, start, stop, or destroy its container. The
`add`, `delete`, `list`, and `get` subcommands manage realms, clients, and users,
or retrieve their metadata. Each subcommand's `--help` describes its arguments.

For direct administration, open a shell in the running container:

```sh
mw dev keycloak exec -- bash
```

Use `/opt/keycloak/bin/kcadm.sh` there for operations beyond the built-in
subcommands. See the [Keycloak Admin CLI guide](https://www.keycloak.org/docs/latest/server_admin/#admin-cli)
for its syntax.

## Learn more

For a workflow creating a realm, client, and user and configuring MediaWiki's
OpenID Connect login, see `mw help guides dev keycloak`.
