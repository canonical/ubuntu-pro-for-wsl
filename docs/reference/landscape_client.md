(ref::landscape-client)=
# Landscape (client)

The Landscape client is a `systemd` unit running on [Landscape](ref::landscape)-managed Ubuntu machines. It sends information about the system to the Landscape server. The server, in turn, sends instructions that the client executes.

In WSL, there is one Landscape client inside every Ubuntu WSL distro. The Landscape client comes pre-installed in your distro as part of the package `landscape-client`, but it must be configured before it can start running.

> See more: [Ubuntu manuals | Landscape client](https://manpages.ubuntu.com/manpages/noble/man1/landscape-client.1.html)

UP4W will configure all Ubuntu WSL distros for you, so you don't need to configure each WSL instance separately; you specify the configuration once and UP4W will distribute it to every distro.

> See more: [How to install and configure UP4W](howto::configure-up4w)

You can see the status of the Landscape client in any particular Ubuntu WSL instance by starting a shell in that instance and running:
```bash
systemctl status landscape-client.service
```
