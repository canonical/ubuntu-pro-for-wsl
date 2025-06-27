---
myst:
  html_meta:
    "description lang=en":
      "Customise your own Ubuntu distro for WSL"
---

(howto::custom-distro)=
# Customise an Ubuntu distro for WSL

This guide shows you how to download an existing Ubuntu release for WSL and
customise  it to your preference, using the new tar-based image format.

## What you will learn

* How to download an Ubuntu distro and extract the rootfs on Windows
* How to edit the WSL distro configuration
* How to customise the out-of-the-box experience (OOBE)
* How to modify the Windows Terminal profile
* How to customise the packages available on startup
* How to create the final distro file

## What you will need

For convenience, this guide uses a single Windows machine for customising
and testing the distro.

To follow the guide, make sure you have:

* Windows 11 with WSL2 installed and enabled
* An existing Ubuntu distro installed on WSL

WSL will be used to extract the rootfs, edit the configuration files and manage packages.

## Download an Ubuntu release for WSL

First, download the latest release of Ubuntu on WSL:

> [Download Ubuntu on WSL](https://ubuntu.com/desktop/wsl)

The downloaded file should have a `.wsl` extension.

For the next step, it is assumed that this file is located in the default
`Downloads` directory on Windows.

## Extract the rootfs

Open a PowerShell terminal and run an instance of Ubuntu on WSL, for example:

```{code-block} text
> wsl ~ -d Ubuntu-24.04
```

From this Ubuntu environment, move the tarball from the Downloads directory
into the Ubuntu instance, and change the extension from `.wsl` to `.tar`, using
this command:

```{code-block} text
$ mv /mnt/c/Users/<yourusername>/Downloads/ubuntu-24.04.2-wsl-amd64.wsl ./ubuntu-24.04.2-wsl-amd64.tar
```

Create a directory to store the rootfs of your custom distro:

```{code-block} text
$ mkdir myNewUbuntu
```

Extract the rootfs into that directory:

```{code-block} text
$ sudo tar -xpf ubuntu-24.04.2-wsl-amd64.tar -C myNewUbuntu --numeric-owner --absolute-names
```

```{tip}
You must extract the rootfs in a Linux environment --- such as a WSL instance --- to prevent file system incompatibilities.
```

## Customise the distro

There are several files that can be edited to customise the Ubuntu distro for WSL.

### Editing WSL distro configuration files

There are two configuration files for the WSL distro that you are customising:

* `myNewUbuntu/etc/wsl-distribution.conf`
* `myNewUbuntu/etc/wsl.conf`

### Distribution configuration file

Open the configuration file:

```{code-block} text
$ vim myNewUbuntu/etc/wsl-distribution.conf
```

Change the name of your distro and the name of its icon:

```{code-block} diff
:linenos:
:caption: myNewUbuntu/etc/wsl-distribution.conf
[oobe]
command = /usr/lib/wsl/wsl-setup
defaultUid = 1000
- defaultName = Ubuntu-24.04
+ defaultName = my-new-ubuntu

[shortcut]
- icon = /usr/share/wsl/ubuntu.ico
+ icon = /usr/share/wsl/myIcon.ico

[windowsterminal]
ProfileTemplate = /usr/share/wsl/terminal-profile.json
```

Note the following options, which will be configured later:

* `command` determines the out-of-the-box experience (OOBE)
* `ProfileTemplate` affects the behaviour of the distro in Windows terminal

A quick way to test modifying the distro icon is to use `imagemagick`:

```{code-block} text
$ sudo apt update
$ sudo apt install imagemagick
```

Once installed, create a grayscale icon using its `convert` command:

```{code-block} text
$ convert myNewUbuntu/usr/share/wsl/input.ico -colorspace Gray myNewUbuntu/usr/share/wsl/myIcon.ico
```

### Boot configuration file

You can also customise which settings are applied to the distro on boot:

```{code-block} text
$ vim myNewUbuntu/etc/wsl.conf
```

For the purpose of this guide, we will keep the default boot settings.

```ini
[boot]
systemd=true
```

## Customise the out-of-the-box experience

The OOBE is a relatively complex script, handling aspects of the user
experience, including the user prompt and log messages during provisioning.

Make a minor change to the following echo command in `myNewUbuntu/usr/lib/wsl/wsl-setup`:

```{code-block} diff
:linenos:
:caption: myNewUbuntu/usr/lib/wsl/wsl-setup

#!/bin/bash
set -euo pipefail
...
...
echo "Provisioning the new WSL instance $(wslpath -am / | cut -d '/' -f 4)"
-echo "This might take a while..."
+echo "This will take a little longer..."
...
...
```

## Modify the Windows Terminal profile

Change the background colour to black, reduce its opacity and add a retro-style
terminal effect in `myNewUbuntu/usr/share/wsl/terminal-profile.json` :

```{code-block} diff
:caption: myNewUbuntu/usr/share/wsl/terminal-profile.json
{
    "profiles": [
        {
            "colorScheme": "Ubuntu",
+           "opacity": 90,
+           "experimental.retroTerminalEffect": true,
            "suppressApplicationTitle": true,
            "cursorShape": "filledBox",
            "font": {
                "face": "Ubuntu Mono",
                "size": 13
        }
    ],
    "schemes": [
        {
            "name": "Ubuntu",
-           "background": "#300A24",
+           "background": "#000000",
            ...
            ...
            "yellow": "#A2734C"
        }
    ]
}
```

## Install and remove packages

You can customise the packages available when your custom image is installed
using the `apt` package manager for Ubuntu.

From the home directory of your Ubuntu instance, mount the necessary file
systems:

```{code-block} text
$ sudo mount -t proc /proc myNewUbuntu/proc
$ sudo mount --rbind /sys myNewUbuntu/sys
$ sudo mount --rbind /dev myNewUbuntu/dev
$ sudo mount --rbind /run myNewUbuntu/run
```

Then `chroot` into the root directory of your custom distro and open a bash shell:

```{code-block} text
$ sudo chroot myNewDistro /bin/bash
```

If successful, you will see a prompt for the root user:

```{code-block} text
:class: no-copy
root@<your-machine>:/#
```

You can now manage packages for your custom distro. For example, `btop` is a
terminal-based resource monitor that is not installed on Ubuntu by default. To
make it available in your custom distro, run the following as the root user:

```{code-block} text
apt update
apt upgrade -y
apt install btop
```

When you're finished managing packages, exit the `chroot` environment:

```{code-block} text
exit
```

## Create the final distro file

Next, you will create the installable version of your custom distro.

Change into the distro directory:

```{code-block} text
:caption: /home/\<username>
$ cd myNewUbuntu
```

Then compress the rootfs, outputting the tarball to the parent directory:

```{code-block} text
:caption: /home/\<username>/myNewUbuntu
$ sudo tar -czvf --numeric-owner --absolute-names ../myNewUbuntu.tar .
```

Then `cd` into the home directory and change the file extension of the distro
file:

```text
$ cd ..
$ mv myNewUbuntu.tar myNewUbuntu.wsl
```

Finally, move the distro to a Windows directory:

```text
$ sudo mv myNewUbuntu.wsl /mnt/c/Users/<yourusername>/Downloads/
```

## Test your custom Ubuntu distro

Find the `.wsl` file in your downloads folder and double left-click it.

Your custom distro should install and launch automatically, with:

* ✅ A custom name
* ✅ A custom icon
* ✅ A custom theme
* ✅ A custom welcome message
* ✅ A custom package installed

When you need to launch it in future, you can find it in the Start menu
or in the Windows Terminal dropdown.

## Next steps

Now that you know how to make your own Ubuntu distro for WSL, you can customise
it to suit your personal needs or the requirements of your organisation.

You can automatically set up new instances of your custom images with
[cloud-init](howto::cloud-init).

With Ubuntu Pro for WSL, custom images can be deployed remotely to Windows
machines using Landscape.
