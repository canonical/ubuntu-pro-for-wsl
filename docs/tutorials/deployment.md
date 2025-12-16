---
relatedlinks: "[Download&#32;Pro&#32;for&#32;WSL&#32;from&#32;ubuntu.com](https://www.ubuntu.com/desktop/wsl)"
myst:
  html_meta:
    "description lang=en":
      "Use the Ubuntu Pro for WSL application to deploy Ubuntu on WSL to remote Windows machines from a Landscape server."
---

(tut::deploy)=
# Deploy WSL instances remotely with Ubuntu Pro for WSL and Landscape

```{include} ../includes/pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

In this tutorial you will learn how Ubuntu Pro for WSL can help you
deploy Ubuntu to remote Windows machines using Landscape.

## What you will do

- Register a Windows host instance with Landscape
- Create a custom WSL profile on Landscape
- Deploy Ubuntu with the custom profile to a remote Windows machine

## What you need

* Windows 11 (recommended) or Windows 10 with minimum version 21H2 on a physical machine
* A minimum of 16GB RAM and 8-core processor
* The latest version of Landscape Server set up and configured on a physical or virtual machine
* WSL installed and configured on Windows
* The Pro for WSL app installed and configured with a Pro token

```{tip}
It is recommended that you complete the [getting
started](./getting-started-with-up4w.md) tutorial to familiarise yourself with
installation and configuration of the Pro for WSL app.
```

### WSL must be installed and configured

You need WSL installed and configured to follow this tutorial.

Installation instructions are provided in the [official Microsoft
documentation](https://learn.microsoft.com/en-us/windows/wsl/install).

### Any existing instance of Ubuntu-24.04 should be removed

You will be remotely deploying Ubuntu-24.04 on a Windows machine using Landscape.

Uninstall any pre-existing instance of Ubuntu-24.04 on the machine before you
start the tutorial.

To check if an Ubuntu-24.04 instance exists, run the following in PowerShell:

```{code-block} text
> wsl -l -v
```

Confirm that there is no Ubuntu-24.04 instance before continuing.
If one does exist, back it up and uninstall it.

````{dropdown} Backing up and uninstalling an existing Ubuntu-24.04 instance
If you have an existing Ubuntu-24.04 instance, run the following commands:

```{code-block} text
> wsl --terminate Ubuntu-24.04
> mkdir backup
> wsl --export Ubuntu-24.04 .\backup\Ubuntu-24.04.tar.gz
> wsl --unregister Ubuntu-24.04

```

This stops any running instance of Ubuntu-24.04, creates a backup folder,
generates a compressed backup of the distro, and uninstalls the instance.

Instructions for restoring the backup can be found at the end of the tutorial.

````

### A Landscape server must be set up and available

You need a Landscape server set up and access to the Landscape dashboard in a browser.

[Landscape SaaS edition](https://documentation.ubuntu.com/landscape/what-is-landscape/#editions-of-landscape)
is bundled with your Pro subscription and can be set up as follows:

1. Use your Ubuntu One SSO credentials to sign in to [landscape.canonical.com](https://landscape.canonical.com)
2. Create a Landscape SaaS account
3. Note the account name associated with the Landscape server

Please refer to the [Landscape
documentation](https://documentation.ubuntu.com/landscape/how-to-guides/landscape-installation-and-set-up/) for detailed setup and
additional installation options.

(tut::config-landscape-up4w)=
## Configure Landscape in the Ubuntu Pro for WSL app

Open the Pro for WSL app, enter your Pro token and continue to the Landscape configuration screen.

Choose your preferred configuration option and enter the required details.
If you choose Manual configuration, you only require the FQDN of your Landscape server.

```{note}
If you are using Landscape SaaS, enter `landscape.canonical.com` for the FQDN
and the account name from your Landscape dashboard.
```

When you continue, a status screen will confirm that your configuration is complete.

> A dedicated how-to guide on configuring Landscape with Pro for WSL can be found [here](../howto/set-up-landscape-client).

## Register the Windows host instance with Landscape

```{admonition} Usage of the term "instance"
:class: warning
In the Landscape dashboard, an "instance" refers to the Windows host running WSL.

In this documentation, we often use "instance" to refer to instances of WSL running on the Windows host.
```

Refresh the Landscape dashboard.

Go to {guilabel}`Instances`, and review the pending instances.

Check the box for your Windows host instance and approve it, leaving the access
group as "Global access".

Refresh the page and the Windows host will be listed under {guilabel}`Instances`.

Select the Windows host and assign it the tag "wsl-target".

## Create a WSL profile and deploy an Ubuntu instance

WSL profiles on Landscape enable the deployment of custom Ubuntu instances to
your Windows machine.

Go to **Profiles > WSL profiles** in the dashboard and add a WSL profile.

Complete the fields as follows:

| Field               | Value                                  |
| ------------------- | -------------------------------------- |
| Name                | WSL-CUDA                               |
| Description         | CUDA-enabled WSL instances             |
| Access group        | Global access                          |
| RootFS image        | Ubuntu 24.04 LTS                       |
| Cloud-init          | Plain text                             |

Copy and paste this cloud-init configuration:

```yaml
#cloud-config
locale: en_GB.UTF-8
users:
- name: u
  gecos: Ubuntu
  groups: [adm,dialout,cdrom,floppy,sudo,audio,dip,video,plugdev,netdev]
  sudo: ALL=(ALL) NOPASSWD:ALL
  shell: /bin/bash

write_files:
- path: /etc/wsl.conf
  append: true
  content: |
    [user]
    default=u

runcmd:
  - cd /tmp
  - wget https://developer.download.nvidia.com/compute/cuda/repos/wsl-ubuntu/x86_64/cuda-keyring_1.1-1_all.deb
  - dpkg -i cuda-keyring_1.1-1_all.deb
  - apt-get update
  - apt-get -y install cuda-toolkit-12-6
```

```{admonition} What this config file does
:class: tip
* Sets a default user "u"
* Assigns the user to groups and grants permissions
* Gives the user passwordless `sudo` rights
* Specifies the login shell for the user
* Downloads and installs the CUDA toolkit
```

Search for the "wsl-target" tag and select it, then confirm that you want to
add the WSL profile.

Go to {guilabel}`Activities` and confirm that the "Create instance Ubuntu-24.04" activity is queued.

This means that an instance of Ubuntu is in the process of being deployed to
the Windows host.

<!-- (tut::create-instance-remote)= -->
## Test the deployed instance

On the Windows host machine, list the installed WSL distros:

```{code-block} text
> wsl -l -v
```

The output should now confirm that Ubuntu-24.04 is "installing" or "running".

Installing the CUDA toolkit can take some time. After a few minutes you should
be able to list the distros again and confirm that Ubuntu-24.04 is "stopped".

When the Ubuntu-24.04 instance has launched, confirm that the correct default user "u" has been set from the prompt:

```{code-block} text
:class: no-copy
u@<hostname>:~$
```

Next, confirm that CUDA has been installed successfully:

```{code-block} text
$ apt policy cuda-toolkit-12-6
```


```{code-block} text
:class: no-copy
cuda-toolkit-12-6
  Installed: 12.6.3-1
  Candidate: 12.6.3-1
...
...
```

Then confirm that your GPU is being detected correctly:

```{code-block} text
$ nvidia-smi
```

```{code-block} text
:class: no-copy
+---------------------------------------------------------------------------+
| NVIDIA-SMI 535.157        Driver Version 538.18       CUDA Version: 12.2  |
|...                                                                        |
|...                                                                        |
+---------------------------------------------------------------------------+
```

Finally, run `pro status`, to confirm that Pro for WSL has automatically Pro-attached the Ubuntu instance.

````{dropdown} Deleting the deployed instance and restoring any backups
:icon: undo

Terminate the new instance and uninstall it from PowerShell:

```{code-block} text
> wsl --terminate Ubuntu-24.04
> wsl --unregister Ubuntu-24.04
```

Restore the backup:

```{code-block} text
> wsl --import Ubuntu-24.04 <directory-to-install-filesystem> .\backup\Ubuntu-24.04.tar.gz
```

This will restore your data and install the filesystem to the path you specify.

You can then launch the distro as before.

````

## Next steps

Our documentation includes several [how-to guides](../howto/index)
for completing specific tasks and [reference](../reference/index) material
describing key information relating to Pro for WSL.
