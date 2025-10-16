---
myst:
  html_meta:
    "description lang=en":
      "Install the latest version of Ubuntu on WSL using different methods."
---

(howto::install-ubuntu-wsl)=
# Install Ubuntu on WSL 2

## What you will learn

* How to install and enable WSL on Windows
* How to install Ubuntu 24.04 LTS using the terminal or the Microsoft Store
* How to start Ubuntu instances after they have been installed

## What you will need

* Windows 11 (recommended) or Windows 10 with minimum version 21H2 on a physical machine

```{include} ../includes/virtualisation_requirements.txt
    :start-after: <!-- Include start virtualisation requirements -->
    :end-before: <!-- Include end virtualisation requirements -->
```

## Install and enable WSL

To install Ubuntu using any method, you first need to install and enable WSL on
your Windows machine.

Open PowerShell and run:

```{code-block} text
> wsl --install
```

You may be prompted to grant permission to continue the installation.

You may need to reboot your machine before installing and running any Ubuntu distro.

:::{dropdown} Variation in WSL installation behaviour
:color: primary
:icon: question

The effect of `wsl --install` may vary depending on your system. After
installation, a reboot is usually required before WSL distros can be installed.
If those WSL components are already enabled, a reboot may not be required and
an installation of the default Ubuntu distribution will be attempted.

Read Microsoft’s documentation for more information on
[installing WSL](https://learn.microsoft.com/en-us/windows/wsl/install).
:::

## Install specific versions of Ubuntu on WSL

There are multiple ways of installing Ubuntu distros on WSL.
The best method depends on your specific requirements.

:::{dropdown} WSL versions
:open:
:color: secondary
:icon: pencil

WSL 2 is the default [WSL architecture](explanation::wsl-version) in recent
versions of Windows. Our documentation assumes that you are installing Ubuntu
on WSL 2.

Read more about the [differences between WSL versions](explanation::wsl-version).
:::

### Method 1: Install Ubuntu from the terminal

In a PowerShell terminal, run `wsl --list --online` to see a list of all available distros and versions:

```{code-block} text
:class: no-copy
The following is a list of valid distributions that can be installed.
Install using 'wsl --install <Distro>'.

  NAME                                   FRIENDLY NAME
  AlmaLinux-8                            AlmaLinux OS 8
  ...                                    ...
  Ubuntu                                 Ubuntu
  Ubuntu-24.04                           Ubuntu 24.04 LTS
  archlinux                              Arch Linux
  kali-linux                             Kali Linux Rolling
  ...                                    ...
  Ubuntu-18.04                           Ubuntu 18.04 LTS
  Ubuntu-20.04                           Ubuntu 20.04 LTS
  Ubuntu-22.04                           Ubuntu 22.04 LTS
...

```

Install a specific Ubuntu distro using a NAME from the output:

```{code-block} text
> wsl --install Ubuntu-24.04
```

```{dropdown} Using the new WSL distro format
:open:
:color: info
:icon: info

At time of writing, Ubuntu 24.04 LTS and later versions are downloaded in [WSL's
new tar-based format](https://ubuntu.com/blog/ubuntu-wsl-new-format-available).
Earlier Ubuntu versions are currently downloaded in the old format. The new format
requires WSL 2.4.10 or higher.
```

### Method 2: Download and install from the Ubuntu archive

Ubuntu images for WSL can be downloaded directly from
[releases.ubuntu.com](https://releases.ubuntu.com).

To download Ubuntu 24.04 LTS (Noble Numbat), go to
[releases.ubuntu.com/noble](https://releases.ubuntu.com/noble) and select the WSL
image.

The image has a `.wsl` extension and can be installed in two ways:

1. Double-clicking the downloaded file
2. Running `wsl --install --from-file <image>.wsl` in the download directory

You do not need access to the Microsoft Store to use this installation method
and the images can be self-hosted on an internal network.

The downloaded image can also be customised, as described in our [image
customisation guide](custom-ubuntu-distro.md).

> Read our [blog post](https://ubuntu.com/blog/ubuntu-wsl-new-format-available)
about the new format and [Microsoft's guide on building custom WSL
distros](https://learn.microsoft.com/en-us/windows/wsl/build-custom-distro).


### Method 3: Install from the Microsoft Store

If you prefer a graphical method of installation, open the Microsoft Store on
your Windows machine and search for "Ubuntu".

Go to the page of an available Ubuntu distribution and click {guilabel}`Get` to
start the installation.

## Starting an Ubuntu instance

During installation of an Ubuntu distro on WSL, you are asked to create a
username and password specific to that instance.
This also starts an Ubuntu session and logs you in.

After installation, you can open Ubuntu instances by:

* Running the `wsl -d <Distro>` command in PowerShell
* Opening the dropdown in [Windows Terminal](https://github.com/microsoft/terminal?tab=readme-ov-file#installing-and-running-windows-terminal)
* Searching for them in the Window's search bar

At any point, you can list the Ubuntu distros that you can start with `wsl -l -v`.

## Starting an instance in the right directory

By default, if you open Ubuntu using the Windows search bar or the Windows Terminal dropdown,
the instance starts in the Ubuntu home directory.

When starting an instance from the terminal, the specific command that you run
determines the starting directory.

### Start Ubuntu in the current Windows directory from the terminal

When you open PowerShell, the working Windows directory is `C:\Users\username`.

Run `wsl -d <Distro>` to start an Ubuntu session in that directory. The prompt
will indicate that the Windows `C:` drive is mounted to Ubuntu and that you are
in the Windows home directory:

```{code-block} text
:class: no-copy
<ubuntu-username>@<hostname>:/mnt/c/Users/<windows-username>$
```

### Start Ubuntu in the Ubuntu home directory from the terminal

When in a directory in the mounted `C:` drive, you can change to the Ubuntu
home directory with:

```{code-block} text
:caption: /mnt/c/Users/\<windows-username\>
$ cd ~
```

To skip this step, and start an instance from PowerShell with Ubuntu home as
the working directory, run:

```{code-block} text
> wsl ~ -d Ubuntu
```

:::{dropdown} Tip
:open:
:color: success
:icon: light-bulb

For the **default WSL distro**, this command can be shortened further to:

```{code-block} text
> wsl ~
```

The default distro for WSL is Ubuntu, although [this can be
configured](https://learn.microsoft.com/en-us/windows/wsl/basic-commands#set-default-linux-distribution).

:::

## Enjoy Ubuntu on WSL

In this guide, we’ve shown you how to install Ubuntu WSL using different methods.

We hope you enjoy working with Ubuntu in WSL. Don’t forget to check out [our blog](https://ubuntu.com/blog) for the latest news on all things Ubuntu.

## Further Reading

* [Read a detailed reference on WSL terminal commands](https://learn.microsoft.com/en-us/windows/wsl/basic-commands)
* [Setting up WSL for Data Science](https://ubuntu.com/blog/upgrade-data-science-workflows-ubuntu-wsl)
* [Whitepaper: Ubuntu WSL for Data Scientists](https://ubuntu.com/engage/ubuntu-wsl-for-data-scientists)
* [Microsoft WSL Documentation](https://learn.microsoft.com/en-us/windows/wsl/)
* [Ask Ubuntu](https://askubuntu.com/)
