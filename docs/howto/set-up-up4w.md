---
relatedlinks: "[Download&#32;Pro&#32;for&#32;WSL&#32;from&#32;ubuntu.com](https://www.ubuntu.com/desktop/wsl)"
myst:
  html_meta:
    "description lang=en":
      "Ubuntu Pro for WSL is a Windows application that automatically attaches your Ubuntu Pro subscription to instances of Ubuntu on WSL."
---

# Install Ubuntu Pro for WSL and add a Pro token

```{include} ../includes/pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

To install Ubuntu Pro for WSL, you need Windows 11 (recommended) or Windows 10 with minimum version 21H2 on a physical machine

To configure your Ubuntu instances to Pro-attach, you must have an Ubuntu Pro token.

If necessary, you can verify that your [firewall rules are correctly set up](../reference/firewall_requirements.md).

(howto::install-up4w)=
## Install Ubuntu Pro for WSL

Pro for WSL can be installed from the Ubuntu website, the Microsoft Store, or using WinGet.

`````{tab-set}
````{tab-item} Ubuntu website
Go to [ubuntu.com/desktop/wsl](https://ubuntu.com/desktop/wsl).

Click {guilabel}`Download the Ubuntu Pro for WSL app`.

Run the downloaded installer.
````

````{tab-item} Microsoft Store
Search for "Ubuntu Pro" in the Microsoft Store.

Open the store page and install the app.
````

````{tab-item} WinGet
Run the following command in PowerShell to install the app:

```{code-block} text
> winget install Canonical.UbuntuProforWSL
```
````
``````

(howto::config-up4w)=
## Choose a configuration method

After installing Pro for WSL, the application can be configured in two ways:

- The **Windows registry**: useful for automating deployment at scale 
- The application's **graphical interface**: convenient option for individual users

`````{tab-set}

````{tab-item} Configure with the Windows registry

## Set up the key and values

First, ensure that Ubuntu Pro for WSL has run at least once after installation.
This guarantees that the key and values necessary for configuration are set up
in the registry.

```{admonition} Methods of modifying registry data
:class: tip
This guide uses the registry editor for setting the Pro token.

Advanced users of the registry can find relevant information in the
[Microsoft documentation](https://learn.microsoft.com/en-us/troubleshoot/windows-server/performance/windows-registry-advanced-users)
for alternative methods to modify the registry data.
```

## Add Pro token using the registry editor

To open the registry editor on a Windows machine, type
<kbd>Win</kbd>+<kbd>R</kbd> then enter `regedit`:

Navigate to the `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro` key.

Locate the {guilabel}`UbuntuProToken` value and enter your Pro token in the
{guilabel}`Value data` field.

After configuration using the Windows Registry, the status in the Pro for WSL application will show that
the Pro subscription is active and managed by the user's organisation.

Unlike configuration through the graphical interface, when the registry is used
to configure Pro for WSL there is no option for the user to detach the Pro
subscription in the application:

![alt text](./assets/status-complete-registry.png) 
````

````{tab-item} Configure with the graphical interface

## Enter your Pro token

Enter your Ubuntu Pro token in the space provided:

![Graphical interface of Ubuntu Pro for WSL with option to paste Pro token.](../assets/token-input-placeholder.png) 

Continue to the confirmation screen.

## Confirm subscription is active

You should now see that your Pro subscription is active:

![Confirmation in graphical interface that subscription is active.](../assets/status-complete.png)

Opening the application again at any point will show this screen, confirm the subscription is
active and enable detaching of the subscription.
````
`````

For additional verification steps refer to [our guide](./verify-subscribe-attach.md).
