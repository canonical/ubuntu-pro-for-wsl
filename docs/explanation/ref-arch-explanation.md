---
myst:
  html_meta:
    "description lang=en":
      "The different components of Ubuntu Pro for WSL work together to support automatic securing of WSL instances and integration with remote management tools."
---

# Architecture of Ubuntu Pro for WSL

This page describes the different components of Pro for WSL and how they integrate
together to form the software architecture.

## The components of Ubuntu Pro for WSL

```{admonition} WSL architecture
:class: tip
WSL is maintained by Microsoft and its architecture is outside the scope of this page.

A good overview of WSL architecture is provided in [this blog from Microsoft](https://learn.microsoft.com/en-us/previous-versions/windows/desktop/cmdline/wsl-architectural-overview).
```

### Overview

Pro for WSL consists of some components that run on a Windows host
and others that run within instances of Ubuntu  WSL.

A user interacts with the Pro for WSL application. Pro for WSL then automatically
pro-attaches and Landscape-enrolls each new instance of Ubuntu WSL that is
created on the Windows host.

In an organisation, multiple users of Windows machines can create Ubuntu WSL
instances, which are secured by Ubuntu Pro and that can be managed by Landscape.

```{figure} ../diagrams/structurizr-SystemLandscape.png
:name: user-interact-arch
:alt: architecture diagram showing user interaction with Ubuntu Pro for WSL
:align: center
```

### Components on the Windows host

The Windows host is a single machine running the Windows OS. WSL instances can
be created and instanced on this host. The Pro for WSL application that is installed
on the Windows host consists of a GUI front end and an agent that runs in the
background.

A user enters a Pro token and Landscape configuration using the GUI. When the
GUI is launched it starts the Windows Agent, if it's not already running. The agent runs in the background
on the Windows host and manages communication with other components, including
the remote Landscape server and the Pro service running within each instance of
Ubuntu WSL. The agent is responsible for managing the state of instances and
acts as a bridge between those instances and Landscape. If the configuration
details are valid, all new instances will have Ubuntu Pro enabled and will be
able to communicate with the Landscape server.

```{figure} ../diagrams/structurizr-SystemContainers.png
:name: top-level-arch
:alt: architecture diagram showing coordinating role of agent
:align: center
```

It is possible to bypass the GUI and instead configure Pro for WSL using the Windows
registry. This may be the preferred option for those operating at scale. When
Pro for WSL is launched, a registry path is created that can be used to store a Pro
token and a Landscape configuration. A system administrator can use a remote
management solution like Intune to configure the registry on fleets of devices.

### Components on Ubuntu WSL instances

The WSL Pro service runs in each instance of Ubuntu WSL. From the host, the
Windows agent communicates with this service. This allows commands to be sent
from the host, which are then executed by the Pro service on each instance.
When an Ubuntu WSL instance is started, the WSL Pro service runs and queries
the Windows agent on the host for the status of the Pro subscription.
If the Pro token is valid, it is retrieved from the Windows agent and
passed to the Pro service running on Ubuntu WSL instances.
If not, Ubuntu Pro is disabled on the instances.

Pre-installed on each instance of Ubuntu WSL is an Ubuntu Pro client
and a Landscape client.
After a Pro token is provided, the Windows agent can send a
command to the Ubuntu Pro client to execute pro-attach on active instances.
Similarly, when a Landscape configuration is provided, the Windows agent
can send a command to configure the Landscape client in each instance.

The administrator of the Landscape server can also send commands to the agent
to deploy new instances or delete existing instances.

```{figure} ../diagrams/structurizr-Production.png
:name: production-arch
:alt: architecture diagram for production, with instances deployed from remote server
:align: center
```

Ubuntu WSL instances that are deployed at scale can be extensively customised.
The Landscape API can be used to automate the deployment of a custom rootfs. As
of Ubuntu 24.04 LTS, cloud-init is pre-installed on Ubuntu WSL instances, which
makes it possible to automate the configuration of instances created from that
release.

### Source code

Pro for WSL is open-source software. You can look at the code in the [GitHub
repo](https://github.com/canonical/ubuntu-pro-for-wsl).

The following technologies are used to build Pro for WSL:

* **Go**: Windows agent and WSL Pro service
* **Flutter**: GUI front end
* **gRPC**: communication between back end and front end

### Deployment and updates

The WSL Pro service, a component that runs inside instances of Ubuntu on WSL, is
distributed as a Debian package from the Ubuntu archive. It receives
automatic updates and upgrades that are typical for Ubuntu packages.
The Pro service comes pre-installed on the most recent LTS releases of Ubuntu on WSL.

The Windows components of Pro for WSL are distributed in a single
[MSIX](https://learn.microsoft.com/en-us/windows/msix/overview) package.
The package is available through the Microsoft Store, our download page, and as GitHub
Release assets. Together with the MSIX package, we also publish an
[`AppInstaller` file](https://learn.microsoft.com/en-us/windows/msix/app-installer/app-installer-file-overview)
that Windows treats as an installable file. The file is later used to track and
fetch updates for the Pro for WSL app, if it has not been installed from
the Microsoft Store. This a common scenario in many corporate environments,
where access to the Microsoft Store may be restricted.

On Windows 11, packages installed by any of these methods will receive automatic
updates, which is guaranteed by the operating system. In comparison, Windows 10
has limited support for automatic updates of MSIX packages that have been
installed directly. To ensure automatic updates on Windows 10, it is preferable
to install the Pro for WSL application through the Microsoft Store or
using the `AppInstaller` file that accompanies the MSIX package.
