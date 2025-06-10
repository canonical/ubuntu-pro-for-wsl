---
myst:
  html_meta:
    "description lang=en":
      "Install the latest version of Ubuntu on WSL using different methods."
---

(howto::install-ubuntu-wsl)=
# Install Ubuntu on WSL2

## What you will learn

* How to enable and install WSL on Windows
* How to install Ubuntu 24.04 LTS using the Microsoft Store or WSL commands in the terminal
* How to start Ubuntu instances

## What you will need

* Windows 10 or 11 running on either a physical device or virtual machine 
* All of the latest Windows updates installed

## Install WSL and run the default Ubuntu distro

To install WSL, open PowerShell as an Administrator and run:

```{code-block} text
> wsl --install
```

This installs both WSL and the default distro for WSL, which is the latest LTS version of Ubuntu.

## Install specific versions of Ubuntu on WSL

There are multiple ways of installing Ubuntu distros on WSL.
The best method depends on your specific requirements.

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

```{important}
At time of writing, Ubuntu 24.04 LTS and later versions are download in [WSL's
new tar-based format](https://ubuntu.com/blog/ubuntu-wsl-new-format-available).
Earlier Ubuntu versions are currently downloaded in the old format. The new format
requires WSL 2.4.10 or higher.
```

### Method 2: Download and install from the Ubuntu archive

Ubuntu images for WSL can be downloaded directly from [ubuntu.com/wsl](https://ubuntu.com/desktop/wsl).

The image has a `.wsl` extension and can be installed in two ways:

1. Double-clicking the downloaded file
2. Running `wsl --install --from-file <image>.wsl` in the download directory

This method has advantages in some contexts:

* Access to the Microsoft Store is not required
* Images can be self-hosted on an internal network
* Custom installations can be created by modifying the image

> Read our [blog post](https://ubuntu.com/blog/ubuntu-wsl-new-format-available)
about the new format and [Microsoft's guide on building custom WSL
distros](https://learn.microsoft.com/en-us/windows/wsl/build-custom-distro).


### Method 3: Install from the Microsoft Store

Find the Ubuntu distribution that you want in the Microsoft Store and click **Get**.

![Installation page for Ubuntu 24.04 LTS in the Microsoft store.](assets/install-ubuntu-wsl2/choose-distribution.png)

Once installed, you can either launch Ubuntu 24.04 LTS directly from the Microsoft Store or search for Ubuntu in your Windows search bar.

![Search results for Ubuntu 24.04 LTS in Windows search bar.](assets/install-ubuntu-wsl2/search-ubuntu-windows.png)

## Starting an Ubuntu instance

During installation of an Ubuntu distro, you are asked to create a username and password specific to that instance.
This also starts an Ubuntu session and logs you in.

After installation, you can open Ubuntu instances by:

* Searching for them in the Window's search bar
* Opening the dropdown in [Windows Terminal](https://github.com/microsoft/terminal?tab=readme-ov-file#installing-and-running-windows-terminal)
* Running the `wsl -d <Distro>` command in PowerShell

At any point, you can list the Ubuntu distros that you can start with `wsl -l -v`.

## Starting an instance in the right directory

By default, if you open Ubuntu using the Windows search bar or the Windows Terminal dropdown,
the instance starts in the Ubuntu home directory.

When starting an instance from the terminal, the command run determines the starting directory.

### Start Ubuntu in the current Windows directory from the terminal

When you open PowerShell, the working Windows directory is `C:\Users\username`.

Run `wsl -d <Distro>` to start an Ubuntu session in that directory. The prompt
will indicate that the Windows `C:` drive is mounted to Ubuntu and that you are
in the Windows home directory:

```{code-block} text
:class: no-copy
username@pc:/mnt/c/Users/username$
```

### Start Ubuntu in the Ubuntu home directory from the terminal

When in a directory in the mounted `C:` drive, you can change to the Ubuntu
home directory with:

```{code-block} text
username@pc:/mnt/c/Users/username$ cd ~
```

To skip this step, and start an instance from PowerShell with Ubuntu home as
the working directory, run:

```{code-block} text
> wsl ~ -d Ubuntu
```

````{tip}
For the **default Ubuntu distro only**, this command can be shortened further to:

```{code-block} text
> wsl ~
```

````

## Enjoy Ubuntu on WSL

In this guide, we’ve shown you how to install Ubuntu WSL using different methods.

We hope you enjoy working with Ubuntu in WSL. Don’t forget to check out [our blog](https://ubuntu.com/blog) for the latest news on all things Ubuntu.

## Further Reading

* [Read a detailed reference on WSL terminal commands](https://learn.microsoft.com/en-us/windows/wsl/basic-commands)
* [Setting up WSL for Data Science](https://ubuntu.com/blog/upgrade-data-science-workflows-ubuntu-wsl)
* [Whitepaper: Ubuntu WSL for Data Scientists](https://ubuntu.com/engage/ubuntu-wsl-for-data-scientists)
* [Microsoft WSL Documentation](https://learn.microsoft.com/en-us/windows/wsl/)
* [Ask Ubuntu](https://askubuntu.com/)
