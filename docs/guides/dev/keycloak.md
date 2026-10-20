# Configure MediaWiki login with Keycloak

Connect a development wiki to Keycloak using OpenID Connect. This workflow
combines service creation, identity configuration, and MediaWiki settings.
For Keycloak command behavior and direct administration, read
`mw dev keycloak --help`.

## Create a realm, client, and user

With a [development wiki](setup.md) running, install the PluggableAuth and
OpenIDConnect extensions before enabling them in MediaWiki. Create a realm, a
client, and at least one user:

```bash
mw dev keycloak create
mw dev keycloak add realm <realmname>
mw dev keycloak add client <clientname> <realmname>
mw dev keycloak add user <username> <temporarypassword> <realmname>
```

where &lt;realmname&gt; is the name you choose for your realm, &lt;clientname&gt; is the name
you choose for your client, &lt;username&gt; is the name you choose for your user, and
&lt;temporarypassword&gt; is a temporary password that you will be asked to change at your
first login.

Then, you will need to get the client secret that was assigned to your client:

```bash
mw dev keycloak get clientsecret <clientname> <realmname>
```

## Configure MediaWiki

Using the returned secret as &lt;clientsecret&gt;, add the following to your
[LocalSettings.php](configuration.md). Replace the example port `8080` with your
environment's HTTP port if different:

```php
wfLoadExtension('PluggableAuth');
wfLoadExtension('OpenIDConnect');
$wgPluggableAuth_Config = [
  "Keycloak" => [
    'plugin' => 'OpenIDConnect',
    'data' => [
      'providerURL' => 'http://keycloak.local.wmftest.net:8080/realms/<realmname>',
      'clientID' => '<clientname>',
      'clientsecret' => '<clientsecret>'
    ]
  ]
];
```

Open your wiki and try logging in through Keycloak with the user you created.
Change the temporary password when prompted. If login fails, check that the
realm name, client name, client secret, and Keycloak URL match your configuration.

## References

- [Keycloak](https://www.keycloak.org/docs/latest/server_admin/)
- [PluggableAuth](https://www.mediawiki.org/wiki/Extension:PluggableAuth)
- [OpenID Connect](https://www.mediawiki.org/wiki/Extension:OpenID_Connect)
