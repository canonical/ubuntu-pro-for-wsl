---
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

To install and configure Ubuntu Pro for WSL you will need:

* Windows 11 (recommended) or Windows 10 with minimum version 21H2 on a physical machine
* An Ubuntu Pro token

You should also verify that the [firewall rules are correctly set up](../reference/firewall_requirements.md).

(howto::install-up4w)=
## Install Ubuntu Pro for WSL

% :TODO: remove this warning once the app is made generally available (after the beta period).

```{warning}
The install link below will work only if you're logged in to the Microsoft Store with an account for which access to the app has been enabled.
```

You can install Pro for WSL [on this page of the Microsoft Store](https://apps.microsoft.com/detail/9PD1WZNBDXKZ):

![Install Ubuntu Pro for WSL from the Store](./assets/store.png)

(howto::config-up4w)=
## Choose a configuration method

After installation has finished you can start configuring Pro for WSL in two ways:

- Windows registry: easier to automate and deploy at scale 
- Graphical Windows application: convenient option for individual users

Click the appropriate tab to read more.

`````{tabs}

````{group-tab} Windows registry

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

After configuration using the Windows Registry the status in the Pro for WSL application will show that
the Pro subscription is active and managed by the user's organisation.
Unlike installation through the graphical Windows application, there is no option to detach the Pro
subscription in the application interface when the registry is used:

![alt text](./assets/status-complete-registry.png) 


````

````{group-tab} Graphical Windows application

## Enter your Pro token

Enter your Ubuntu Pro token in the space provided:

![Graphical interface of Ubuntu Pro for WSL with option to paste Pro token.](../assets/token-input-placeholder.png) 

Continue to the confirmation screen.

## Confirm subscription is active

You should now see that your Pro subscription is active.

Opening the application again at any point will show this screen, confirm the subscription is
active and enable detaching of the subscription.

````


`````

For additional verification steps refer to [our guide](./verify-subscribe-attach.md).
