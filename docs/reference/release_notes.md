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

This application is pending an official release of its 1.0 version.


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


```{dropdown} Click to expand for 25.04 (Plucky Puffin), and more...
#### Ubuntu 25.04 (Plucky Puffin)

* Published WSL images have moved to
[cdimage.ubuntu.com](https://cdimage.ubuntu.com/ubuntu-wsl/) (previously on
[cloud-images.ubuntu.com](http://cloud-images.ubuntu.com/)).

#### Ubuntu 24.10 (Oracular Oriole)

* None

```

