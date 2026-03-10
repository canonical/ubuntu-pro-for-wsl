---
myst:
  html_meta:
    description: "Glossary of technical terms for Ubuntu on WSL."
---

# Glossary for Ubuntu on WSL

Overview of technical terms used in the documentation.

```{tip}
Think a term is missing and should be included?

You can [edit this glossary](https://github.com/canonical/ubuntu-pro-for-wsl/edit/main/docs/reference/glossary.md) on GitHub.
```

```{glossary}
:sorted:

cloud-init
    Cloud-init is used to initalize and provision WSL instances with specific
    configurations that are applied on initial booting of an instance.
    See the [cloud-init documentation](https://cloudinit.readthedocs.io/).

    Related topic(s): {term}`instance`, {term}`WSL`

distro
distribution
    A packaged Linux environment that can be downloaded
    and installed for launching as a WSL instance.

    Related topic(s): {term}`instance`, {term}`legacy distro`, {term}`modern distro`, {term}`WSL`

instance
    Most commonly refers to a Linux distribution that has been launched through
    WSL. In [other
    documentation](https://learn.microsoft.com/en-us/windows/wsl/),
    distribution and instance are sometimes used interchangeably. In the
    Landscape dashboard, "instance" refers to a Windows machine that runs WSL.

    Related topic(s): {term}`distro`, {term}`session`, {term}`WSL`

Landscape
    A systems management tool for Ubuntu. See the [Landscape
    website](https://ubuntu.com/landscape) and [Landscape
    documentation](https://documentation.ubuntu.com/landscape/).

    Related topic(s): {term}`Landscape client`, {term}`Landscape dashboard`, {term}`Landscape server`

Landscape client
    A systemd unit running inside every instance of Ubuntu on WSL. The
    Landscape client comes pre-installed in your distro as part of the package
    landscape-client. It sends information about the system to the Landscape
    server. The server, in turn, sends instructions that the client executes.

    Related topic(s): {term}`Landscape`, {term}`Landscape server`, {term}`Ubuntu`, {term}`WSL`

Landscape dashboard
    A browser-based GUI interface for Landscape where WSL instances can be
    managed.

    Related topic(s): {term}`instance`, {term}`Landscape`, {term}`WSL profile`

Landscape server
    Tool used for the centralized management of remote Windows clients and the
    WSL instances that they host.

    Related topic(s): {term}`Landscape`, {term}`Landscape client`, {term}`Windows agent`, {term}`WSL`

legacy distro
    A distro that uses the old format for packaging and distributing WSL
    distributions, based on msix/appx.

    Related topic(s): {term}`distro`, {term}`modern distro`, {term}`WSL`

modern distro
    A distro that uses the new tar-based format for packaging and distributing
    WSL distributions.

    Related topic(s): {term}`distro`, {term}`legacy distro`, {term}`WSL`

Pro-attachment
    The process of an Ubuntu client getting attached to an Ubuntu Pro subscription.

    Related topic(s): {term}`Ubuntu Pro`

remote development
    Developing in a WSL instance from a native Windows app, like Visual Studio Code.

    Related topic(s): {term}`instance`, {term}`WSL`

session
    Launching an instance creates a terminal session for that instance. You can
    create multiple, parallel sessions for the same instance.

    Related topic(s): {term}`instance`

Ubuntu
    A free, open-source operating system, and one of the most popular Linux
    distributions in the world.

    Related topic(s): {term}`Ubuntu Pro`, {term}`WSL`

Ubuntu Pro
    A subscription service offered by Canonical, which offers enhanced security
    and support. See the [Ubuntu Pro website](https://ubuntu.com/pro).

    Related topic(s): {term}`Ubuntu`, {term}`Ubuntu Pro client`, {term}`Ubuntu Pro token`, {term}`Ubuntu Pro for WSL`

Ubuntu Pro client
    A tool installed within each instance of Ubuntu on WSL that enables Ubuntu
    Pro services. Can be used to Pro-attach an instance. See the [Ubuntu Pro
    Client
    documentation](https://canonical-ubuntu-pro-client.readthedocs-hosted.com/).

    Related topic(s): {term}`Ubuntu Pro`, {term}`Ubuntu Pro for WSL`

Ubuntu Pro for WSL
Pro for WSL
    A Windows application for automating Pro-attachment and configuring
    Landscape-enrollment.

    Related topic(s): {term}`Ubuntu Pro`, {term}`Windows agent`, {term}`WSL Pro service`

Ubuntu Pro token
    A unique token for accessing Pro-subscriber benefits.

    Related topic(s): {term}`Ubuntu Pro`, {term}`Ubuntu Pro client`

Windows agent
    Pro for WSL's central hub that communicates and coordinates its various
    components.

    Related topic(s): {term}`Ubuntu Pro for WSL`, {term}`WSL Pro service`

Windows registry
    A hierarchical database for storing settings on Windows.

    Related topic(s): {term}`Windows agent`

WSL
Windows Subsystem for Linux
    A virtualization layer for running Linux distributions on Windows.
    See the [Microsoft WSL documentation](https://learn.microsoft.com/en-us/windows/wsl/).

    Related topic(s): {term}`instance`, {term}`wsl.exe`, {term}`WSL version`

wsl.exe
    A native Windows executable for installing WSL distributions and managing
    WSL instances. See the [WSL command
    reference](https://learn.microsoft.com/en-us/windows/wsl/basic-commands).

    Related topic(s): {term}`distro`, {term}`instance`, {term}`WSL`,

WSL Pro service
    A bridge between the Windows agent and instances of Ubuntu on WSL. The
    Windows agent running on the Windows host sends commands that the WSL Pro
    Service executes, such as pro-attaching or configuring the Landscape
    client.

    Related topic(s): {term}`Windows agent`, {term}`Ubuntu Pro client`

WSL profile
    A set of configurations defined in Landscape for deploying pre-configured
    WSL instances.

    Related topic(s): {term}`Landscape`, {term}`Landscape dashboard`

WSL version
    WSL is available in two version: WSL 1 and WSL 2. WSL 2 is the latest,
    recommended version.

    Related topic(s): {term}`WSL`
```

