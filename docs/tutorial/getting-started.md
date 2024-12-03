# Get started with UP4W

Windows Subsystem for Linux ([WSL](https://ubuntu.com/desktop/wsl)) makes it possible to run Ubuntu on a Windows machine.
Ubuntu Pro for WSL (UP4W) ensures that each new Ubuntu WSL instance that you create will automatically attach to your [Ubuntu Pro](https://ubuntu.com/pro) subscription.

In this tutorial you will learn how to install UP4W on Windows and verify that Ubuntu WSL instances are Pro-attaching.
You should then be ready for more advanced usage scenarios.

## What you will do

- Install UP4W from the Microsoft Store
- Configure UP4W with a Pro token
- Test automatic Pro-attachment of WSL instances

(ref::backup-warning)=
```{warning}
**If you already have Ubuntu WSL pre-installed:** 

We recommend that any Ubuntu WSL installed is exported then deleted.
You can then install it as described in this tutorial.
At the end of the tutorial you can import and restore your data.

Read our [how-to guide on backup and restore](../howto/backup-and-restore.md).
```

## What you will need

- A Windows 10 or 11 machine with a minimum of 16GB RAM and 8-core processor
- Some familiarity with commands for the Linux shell and PowerShell

```{note}
WSL enables using a Linux shell and Windows PowerShell side-by-side on the same machine.
In this tutorial, commands will be prefixed by a prompt that indicates the shell being used, for example:

- `PS C:\Users\me\tutorial>` is a PowerShell prompt where the current working directory is `C:\Users\me\tutorial`.

- `u@mib:~/tutorial$` indicates a Linux shell prompt login as user "u" where the current working directory is `/home/ubuntu/tutorial/`

Output logs are included in this tutorial when instructive but are sometimes omitted to save space.
```

## Set up Ubuntu WSL

(tut::get-wsl)=
### Install WSL

WSL can be installed directly from the [Microsoft Store](https://apps.microsoft.com/detail/9P9TQF7MRM4R).

If you already have WSL installed, with `~\.wslconfig` on your system, you
are advised to backup the file then remove it before continuing the tutorial.

To check if the file exists run:

```text
PS C:\Users\me\tutorial> Test-Path -Path "~\.wslconfig"
```

If this returns `True` then the file exists and can be removed with:

```text
PS C:\Users\me\tutorial> Remove-Item ~\.wslconfig
```

(tut::get-ubuntu)=
### Install Ubuntu

Ubuntu 24.04 LTS is recommended for this tutorial and can be installed from the
Microsoft Store:

> Install [Ubuntu 24.04 LTS](https://apps.microsoft.com/detail/9nz3klhxdjp5) from the Microsoft Store

For other installation options refer to our [install Ubuntu on WSL2 guide](https://canonical-ubuntu-wsl.readthedocs-hosted.com/en/latest/guides/install-ubuntu-wsl2/).

At this point, running `ubuntu2404.exe` in PowerShell will launch an Ubuntu WSL instance
and log in to its shell.

To manually associate that Ubuntu instance with a Pro subscription you could
run the `sudo pro attach` command from within the Ubuntu instance.

This, however, would need to be repeated manually for each new instance.
UP4W solves this scalability problem by automating Pro-attachment.
Next, let's take a look at how that works in practice.

## Set up Ubuntu Pro for WSL

(tut::get-ubuntu-pro)=
### Get an Ubuntu Pro token

An active Ubuntu Pro subscription provides you with a token that can be added to the Ubuntu Pro client on WSL instances.

Your subscription token can be retrieved from the [Ubuntu Pro Dashboard](https://ubuntu.com/pro/dashboard).

Visit the [Ubuntu Pro](https://ubuntu.com/pro/subscribe) page if you need a new subscription.
The `Myself` option for a personal subscription is free for up to 5 machines. 

Once you have a token you are ready to install UP4W.

(tut::get-up4w)=
### Install and configure UP4W

% :TODO: remove this warning once the app is made generally available (after the beta period).

```{warning}
The install link below will work only if you're logged in to the Microsoft Store with an account for which access to the app has been enabled.
```

UP4W can be installed from [this link to the Microsoft Store](https://apps.microsoft.com/detail/9PD1WZNBDXKZ).

Open the application and paste the token you copied from the Ubuntu Pro dashboard:

![UP4W GUI main screen](../assets/token-input-placeholder.png)

After you confirm, a status screen will appear showing that configuration is complete:

![Configuration is complete](../assets/status-complete.png)

Done! You can close the UP4W window before continuing.
If at any time you want to detach your Pro subscription just open the UP4W application
and select **Detach Ubuntu Pro**.

Your Ubuntu Pro subscription is now attached to UP4W on the Windows host.
UP4W will automatically forward the subscription to the Ubuntu Pro client on your Ubuntu WSL instances.

(tut::verify-pro-attach)=
## Verify Pro-attachment

All Ubuntu WSL instances will now be automatically added to your Ubuntu Pro subscription.

Open Windows PowerShell and run the following command to create a new Ubuntu 24.04 instance,
entering a user and password when prompted. For quick testing, set both to `u`:

```text
PS C:\Users\me\tutorial> ubuntu2404.exe
```

You will now be logged in to the new instance shell and can check that UP4W has Pro-attached this instance with:

```text
u@mib:~$ pro status
```

The output should indicate that services like ESM are enabled, with account and subscription information also shown:

```text
SERVICE          ENTITLED  STATUS       DESCRIPTION
esm-apps         yes       enabled      Expanded Security Maintenance for Applications
esm-infra        yes       enabled      Expanded Security Maintenance for Infrastructure

NOTICES
Operation in progress: pro attach

For a list of all Ubuntu Pro services, run 'pro status --all'
Enable services with: pro enable <service>

     Account: me@ubuntu.com
Subscription: Ubuntu Pro - free personal subscription
```

Packages can also be accessed from all the enabled services.
Running `sudo apt update` will produce output like the following:

```text
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

Now let's check that another Ubuntu instance will also Pro-attach.

Install Ubuntu 22.04 LTS directly from PowerShell:

```text
PS C:\Users\me\tutorial> wsl --install Ubuntu-22.04
```

Once you are in the instance shell, enter a username and password then run `pro status`.
You should again get confirmation of successful Pro-attachment for the new instance.

> If you want to uninstall UP4W after this tutorial refer to [our how-to guide](../howto/uninstalling.md).

## Next steps

This is only the start of what you can do with UP4W.

If you need to create and manage large numbers of Ubuntu WSL instances
you will probably want to use the Windows registry.
By using the Windows registry you can associate a Pro token with
each new WSL instance using your organisation's own deployment solution.

> For detailed step-by-step instructions on using the Windows registry read our short guide on how to [install and configure UP4W](../howto/set-up-up4w.md).

Landscape support is also built-in to UP4W.
With a single configuration file, you can create and manage
multiple WSL instances that will automatically be registered
with your Landscape server:

> For more information, please refer to our tutorial on how to [deploy WSL instances with UP4W and Landscape](./deployment.md).

Our documentation includes several other [how-to guides](../howto/index)
for completing specific tasks, [reference](../reference/index) material
describing key information relating to UP4W.
