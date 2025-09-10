---
myst:
  html_meta:
    "description lang=en":
      "The Ubuntu Pro for WSL applications consists of multiple components: a GUI front end, Landscape client, Ubuntu Pro client, Windows agent, and WSL Pro service."
---

(ref::glossary-up4w-components)=
# Glossary of Ubuntu Pro for WSL components

The architecture of UP4W and how its components integrate together is covered in our detailed [explanation article](../explanation/ref-arch-explanation.md).
This glossary includes concise descriptions of the components for reference.

(ref::up4w-gui)=
## GUI front end

UP4W has a small GUI that helps users provide an Ubuntu Pro token.

When the GUI starts, it attempts to establish a connection to the [UP4W Windows Agent](ref::up4w-windows-agent). If this fails, the agent is restarted. For troubleshooting purposes, you can restart the agent by first stopping the Windows process `ubuntu-pro-agent-launcher.exe` in Windows Task Manger or by issuing the following command in a PowerShell terminal:

```text
Stop-Process -Name ubuntu-pro-agent.exe
```

You can then launch the GUI to complete the restart.

(ref::ubuntu-pro-client)=
## Ubuntu Pro client

The Ubuntu Pro client is a command-line utility that manages the
different offerings of your Ubuntu Pro subscription. In UP4W, this executable
is used within each of the managed WSL distros to enable [Ubuntu
Pro](https://documentation.ubuntu.com/pro/) services within that distro.

This executable is provided as part of the `ubuntu-pro-client` package,
which comes pre-installed in Ubuntu WSL instances since Ubuntu 24.04 LTS.

(ref::up4w-windows-agent)=
## Windows agent

UP4W's Windows agent is a Windows application running in the background. It starts automatically when the user logs in to Windows. If it stops for any reason, it can be started by launching the UP4W GUI or running the executable from the terminal, optionally with `-vvv` for verbose logging:

```text
ubuntu-pro-agent.exe -vvv
```

The Windows agent is UP4W's central hub that communicates with all the components to coordinate them.

(ref::up4w-wsl-pro-service)=
## WSL Pro service

This is a `systemd` unit running inside every Ubuntu WSL instance. The [Windows agent](ref::up4w-windows-agent) running on the Windows host sends commands that the WSL Pro Service executes.

You can check the current status of the WSL Pro Service in any particular distro with:

```text
systemctl status wsl-pro.service
```

(ref::landscape-client)=
## Landscape client

```{admonition} Feature in development
:class: important
Landscape integration is an in-development feature of Ubuntu Pro for WSL.
```

The Landscape client is a `systemd` unit running inside every Ubuntu WSL instance.
In a future version of UP4W, it will be possible to connect WSL instances with a central Landscape server.
The instances will then send information about the system to the Landscape server. 
The server, in turn, can send instructions that the client executes.

The Landscape client comes pre-installed in your distro as part of the package `landscape-client`.

You can check the status of the Landscape client in any particular Ubuntu WSL instance by starting a shell in that instance and running:

```text
systemctl status landscape-client.service
```

