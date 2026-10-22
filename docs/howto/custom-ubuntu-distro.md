---
myst:
  html_meta:
    "description lang=en":
      "Customise your own Ubuntu distro for WSL"
---

(howto::custom-distro)=
# Customise an Ubuntu distro for WSL

This guide shows you how to download an existing Ubuntu release for WSL and
customise  it to your preference, using the new {term}`tar-based image format <modern distro>`.

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

* Windows 11 (recommended) or Windows 10 with minimum version 21H2 on a physical machine
* The latest version of WSL (2.4.8 or higher) installed and enabled
* An existing Ubuntu distro installed on WSL 2

WSL will be used to extract the rootfs, edit the configuration files and manage packages.

## Download an Ubuntu release for WSL

First, get Ubuntu 26.04 LTS (Resolute Raccoon) by visiting
[releases.ubuntu.com/resolute](https://releases.ubuntu.com/resolute) and downloading
the WSL image.

The downloaded file should have a `.wsl` extension.

For the next step, it is assumed that this file is located in the default
`Downloads` directory on Windows.

## Extract the rootfs

Open a PowerShell terminal and run an instance of Ubuntu on WSL, for example:

```{terminal}
:prompt: PS>
:copy:

wsl ~ -d Ubuntu-26.04
```

From this Ubuntu environment, move the tarball from the Downloads directory
into the Ubuntu instance, and change the extension from `.wsl` to `.tar`, using
this command:

```{terminal}
:prompt: $
:copy:

mv /mnt/c/Users/<username>/Downloads/ubuntu-26.04.1-wsl-amd64.wsl ./ubuntu-26.04.1-wsl-amd64.tar
```

````{admonition} Additional step if the WSL image has a .gz extension
:class: tip
If the image you downloaded has a filename that ends in `.wsl.gz`, rather than
`.wsl`, run the following command before changing the extension to `.tar`:

```{terminal}
:prompt: $
:copy:

gunzip /mnt/c/Users/<username>/Downloads/ubuntu-26.04.1-wsl-amd64.wsl.gz
```
````

In the home directory (`~`), create a directory to store the rootfs of your
custom distro:

```{terminal}
:prompt: $
:copy:

mkdir myNewUbuntu
```

Extract the rootfs into that new directory:

```{terminal}
:prompt: $
:copy:

sudo tar -xpf ubuntu-26.04.1-wsl-amd64.tar -C myNewUbuntu --numeric-owner --absolute-names
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

```{terminal}
:prompt: $
:copy:

sudo vim ~/myNewUbuntu/etc/wsl-distribution.conf
```

Change the name of your distro and the name of its icon:

```{code-block} diff
:linenos:
:caption: ~/myNewUbuntu/etc/wsl-distribution.conf
[oobe]
command = /usr/lib/wsl/wsl-setup
defaultUid = 1000
- defaultName = Ubuntu-26.04
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

```{terminal}
:prompt: $
:copy:

sudo apt update
```

```{terminal}
:prompt: $
:copy:

sudo apt install imagemagick
```

Once installed, create a grayscale icon using its `convert` command:

```{terminal}
:prompt: $
:copy:

convert ~/myNewUbuntu/usr/share/wsl/input.ico -colorspace Gray ~/myNewUbuntu/usr/share/wsl/myIcon.ico
```

### Boot configuration file

You can also customise which settings are applied to the distro on boot:

```{terminal}
:prompt: $
:copy:

sudo vim ~/myNewUbuntu/etc/wsl.conf
```

For the purpose of this guide, we will keep the default boot settings.

```ini
[boot]
systemd=true
```

## Customise the out-of-the-box experience

The OOBE is a relatively complex script, handling aspects of the user
experience, including the user prompt and log messages during provisioning.

Make a minor change to the following echo command in the `myNewUbuntu/usr/lib/wsl/wsl-setup` file:

```{code-block} diff
:linenos:
:caption: ~/myNewUbuntu/usr/lib/wsl/wsl-setup

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
:caption: ~/myNewUbuntu/usr/share/wsl/terminal-profile.json
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

From the home directory (`~`) of your Ubuntu instance, mount the necessary file
systems:

```{terminal}
:prompt: $
:copy:

sudo mount -t proc /proc myNewUbuntu/proc
```

```{terminal}
:prompt: $
:copy:

sudo mount --rbind --make-rslave /sys myNewUbuntu/sys
```

```{terminal}
:prompt: $
:copy:

sudo mount --rbind --make-rslave /dev myNewUbuntu/dev
```

```{terminal}
:prompt: $
:copy:

sudo mount --rbind --make-rslave /run myNewUbuntu/run
```

Then `chroot` into the root directory of your custom distro and open a bash shell:

```{terminal}
:prompt: $
:copy:

sudo chroot ~/myNewUbuntu /bin/bash
```

If successful, you will see a prompt for the root user:

```{code-block} text
:class: no-copybutton
root@<hostname>:/#
```

You can now manage packages for your custom distro. For example, `btop` is a
terminal-based resource monitor that is not installed on Ubuntu by default. To
make it available in your custom distro, run the following as the root user:

```{terminal}
:user: root
:host: <hostname>
:dir: /
:copy:

apt update
```

```{terminal}
:user: root
:host: <hostname>
:dir: /
:copy:

apt upgrade -y
```

```{terminal}
:user: root
:host: <hostname>
:dir: /
:copy:

apt install btop
```

When you're finished managing packages, exit the `chroot` environment:

```{terminal}
:user: root
:host: <hostname>
:dir: /
:copy:

exit
```

After exiting the `chroot` environment it's useful to cleanup the mount points.

```{terminal}
:prompt: $
:copy:

sudo umount -R myNewUbuntu/proc
```

```{terminal}
:prompt: $
:copy:

sudo umount -R myNewUbuntu/sys
```

```{terminal}
:prompt: $
:copy:

sudo umount -R myNewUbuntu/dev
```

```{terminal}
:prompt: $
:copy:

sudo umount -R myNewUbuntu/run
```

## Create the final distro file

Next, you will create the installable version of your custom distro.

Change into the distro directory:

```{terminal}
:prompt: $
:copy:

cd ~/myNewUbuntu
```

Inside that directory, compress the rootfs, outputting the tarball to the parent directory:

```{terminal}
:prompt: $
:copy:

sudo tar --numeric-owner --absolute-names --one-file-system -czvf ../myNewUbuntu.tar .
```

Then `cd` into the home directory and change the file extension of the distro
file:

```{terminal}
:prompt: $
:copy:

cd ~
```

```{terminal}
:prompt: $
:copy:

mv myNewUbuntu.tar myNewUbuntu.wsl
```

Finally, move the distro to a Windows directory:

```{terminal}
:prompt: $
:copy:

sudo mv myNewUbuntu.wsl /mnt/c/Users/<username>/Downloads/
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
