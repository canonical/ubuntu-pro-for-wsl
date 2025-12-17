---
myst:
  html_meta:
    "description lang=en":
      "Release notes for Ubuntu on WSL and Ubuntu Pro for WSL."
---

(ref::release-notes)=
# Release notes for Ubuntu on WSL

This page includes:

* [Release notes for LTS versions of Ubuntu on WSL](ref::ubuntu-wsl-lts-releases)
* [Release notes for interim releases of Ubuntu on WSL](ref::ubuntu-wsl-interim-releases)
* [Release notes for the Ubuntu Pro for WSL application](ref::up4w-releases)

(ref::upgrade-instructions)=
## Upgrade instructions for Ubuntu on WSL

The default Ubuntu distro for WSL (`wsl --install Ubuntu`) always ships the
latest stable LTS release, which can be upgraded once the first point release
of a new LTS is available:

```{code-block} text
$ sudo apt update && sudo apt full-upgrade 
$ do-release-upgrade
```

By default, Ubuntu distros downloaded as explicitly numbered releases (`wsl
--install Ubuntu-24.04`) will not be upgraded in this manner, unless the
following configuration change is made:

```{code-block} diff
:caption: /etc/update-manager/release-upgrades
:class: no-copy
- Prompt=never
+ Prompt=normal
```

(ref::latest-supported-releases)=
## Latest supported releases

```{admonition} LTS is recommended for Ubuntu on WSL
:class: important
We recommend LTS versions of Ubuntu on WSL, which receive standard support for
five years.

You can try interim releases of Ubuntu but they are not recommended for
production.
```

(ref::ubuntu-wsl-lts-releases)=
### Ubuntu on WSL distro (LTS)

#### Ubuntu 24.04 LTS (Noble Numbat)

##### Cloud-init support

`cloud-init` is the *industry standard* multi-distribution method for cross-platform cloud instance initialisation. It is supported across all major public cloud providers, provisioning systems for private cloud infrastructure, and bare-metal installations.

With `cloud-init` on WSL you can now automatically and reproducibly configure your WSL instances on first boot. Make the first steps with [this guide](https://documentation.ubuntu.com/wsl/stable/howto/cloud-init/).

##### New documentation

The documentation specific to [Ubuntu on WSL is available on Read the Docs](https://documentation.ubuntu.com/wsl). This evolving project is regularly updated with new content about Ubuntu’s specifics on WSL.

##### Reduced footprint

Experience faster download and installation times with 24.04, with a 200MB reduction in image size.

##### Systemd by default

systemd is now enabled by default even when the instance is launched directly from a terminal with the wsl.exe command or from an imported root files system.

##### Point release changes

###### 24.04.3

* WSL setup: remove the `systemd-binfmt.service` override after WSL implemented a more robust binfmt protection mechanism.
([wsl-setup/#11](https://github.com/ubuntu/wsl-setup/issues/11))

* WSL setup: Fix an issue when the Windows user name contains non-ASCII characters
([wsl-setup/#23](https://github.com/ubuntu/wsl-setup/issues/23))

###### 24.04.2

* WSL setup: Adapt to new Microsoft package format
([2091293](https://bugs.launchpad.net/bugs/2091293))

* Cloud init: Deliver warnings via MotD if cloud-init doesn’t succeed
([2080223](https://bugs.launchpad.net/bugs/2080223)).

###### 24.04.1

* WSL setup: Override cloud-init default_user configuration for Ubuntu distros to
prevent creation of a default user which confused WSLg
([2065349](https://bugs.launchpad.net/bugs/2065349))

(ref::up4w-releases)=
### Ubuntu Pro for WSL

#### Version 1.0

The first public release of Pro for WSL introduces turnkey security maintenance
and enterprise support for Ubuntu 24.04 LTS running on WSL, and enables
the effective management of Ubuntu on WSL by system administrators.

##### Automatic Pro attachment of Ubuntu instances

Once installed and configured with a Pro token, Pro for WSL runs in the
background of the Windows machine, automatically detecting and Pro-attaching
instances of Ubuntu on WSL.

##### Landscape integration for remote management of WSL

Landscape can register Windows machines in which the Pro for WSL application
has been configured to connect to the Landscape server.

Using the Landscape dashboard or API, system administrators can then monitor,
secure, and provision instances of Ubuntu on WSL across fleets of Windows machines.

##### Multiple configuration methods

Pro for WSL offers two approaches to configure Pro-attachment and Landscape-integration:

* The **Windows registry** is the best option for system administrators who need to develop
scalable deployment flows for over 5 Windows machines with an enterprise Pro subscription.
* The **Graphical application** is suitable for individual users who want to secure their
own device with a free Pro subscription and the free Pro for WSL app. 

##### Comprehensive documentation

Supporting documentation is available for [getting started using Pro for
WSL](howto::up4w) and [configuring a Pro subscription](howto::config-up4w). An
[architectural overview](../explanation/ref-arch-explanation) of Pro for WSL is
also provided.

### Previous LTS distro releases

```{dropdown} Ubuntu 22.04 LTS (Jammy Jellyfish)

#### 22.04

* Various bug fixes relating to the installer, UI and profile page

##### Point release changes

###### 22.04.4, 22.04.5

* None

###### 22.04.3

* Fix for get_ppid() not working on WSL

###### 22.04.2

* Cherry-pick upstream patch to use more portable alignment to resolve failure to execute on WSL 1.

###### 22.04.1

* Fixes and improvements to the store listing, slide show and release upgrader policy.

```

(ref::ubuntu-wsl-interim-releases)=
### Interim distro releases


```{dropdown} Click to expand for 25.10 (Questing Quokka), and more...
#### Ubuntu 25.10 (Questing Quokka)

* None

#### Ubuntu 25.04 (Plucky Puffin)

* Published WSL images have moved to
[cdimage.ubuntu.com](https://cdimage.ubuntu.com/ubuntu-wsl/) (previously on
[cloud-images.ubuntu.com](http://cloud-images.ubuntu.com/)).

#### Ubuntu 24.10 (Oracular Oriole)

* None

```

