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

distro
distribution
    Distro is short for a Linux distribution. Distros that have been downloaded
    and installed can then be launched as WSL instances.

instance
    A Linux distribution that has been launched through WSL. WSL can be used to
    launch multiple instances of different Linux distributions, multiple
    instances of different versions of the same Linux distribution, and two or more
    instances of the same Linux distribution if they exist on the filesystem
    with unique names.

Landscape
    Landscape is a systems management tool for Ubuntu. See the [Landscape
    website](https://ubuntu.com/landscape) and [Landscape
    documentation](https://documentation.ubuntu.com/landscape/).

Landscape client
    Landscape client is a systemd unit running inside every Ubuntu WSL
    instance. The Landscape client comes pre-installed in your distro as part
    of the package landscape-client. It sends information about the system to
    the Landscape server. The server, in turn, sends instructions that the
    client executes.

Landscape dashboard
    The Landscape dashboard is the browser-based GUI interface for Landscape
    where WSL instances can be managed.

Landscape server
    The Landscape server is used for the centralized management of remote
    Windows clients and the WSL instances that they host.

legacy distro
    A distro that uses the old format for packaging and distributing WSL
    distributions, based on msix/appx.

modern distro
    A distro that uses the new tar-based format for packaging and distributing
    WSL distributions.

remote development
    Developing in a WSL instance from a native Windows app, like Visual Studio Code.

session
    Launching an instance creates a terminal session for that instance. You can
    create multiple, parallel sessions for the same instance.

Ubuntu
    Ubuntu is a free, open-source operating system, and one of the most popular
    Linux distributions in the world.

Ubuntu Pro
    Ubuntu Pro is a subscription service offered by Canonical, which offers
    enhanced security and support.
    See the [Ubuntu Pro website](https://ubuntu.com/pro).

Ubuntu Pro client
    The Ubuntu Pro client runs within each instance of Ubuntu on WSL and
    enables Ubuntu Pro services. Can be used to Pro-attach an instance.
    See the [Ubuntu Pro Client documentation](https://canonical-ubuntu-pro-client.readthedocs-hosted.com/).

Ubuntu Pro for WSL
    Ubuntu Pro for WSL is an application for automating Pro-attachment and
    configuring Landscape-enrollment.

Ubuntu Pro token
    A unique token for accessing Pro-subscriber benefits.

Windows agent
    Windows agent is Pro for WSL's central hub that communicates and
    coordinates its various components.

Windows registry
    A hierarchical database for storing settings on Windows.

WSL
Windows Subsystem for Linux
    A virtualization layer for running Linux distributions on Windows.
    See the [Microsoft WSL documentation](https://learn.microsoft.com/en-us/windows/wsl/).

wsl.exe
    A native Windows executable for installing and managing WSL instances.
    See the [WSL command reference](https://learn.microsoft.com/en-us/windows/wsl/basic-commands).

WSL Pro service
    WSL Pro service is a bridge between the Windows agent and Ubuntu WSL
    instances. The Windows agent running on the Windows host sends commands
    that the WSL Pro Service executes, such as pro-attaching or configuring the
    Landscape client.

WSL profile
    A set of configurations defined in Landscape for deploying pre-configured
    WSL instances.

WSL version
    WSL is available in two version: WSL 1 and WSL 2. WSL 2 is the latest,
    recommended version.
```

