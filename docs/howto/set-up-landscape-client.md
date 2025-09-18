---
myst:
  html_meta:
    "description lang=en":
      "The Landscape client in Ubuntu on WSL instances can be configured with Ubuntu Pro for WSL to support remote management and deployment."
---

# Configure the Landscape client with Ubuntu Pro for WSL

```{include} ../pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

(howto::config-landscape)=
## Choose a configuration method

The Landscape client can be configured in two ways:

- Windows registry: easier to automate and deploy at scale 
- Graphical Windows application: convenient option for individual users

Click the appropriate tab to read more.

````{tabs}

```{group-tab} Windows registry

## Access the registry

First, ensure that UP4W has run at least once after installation.
This ensures that the key and values necessary for configuration will be set up
in the registry.

Advanced users of the registry can find relevant information in the
[Microsoft documentation](https://learn.microsoft.com/en-us/troubleshoot/windows-server/performance/windows-registry-advanced-users)
about alternative methods for modifying the registry data.

To open the registry type `Win+R` and enter `regedit`:

![Windows where regedit command is run.](./assets/regedit.png) 

## Configure Landscape in the registry

If you are using Landscape you can input your configuration in `LandscapeConfig > Modify > Write the Landscape config`:

![Windows registry with child window showing Landscape configuration.](./assets/registry-child-window-config.png) 

Refer to the section on [Landscape client configuration](howto::config-landscape-client) for an example.

After you have populated the configuration with data you should be ready to create and manage 
automatically Pro-attaching WSL instances through Landscape:

![Windows registry showing completed Ubuntu Pro for WSL configuration.](./assets/registry-complete.png) 

```

```{group-tab} Graphical Windows application
In the UP4W app navigate to the Landscape configuration screen:

![UP4W GUI main screen](../assets/landscape-config-ui.png)

Choose your preferred configuration option and enter the required details.

The "Manual configuration" is easier if the server was configured with the
default options (if in doubt, check this with your system administrator), in
which case only the "Landscape FQDN" field is truly required. This field does
not accept a complete URL for the server (with paths and queries, for example),
and a FQDN should always be used. The field accepts URLs like
`https://landscape-server.domain.com` (with a just scheme and a host name) but
the `https://` part (the scheme) is removed from the address, resulting in the
FQDN only. This is for user convenience, as it allows you to copy an address for
a server from a web browser address bar, for example, and paste it into the
field. Richer URLs with queries, paths and fragments will be rejected and an
error message will be shown.

The "Advanced configuration" option requires you to specify a `landscape.conf`.
Refer to the section on [Landscape client configuration](howto::config-landscape-client) for an example.

When you continue, a status screen will appear confirming that configuration is complete:

![Configuration is complete](../assets/status-complete.png)

The application waits a short period of time to confirm that the configuration
data supplied resulted in a successful connection to the Landscape server. In
case of errors, a dialog presents the error details and lets you decide whether
to edit the configuration and try again, or proceed with the configuration
already provided.

```

````

```{warning}
Until version 0.1.15 of Ubuntu Pro for WSL, the app explicitly requires referencing a path
to the SSL certificate on a Windows host machine.
Newer versions completely follow the Windows OS certificate stores, only requiring reference
to that certificate if the machine running the Landscape server is not trusted on your network.

For example, if you followed the [Landscape Quickstart](https://ubuntu.com/landscape/docs/quickstart-deployment)
installation, the auto-generated self-signed certificate can be found at `/etc/ssl/certs/landscape_server.pem`.

This can be copied to a Windows machine:

>  C:\Users\username\landscape_server.pem

The path can then be referenced during Landscape configuration in the UP4W Windows app.
```

(howto::config-landscape-client)=
## Configuring the landscape client

Both the `LandscapeConfig` data in the Windows registry and the Advanced Configuration
option in the graphical Windows application can be configured as follows:


```ini
[host]
url = landscape-server.domain.com:6554

[client]
url = https://landscape-server.domain.com/message-system
ping_url  = http://landscape-server.domain.com/ping
account_name = standalone
log_level = debug
ssl_public_key = C:\Users\username\Downloads\landscape_server.pem
```

```{warning}
The `url` field in the `[host]` section must be `FQDN:PORT`. An actual URL with
scheme, path, queries and/or fragments would cause the connection to fail.

The `ping_url` must be a `http` address. A `https` address will not work.
```

A more comprehensive example of the configuration options is provided [here](https://github.com/canonical/landscape-client/blob/main/example.conf).
