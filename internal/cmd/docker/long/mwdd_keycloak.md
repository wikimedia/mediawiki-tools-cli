# Keycloak service

[Keycloak](https://www.keycloak.org/) is an open source identity manager (IdM) that can be used to
provide single-sign on. It supports OpenID Connect and SAML.

They keycloak service allows you to add, delete, list, and get metadata for keycloak
realms, clients, and users.

## Setting up MediaWiki with OpenID Connect

You will need to create a realm, a client, and at least one user as follows:

```bash
mw docker keycloak create
mw docker keycloak add realm <realmname>
mw docker keycloak add client <clientname> <realmname>
mw docker keycloak add user <username> <temporarypassword> <realmname>
```

where &lt;realmname&gt; is the name you choose for your realm, &lt;clientname&gt; is the name
you choose for your client, &lt;username&gt; is the name you choose for your user, and
&lt;temporarypassword&gt; is a temporary password that you will be asked to change at your
first login.

Then, you will need to get the client secret that was assigned to your client:

```bash
mw docker keycloak get clientsecret <clientname> <realmname>
```

Using the client secret returned as &lt;clientsecret&gt; below, add the following to your
LocalSettings.php:

```php
wfLoadExtension('PluggableAuth');
wfLoadExtension('OpenIDConnect');
$wgPluggableAuth_Config = [
  "Keycloak" => [
    'plugin' => 'OpenIDConnect',
    'data' => [
      'providerURL' => 'http://keycloak.mwdd.localhost:8080/realms/<realmname>',
      'clientID' => '<clientname>',
      'clientsecret' => '<clientsecret>'
    ]
  ]
];
```

## More Control

If you need finer-grained control of the keycloak service, you can
use the exec command:

```bash
mw docker keycloak exec -- bash
```

to get a command line and then use the ```/opt/keycloak/bin/kcadm.sh``` commands shown in
[the Keycloak Admin CLI guide](https://www.keycloak.org/docs/latest/server_admin/#admin-cli).

## See Also

- [Keycloak](https://www.keycloak.org/docs/latest/server_admin/)
- [PluggableAuth](https://www.mediawiki.org/wiki/Extension:PluggableAuth)
- [OpenID Connect](https://www.mediawiki.org/wiki/Extension:OpenID_Connect)