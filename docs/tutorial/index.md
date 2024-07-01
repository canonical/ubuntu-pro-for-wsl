# Get started with UP4W

Windows Subsystem for Linux ([WSL](https://ubuntu.com/desktop/wsl)) makes it possible to run Ubuntu — the number 1 open-source operating system — on a Windows machine.
With Ubuntu Pro for WSL (UP4W) an [Ubuntu Pro](https://ubuntu.com/pro) subscription empowers you to manage Ubuntu WSL instances at scale.

In this tutorial you will develop an understanding of how UP4W can be installed and deployed for managing multiple WSL instances.

## What you will do

- Install UP4W from the Microsoft Store
- Test automatic configuration of WSL instances by UP4W
- Manage WSL instances from Landscape

## What you need

- A Windows 10 or 11 machine with a minimum of 16GB RAM and 8-core processor to host WSL instances
- The latest version of Landscape Server set up and configured on a physical or virtual machine

```{note}
WSL enables using a Linux shell and Windows PowerShell side-by-side on the same machine.
Throughout this tutorial, commands will be prefixed by a prompt that indicates the shell being used, for example:

- `PS C:\Users\me\tutorial>` is a PowerShell prompt where the current working directory is `C:\Users\me\tutorial`.

- `ubuntu@wsl:~/tutorial$` indicates a Linux shell prompt login as ubuntu where the current working directory is `/home/ubuntu/tutorial/`

Output logs are included in this tutorial when instructive but are typically omitted to save space.
```

## Set things up

### Install WSL and Ubuntu

WSL can be installed directly from the [Microsoft Store](https://www.microsoft.com/store/productId/9P9TQF7MRM4R).

If you already have WSL installed, with `~\.wslconfig` on your system, you
are advised to backup the file and remove it before continuing the tutorial.

To check if the file exists run:

```text
PS C:\Users\me\tutorial> Test-Path -Path "~\.wslconfig"
```

If this returns `True` then the file exists and can be removed with:

```text
PS C:\Users\me\tutorial> Remove-Item ~\.wslconfig
```

Ubuntu 24.04 LTS is recommended for this tutorial and can be installed from the
Microsoft Store:

- Install [Ubuntu 24.04 LTS](https://www.microsoft.com/store/productId/9nz3klhxdjp5) from the Microsoft Store

(ref::backup-warning)=
```{warning}
**If you already have Ubuntu WSL pre-installed:** 

We recommend that any Ubuntu WSL installed is exported then deleted.
You can then install them as described in this tutorial.
At the end of the tutorial you can re-import and restore your data.

Exporting, deleting and re-importing Ubuntu is achieved with the following commands:

    PS C:\Users\me\tutorial> wsl --export Ubuntu-24.04 .\backup\Ubuntu-{version}.tar.gz

    PS C:\Users\me\tutorial> wsl --unregister Ubuntu-24.04

    PS C:\Users\me\tutorial> wsl --import Ubuntu-24.04 .\backup\Ubuntu-24.04 .\backup\Ubuntu-24.04.tar.gz


```

You can now launch WSL instances of Ubuntu on your Windows machine.

For the remainder of the tutorial you will need to have a Landscape server
set up and you should be able access your Landscape dashboard in a browser.
Please refer to the [Landscape documentation](https://ubuntu.com/landscape/docs/install-and-configure-landscape)
for setup and configuration instructions.

(tut::ensure-ubuntu-pro)=
### Get an Ubuntu Pro token

An active Ubuntu Pro subscription provides you with a token that can be added to the Ubuntu Pro client on WSL instances.

Your subscription token can be retrieved from the [Ubuntu Pro Dashboard](https://ubuntu.com/pro/dashboard).

Visit the [Ubuntu Pro](https://ubuntu.com/pro/subscribe) page if you need a new subscription.
The `Myself` option for a personal subscription is free for up to 5 machines. 

Once you have a token you are ready to install UP4W.

### Install UP4W

% :TODO: remove this warning once the app is made generally available (after the beta period).

```{warning}
The install link below will work only if you're logged in to the Microsoft Store with an account for which access to the app has been enabled.
```

To install UP4W go to [this link to the Microsoft Store](https://www.microsoft.com/store/productId/9PD1WZNBDXKZ) and click **Install**.

![Install Ubuntu Pro for WSL from the Store](./assets/store.png)

After installation has finished click **Start** to begin configuring UP4W.

```{note}
UP4W can be configured through the [Windows registry](windows-registry) instead of the GUI.
The steps are outlined briefly in [Install and configure UP4W > Using the registry](howto::configure::registry).
This approach may be more suitable when operating at scale.
```

In the UP4W Windows application click the arrow beside "Already have a token?".

Paste your token from the Ubuntu Pro dashboard during [Setup](tut::ensure-ubuntu-pro) and click **Confirm**.
You will then be shown the Landscape configuration screen.

![UP4W GUI main screen](./assets/up4w_gui.png)

Create a new file in your home directory named `landscape.txt` and enter following contents.
For the purpose of this tutorial, replace:

<!-- QUERY: double check below in relation to landscape docs -->

- `<SERVER_URL>` by the host url (or FQDN) of the machine that runs the Landscape server
- `<YOUR_WINDOWS_USER_NAME>` by the user name on the same Windows machine

```text
[host]
url = <SERVER_URL>:6554
[client]
account_name = standalone
registration_key =
url = https://<SERVER_URL>/message-system
log_level = debug
ping_url = https://<SERVER_URL>/ping
```

```{note}
If the machine running the server is not trusted on your network you may need to
explicitly reference a path to the SSL public key on a Windows host machine.

For example, if you followed the [Landscape Quickstart](https://ubuntu.com/landscape/docs/quickstart-deployment)
installation, the auto-generated self-signed certificate can be found at `/etc/ssl/certs/landscape_server.pem`.

This can be copied to a Windows machine and referenced in `landscape.txt`:

  ssl_public_key = C:\Users\<YOUR_WINDOWS_USER_NAME>\landscape_server.pem

If necessary, the SSL public key can be added after the Windows host has first
been registered in Landscape.
```

Then input `<SERVER_URL>` in "Quick Setup" in the field labelled "Landscape FQDN".
Alternatively, provide a path to `landscape.txt` in "Custom Configuration".

![Loading Landscape custom config](./assets/loading-custom-landscape-config.png)

Click on the **Continue** button and you will see a status screen confirming the configuration is complete.

![Configuration is complete](./assets/status-complete.png)

Done! You can now close the UP4W window.

Your Ubuntu Pro subscription is now attached to UP4W on the Windows host.
UP4W will automatically forward the subscription to the Ubuntu Pro client on your Ubuntu WSL instances.
This means that all Ubuntu WSL instances will be automatically added to your Ubuntu Pro subscription.

This has also configured the Landscape client built into your UP4W Windows agent to know about your Landscape server; UP4W will forward this configuration to the Landscape client on your Ubuntu WSL instances as well; and all systems where the Landscape client has been configured this way are automatically registered with Landscape.

<!-- TODO: the section below needs to work without the detail we are removing -->
### UP4W host registration with Landscape

Go back to your web browser and refresh the Landscape dashboard. On the right-hand side of the
page you should see a request to approve your Windows host registration ("Computers needing authorisation").
Click on the computer name (in this case: `mib`) and when the new page loads click **Accept**.

![Approve the Windows host registration](./assets/host-pending-approval.png)

![Accept host](./assets/accept-host.png)

At the top of the page, on the right-hand side of the Landscape logo, click on "Computers". You should see your host machine listed. Details such as the operating system may take some time to appear.

![Host and WSL instances in Landscape](./assets/host.png)

Now you can leverage UP4W from your Landscape server to create and provision Ubuntu WSL instances on the host.

## Deploy WSL instances

### Create an Ubuntu WSL instance locally

Open Windows PowerShell and run the following command to create a new Ubuntu 24.04 instance.
When prompted create the default user and password. For convenience, we'll set both to `u`.

```text
PS C:\Users\me\tutorial> ubuntu2404.exe

Installing, this may take a few minutes...
Please create a default UNIX user account. The username does not need to match your Windows username.
For more information visit: https://aka.ms/wslusers
Enter new UNIX username: u
New password:
Retype new password:
passwd: password updated successfully
Installation successful!
To run a command as administrator (user "root"), use "sudo <command>".
See "man sudo_root" for details.

u@mib:~$

```

You will now be logged in to the new instance shell and can
check that UP4W has Pro-attached this instance:

The output indicates that services like ESM are enabled, with account and subscription information also shown:

```text
u@mib:~$ pro status
SERVICE          ENTITLED  STATUS       DESCRIPTION
esm-apps         yes       enabled      Expanded Security Maintenance for Applications
esm-infra        yes       enabled      Expanded Security Maintenance for Infrastructure

NOTICES
Operation in progress: pro attach

For a list of all Ubuntu Pro services, run 'pro status --all'
Enable services with: pro enable <service>

     Account: me@ubuntu.com
Subscription: Ubuntu Pro - free personal subscription
u@mib:~$
```

Packages can be accessed from all the enabled services:

```text
u@mib:~$ sudo apt update
Hit:1 http://archive.ubuntu.com/ubuntu noble InRelease
Hit:2 http://ppa.launchpad.net/ubuntu-wsl-dev/ppa/ubuntu noble InRelease
Hit:3 http://security.ubuntu.com/ubuntu noble-security InRelease
Hit:4 http://archive.ubuntu.com/ubuntu noble-updates InRelease
Hit:5 http://ppa.launchpad.net/landscape/self-hosted-beta/ubuntu noble InRelease
Hit:6 https://esm.ubuntu.com/apps/ubuntu noble-apps-security InRelease
Hit:7 http://archive.ubuntu.com/ubuntu noble-backports InRelease
Hit:8 http://ppa.launchpad.net/cloud-init-dev/proposed/ubuntu noble InRelease
Hit:9 https://esm.ubuntu.com/infra/ubuntu noble-infra-security InRelease
Reading package lists... Done
Building dependency tree... Done
Reading state information... Done
All packages are up to date.
```

UP4W should have also Landscape-registered this instance.
To verify, refresh the Landscape server web page and the instance should be listed under "Computers needing authorisation".

<!-- ![New WSL instance pending approval](./assets/wsl-pending-approval.png) -->

To accept the registration click on the instance name, set "Tags" to `wsl-vision` in the pop-up then click **Accept**.
The `wsl-vision` tag will be used for all the instances accepted into Landscape.

<!-- ![Accept and tag Ubuntu Preview](./assets/accept-ubuntu-preview-tag.png) -->

### Create an Ubuntu WSL instance remotely

Back on the Landscape page in your web browser, navigate to "Computers" and click on the Windows machine (below: `mib`). You will find "WSL Instances" on the right side of the page.
Click on the "Install new" link then set "Instance Type" to "Ubuntu" and click **Submit**. A status page will
appear showing the progress of the new instance creation.

![Create instance via Landscape](./assets/create-instance-via-landscape.png)

![Creation progress](./assets/creation-progress.png)

The Landscape server will talk to the Landscape client built into your UP4W.
UP4W will then install the `Ubuntu` application and create an Ubuntu WSL instance automatically.
In PowerShell, run `ubuntu.exe` to log in to the new instance.

```text
PS C:\Users\me\tutorial> ubuntu.exe

me@mib:~$
```

You can run `pro status` to verify pro-attachment and refresh your Landscape server page to verify and accept the registration.
As before, apply the `wsl-vision` tag and click `Accept`.

### Deploy packages to all Ubuntu WSL instances

On your Landscape server page, navigate to `Organization` > `Profiles`, click on
`Package Profiles` then `Add package profile`. Fill in the form with the following values and click "Save".

| Field               | Value                                  |
| ------------------- | -------------------------------------- |
| Title               | Vision                                 |
| Description         | Computer Vision work                   |
| Access group        | Global                                 |
| Package constraints | Manually add constraints               |
|                     | Depends on `python3-opencv` `>=` `4.0` |

![Create package profile](./assets/create-package-profile.png)

On the bottom of the "Vision" profile page, in the "Association" section, set the "New tags" field to `wsl-vision` and click **Change**.

![Applying the profile to the WSL instances](./assets/applying-profile.png)

In the "Summary" section in the middle of the page you will see a status message showing that two computers are `applying the profile`. Click on the `applying the profile` link and then, in the "Activities" list, click on **Apply package profile** to see the progress of the package deployment.

<!-- ![Progress of the package deployment](./assets/package-deployment-progress.png) -->

When this process has completed, use one of your instance shells to verify that the `python3-opencv` package has been installed.
For example, in the `Ubuntu` instance the first three packages returned are:

```text
me@mib:~$ apt list --installed | grep -m 3 opencv

libopencv-calib3d4.5d/jammy,now 4.5.4+dfsg-9ubuntu4 amd64 [installed,automatic]
libopencv-contrib4.5d/jammy,now 4.5.4+dfsg-9ubuntu4 amd64 [installed,automatic]
libopencv-core4.5d/jammy,now 4.5.4+dfsg-9ubuntu4 amd64 [installed,automatic]
```

You know how to leverage UP4W and Landscape to efficiently manage your Ubuntu WSL instances at scale.

## Tear things down

### Uninstall UP4W

In the Windows Start Menu, locate the "Ubuntu Pro for WSL" application and right-click on it, then click **Uninstall**.

![Uninstall Ubuntu Pro for WSL](./assets/start-menu-uninstall.png)

Additionally remove the `.ubuntupro` directory from your Windows user profile directory.

```text
PS C:\Users\me\tutorial> Remove-Item -Recurse -Force C:\Users\me\.ubuntupro
```

### Remove Ubuntu WSL apps

```{warning}
**If you already have them pre-installed:**

Refer to the [backup instructions](ref::backup-warning) to restore your pre-existing instances.

Otherwise, proceed with the commands below.
```

In PowerShell run the following command to stop WSL:

```text
PS C:\Users\me\tutorial> wsl --shutdown
```

Then, in the Windows Start Menu, locate the "Ubuntu 24.04 LTS" application, right-click on it, and select "Uninstall".

The instances will be removed automatically.

### Remove WSL app

Only do this if you don't need WSL on this Windows machine following the tutorial.

In the Windows Start Menu locate the "WSL" application, right-click on it then select "Uninstall".

## Next steps

This tutorial has introduced you to the amazing things that can be achieved with UP4W.

In the rest of the documentation you can find [how-to guides](../howto/index)
for completing specific tasks, [reference](../reference/index) material
describing key information relating to UP4W and dedicated [documentation for
developers](../dev/index).
