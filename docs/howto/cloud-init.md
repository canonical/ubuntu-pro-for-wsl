---
myst:
  html_meta:
    "description lang=en":
      "Preconfigure Ubuntu on WSL using cloud-init to automate the creation of custom instances."
---

(howto::cloud-init)=
# Automatic setup of Ubuntu on WSL with cloud-init

Cloud-init is a cross-platform tool for provisioning cloud instances.
It is an industry standard and can now also be used to automatically setup instances of Ubuntu on WSL.

> See more:  [cloud-init official documentation](https://cloudinit.readthedocs.io/en/latest/index.html).

## What you will learn

- How to write cloud-config user data to a specific WSL instance.
- How to automatically set up a WSL instance with cloud-init.
- How to verify that cloud-init succeeded with the configuration supplied.

## What you will need

- Windows 11 with WSL2 already enabled

The guide assumes that you are using Ubuntu 24.04,
but Ubuntu 22.04 can also be used.

In the latest versions of WSL, installing a distro also launches the instance
and prompts the user through setup. Cloud-init will not provision an instance
that has already been set up in this way.

If you want to provision with cloud-init after a distro is installed, first
install the distro with the `--no-launch` flag. Alternatively, set up cloud-init
before you install the distro.

## Write the cloud-config file

Locate your Windows user home directory, which is typically `C:\Users\<YOUR_USER_NAME>`.

````{tip}
If you want to find your home directory, run the following in PowerShell:

```text
echo $env:USERPROFILE` in PowerShell
```
````

Inside your Windows user home directory, create a new folder named
`.cloud-init`, ensuring there is `.` at the start of the directory name. Inside
the new directory, create an empty file named `Ubuntu-24.04.user-data`. The
name of this file name has to match the name of the distro instance that will
be created in the next step.

Open that file with your text editor of choice (`notepad.exe` is just fine) and paste in the following contents:

```{code-block} yaml
#cloud-config
locale: pt_BR
users:
- name: jdoe
  gecos: John Doe
  groups: [adm,dialout,cdrom,floppy,sudo,audio,dip,video,plugdev,netdev]
  sudo: ALL=(ALL) NOPASSWD:ALL
  shell: /bin/bash

write_files:
- path: /etc/wsl.conf
  append: true
  content: |
    [user]
    default=jdoe

packages: [ginac-tools, octave]

runcmd:
   - sudo git clone https://github.com/Microsoft/vcpkg.git /opt/vcpkg
   - sudo apt-get install zip curl -y
   - /opt/vcpkg/bootstrap-vcpkg.sh
```

Save the file and close it.

```{note}
The cloud-config will create a user named `jdoe` and set it as default via
`/etc/wsl.conf`, install the packages `ginac-tools` and `octave`. It will also
install `vcpkg` from a git repository, since there is no deb or snap of that
application.
```

> See more: [WSL data source reference](https://cloudinit.readthedocs.io/en/latest/reference/datasources/wsl.html).

## Install and launch a new Ubuntu-24.04 instance

In PowerShell, run:

```{code-block} text
> wsl --install Ubuntu-24.04
```

This command installs and launches an Ubuntu-24.04 instance.
This instance will then be configured automatically by cloud-init.
The process can take several minutes, depending on your computer and network speeds.

## Verify automatic configuration by cloud-init

When the setup is complete, the WSL instance's shell will be logged in as the user `jdoe`.
You should see the standard welcome text:

```{code-block} text
:class: no-copy
Installing, this may take a few minutes...
Installation successful!
To run a command as administrator (user "root"), use "sudo <command>".
See "man sudo_root" for details.

Welcome to Ubuntu 24.04.1 LTS (GNU/Linux 6.6.36.3-microsoft-standard-WSL2 x86_64)

 * Documentation:  https://help.ubuntu.com
 * Management:     https://landscape.canonical.com
 * Support:        https://ubuntu.com/pro

 System information as of ter 01 out 2024 14:32:47 -03

  System load:  1.64                Processes:             63
  Usage of /:   0.2% of 1006.85GB   Users logged in:       0
  Memory usage: 4%                  IPv4 address for eth0: 172.22.8.90
  Swap usage:   0%


This message is shown once a day. To disable it please create the
/home/jdoe/.hushlogin file.
jdoe@mib01:~$
```

Once logged into the new distro instance's shell, verify that:

1. The default user matches what was configured:

```{code-block} text
jdoe@mib:~$ whoami
```

This should be verified with the output:

```{code-block} text
:class: no-copy
jdoe
```

2. The supplied cloud-config user data was approved by cloud-init validation.

```{code-block} text
jdoe@mib:~$ sudo cloud-init schema --system
```

Verified with the output:

```{code-block} text
:class: no-copy
Valid schema user-data
```

3. The locale is set

```{code-block} text
jdoe@mib:~$ locale
```

Verified with:

```{code-block} text
:class: no-copy
LANG=pt_BR
LANGUAGE=
LC_CTYPE="pt_BR"
LC_NUMERIC="pt_BR"
LC_TIME="pt_BR"
LC_COLLATE="pt_BR"
LC_MONETARY="pt_BR"
LC_MESSAGES="pt_BR"
LC_PAPER="pt_BR"
LC_NAME="pt_BR"
LC_ADDRESS="pt_BR"
LC_TELEPHONE="pt_BR"
LC_MEASUREMENT="pt_BR"
LC_IDENTIFICATION="pt_BR"
LC_ALL=

```

4. The packages were installed and the commands that they provide are available.

```{code-block} text
jdoe@mib:~$ apt list --installed | egrep 'ginac|octave'
```

Verified:

```{code-block} text
:class: no-copy

WARNING: apt does not have a stable CLI interface. Use with caution in scripts.

ginac-tools/noble,now 1.8.7-1 amd64 [installed]
libginac11/noble,now 1.8.7-1 amd64 [installed,automatic]
octave-common/noble,now 8.4.0-1 all [installed,automatic]
octave-doc/noble,now 8.4.0-1 all [installed,automatic]
octave/noble,now 8.4.0-1 amd64 [installed]
```

5. Lastly, verify that the commands requested were also run. In this case we set up `vcpkg` from git, as recommended by its
   documentation (there is no deb or snap available for that program):

```{code-block} text
jdoe@mib:~$ /opt/vcpkg/vcpkg version
```

This should be verified with:

```{code-block} text
:class: no-copy
vcpkg package management program version 2024-01-11-710a3116bbd615864eef5f9010af178034cb9b44

See LICENSE.txt for license information.
```

## Enjoy!

That’s all folks! In this guide, we’ve shown you how to use cloud-init to automatically set up Ubuntu on WSL2 with minimal touch.

This workflow will guarantee a solid foundation for your next Ubuntu WSL project.

We hope you enjoy using Ubuntu inside WSL!

