---
myst:
  html_meta:
    "description lang=en":
      "Ubuntu Pro for WSL automatically attaches your Pro subscription to instances of Ubuntu on WSL. It's easy to confirm that your subscription is active and that instances are Pro-attaching."
---

# Verify active Pro subscription and automatic Pro attachment with Ubuntu Pro for WSL

```{include} ../pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

If you have just installed and configured UP4W and a verification step is failing,
wait for a few seconds and try again. The process should not take longer than a minute.

(howto::verify-pro-sub)=
## Pro subscription

After installing UP4W on a Windows machine and entering your token you should see a confirmation that your Pro subscription is active:

![Confirmation in graphical interface that subscription is active.](../assets/status-complete.png)

Find and run _Ubuntu Pro for WSL_ from the Windows start menu at any time and the app will confirm whether you are subscribed.

(howto::verify-pro-attach)=
## Pro-attachment

```{note}
To verify Pro-attachment WSL should be installed on the Windows machine along
with an Ubuntu distro — Ubuntu 24.04 LTS will be used in this example.
```

To verify Pro-attachment a new Ubuntu instance needs to be created.
Running the following command in PowerShell will create a new Ubuntu-24.04 instance
and prompt you to create a username and password for the machine:

```{code-block} text
PS C:\Users\me\up4wInstall> wsl ~ -d Ubuntu-24.04
```

You will now be logged in to the new instance shell and can
check that UP4W has Pro-attached this instance with:

```{code-block} text
u@mib:~$ pro status
```

The output will confirm the following:

- Services like ESM are enabled
- Account and subscription information for Ubuntu Pro
- Verification of Pro-attachment


```{code-block} text
:class: no-copy
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

Each new Ubuntu WSL instance that is created should automatically now be Pro-attached.

To find other useful Ubuntu pro commands run:

```{code-block} text
u@mib:~$ pro --help
```

(howto::verify-landscape)=
## Landscape

For verification and troubleshooting of Landscape server and client configuration please refer to
[Landscape | View WSL host machines and child computers](https://ubuntu.com/landscape/docs/perform-common-tasks-with-wsl-in-landscape/#view-wsl-host-machines-and-child-computers).
