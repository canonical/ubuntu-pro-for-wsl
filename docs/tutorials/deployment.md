---
myst:
  html_meta:
    "description lang=en":
      "Use the Ubuntu Pro for WSL application to deploy Ubuntu on WSL to remote Windows machines from a Landscape server."
---

(tut::deploy)=
# Deploy WSL instances remotely with Ubuntu Pro for WSL and Landscape

```{include} ../pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

In this tutorial you will learn how Ubuntu Pro for WSL (UP4W) can help you
deploy Ubuntu to remote Windows machines using Landscape.

## What you will do

- Register a Windows host instance with Landscape
- Create a WSL profile on Landscape
- Deploy Ubuntu to a remote Windows machine

## What you need

* Windows 11 (recommended) or Windows 10 with minimum version 21H2 on a physical machine
* A minimum of 16GB RAM and 8-core processor
* The latest version of Landscape Server set up and configured on a physical or virtual machine
* WSL installed and configured on Windows
* The UP4W app installed and configured with a Pro token

```{tip}
It is recommended that you complete the [getting
started](./getting-started-with-up4w.md) tutorial to familiarise yourself with
installation and configuration of the UP4W app.
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

```text
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

### Landscape server must be installed and accessible

You need a Landscape server set up and you should be
able access the Landscape dashboard in a browser.

Please refer to the [Landscape
documentation](https://ubuntu.com/landscape/install) for detailed setup and
configuration instructions.

(tut::config-landscape-up4w)=
## Configure Landscape in the UP4W app

Open the UP4W app, enter your Pro token and continue to the Landscape configuration screen.

Choose your preferred configuration option and enter the required details.
If you choose Manual configuration, you only require the FQDN of your Landscape server.

When you continue, a status screen will confirm that your configuration is complete.

> A dedicated how-to guide on configuring Landscape with UP4W can be found [here](../howto/set-up-landscape-client).

## Register the Windows host instance with Landscape

```{admonition} Usage of the term "instance"
:class: note
In the Landscape dashboard, an "instance" refers to the Windows host running WSL.

On the Windows machine itself, an "instance" refers to an installed WSL distro.
```

Refresh the Landscape dashboard.

Go to **Instances**, and review the pending instances.

Check the box for your Windows host instance and approve it, leaving the access
group as "Global access".

Refresh the page and the Windows host will be listed under **Instances**.

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

Go to **Activities** and confirm that the "Create instance Ubuntu-24.04" activity is queued.

This means that an instance of Ubuntu is in the process of being deployed to
the Windows host.

<!-- (tut::create-instance-remote)= -->
## Test the deployed instance

On the Windows host machine, list the installed WSL distros:

```text
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

```text
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

```text
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

Finally, run `pro status`, to confirm that UP4W has automatically Pro-attached the Ubuntu instance.

````{dropdown} Deleting the deployed instance and restoring any backups
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
describing key information relating to UP4W.
