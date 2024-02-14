(ref::landscape)=
# Landscape

Landscape is a systems management tool designed to help you manage and monitor your Ubuntu systems from a unified platform.
> See more: [Landscape | Documentation ](https://ubuntu.com/landscape/docs).

In the context of UP4W, Landscape consists of a remote server and two clients, (1) the usual Ubuntu-side client, in this case a [Landscape client](ref::landscape-client) that comes automatically with any Ubuntu WSL instance, and (2) a Windows-side client, a Landscape client that is built into your UP4W agent. The latter, Windows-side client, offers Ubuntu WSL specific advantages – the ability to create new instances through Landscape and the ability to configure all your instances at scale (when you configure the Windows-side client, the UP4W agent forwards the configuration to the client on each instance).

(ref::landscape-config)=
## Landscape configuration schema

As in other Landscape setups, in UP4W Landscape is configured via an `.ini` file. In UP4W this file is provided to the Windows host.
> See more: [How to configure UP4W](howto::configure-up4w)

The schema for this file is as usual, with a few additional keys specific to the WSL setting, which can be grouped into keys that affect just the Windows-side client and keys that affect both the Windows-side client and the Ubuntu WSL-side client(s). These additions are documented below.

> See more: [Landscape | Configure Ubuntu Pro for WSL for Landscape](https://ubuntu.com/landscape/docs/register-wsl-hosts-to-landscape/#heading--configure-ubuntu-pro-for-wsl-for-landscape)

Here is an example of what the configuration looks like:
```ini
[host]
url = https://landscape-server.domain.com:6554

[client]
url = https://landscape-server.domain.com/message-system
ping_url  = https://landscape-server.domain.com/ping
account_name = standalone
log_level = debug
```

### Host

This section contains settings unique to the Windows-side client. It contains a single key:
- `url`: The URL of your Landscape account followed by a colon (`:`) and the port number. Port 6554 is the default for Landscape Quickstart installations.

### Client

This section contains settings used by both clients. Most keys in this section behave the same way that they would on a traditional Landscape setup. Only the following keys behave differently:
- `ssl_public_key`: This key must be a Windows path. The WSL instances will have this path translated automatically.
- `computer_title`: This key will be ignored. Instead, each WSL instance will use its Distro name as computer title.
- `hostagent_uid`: This key will be ignored.

> See more: [GitHub | Landscape client configuration schema](https://github.com/canonical/landscape-client/blob/master/example.conf)
