(ref::landscape-client)=
# Landscape (client)
> See more: [Landscape](ref::landscape)

The Landscape client is a `systemd` unit running on Landscape-managed Ubuntu machines. It sends information about the system to the Landscape server. The server, in turn, sends instructions that the client executes.

In WSL, there is one Landscape client inside every Ubuntu WSL distro. The Landscape client comes pre-installed in your distro as part of the package `landscape-client`.

> See more: [Ubuntu manuals | Landscape client](https://manpages.ubuntu.com/manpages/noble/man1/landscape-client.1.html)

The client must be configured to communicate with the server. With UP4W, you don't need to configure each WSL instance separately; you need to specify the configuration only once, and UP4W will distribute it to every distro.

> See more: [How to set up Ubuntu Pro for Windows](howto::configure-up4w)

You can see the status of the Landscape client in any particular Ubuntu WSL instance by starting a shell in that instance and running:
```bash
systemctl status landscape-client.service
```
