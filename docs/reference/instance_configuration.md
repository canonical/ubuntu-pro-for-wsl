---
myst:
  html_meta:
    "description lang=en": "Reference information on configuring WSL instances."
---

# WSL instance configuration

You can configure instances of Ubuntu on WSL using different methods.

Each configuration method has a different scope and use-case. Depending on the method, you may affect all Ubuntu instances or only one Ubuntu instance.

| Method                            | Scope                                                              | Location                                                                                   |
| --------------------------------- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------ |
| [WSL Settings](ref::wsl-settings) | General settings that apply to all WSL instances                   | Graphical application named `WSL Settings` pre-installed with WSL                          |
| [`.wslconfig`](ref::.wslconfig)   | General settings that apply to all WSL instances                   | `%UserProfile%\.wslconfig`, in the Windows file system                                     |
| [`wsl.config`](ref::wsl.config)   | Settings for a specific WSL instance only                          | `/etc/wsl.conf`, while inside a WSL instance                                               |
| [Cloud-init](ref::cloud-init)     | Ubuntu provisioning settings for instances of a named distribution | `<Distro Name>.userdata` files in `%UserProfile%\.cloud-init\`, in the Windows file system |
| [Ubuntu Pro for WSL](ref::up4w)   | Pro settings that apply to all compatible Ubuntu instances         | Installable [graphical application ](howto::up4w)                                          |

(ref::wsl-settings)=

## WSL Settings

WSL Settings is a graphical application that comes with WSL, allowing you to manage general settings that apply to all WSL 2 instances. It is analogous to the [`.wslconfig` file](ref::.wslconfig).

The WSL Settings application can be used for configurations including hardware resource limits, networking, and custom kernels.

For changes to apply, you may need to run `wsl --shutdown` from PowerShell to shut down the WSL 2 VM and then restart your WSL instance.

(ref::.wslconfig)=

## .wslconfig

Global configuration settings that apply to all WSL 2 instances can be managed using the `.wslconfig` file located at `%UserProfile%\.wslconfig`. By default, `.wslconfig` does not exist, and it may need to be created manually.

The `.wslconfig` file is analogous to the graphical application [WSL Settings](ref::wsl-settings). When possible, it is recommended to use WSL settings instead of directly modifying `.wslconfig` as it is simpler to use and more robust.

For changes to apply, you may need to run `wsl --shutdown` from PowerShell to shut down the WSL 2 VM and then restart your WSL instance.

> [Read more about `.wslconfig`](https://learn.microsoft.com/en-us/windows/wsl/wsl-config#wslconfig)

(ref::wsl.config)=

## wsl.config

Each unique instance can be configured using the `wsl.config` file located at `/etc/wsl.config` within a given WSL instance. Settings stored in this file are only applied to that instance.

To modify this file, enter the target WSL instance, and then open `/etc/wsl.config` in a text editor of your choice with admin permissions using `sudo`. For example, `sudo nano /etc/wsl.config`.

The `wsl.config` file can be used to configure instance-specific settings such as systemd support, automount settings, network settings, interoperability settings, and user settings.

```{warning}
Certain settings in `.wslconfig` are incompatible with specific Ubuntu features. For example, we generally recommend keeping systemd and interoperability enabled.

For additional information on how features like systemd and interoperability affect the functionality of Ubuntu on WSL, please see the explanation page [comparing WSL versions](explanation::wsl-version).
```

To apply changes made to a `wsl.config` file, you need to restart your WSL instances by running `wsl --shutdown` or `wsl --terminate <Instance Name>` from PowerShell. Confirm that the instance is no longer running with `wsl --list --running`.

> [Read more about `wsl.config`](https://learn.microsoft.com/en-us/windows/wsl/wsl-config#wslconf)

(ref::cloud-init)=

## Cloud-init

Cloud-init can be used to automatically set up instances of Ubuntu on WSL.
With cloud-init, you can pre-configure instances of a named Ubuntu distribution, such as by creating users, pre-installing software, or running arbitrary commands on initial startup.

```{important}
Cloud-init is only available when systemd and interoperability is enabled. For additional information, please reference the explanation page on how [WSL versions affect Ubuntu](explanation::wsl-version).
```

To use cloud-init, place your cloud-init files in `%UserProfile%\.cloud-init\` (create this folder if it does not exist), and name your cloud-init file `<Distro Name>.user-data`, replacing `<Distro Name>` with the name of the distribution that you want to configure.

```{note}
Cloud-init is only applied once, during the first startup of a WSL instance. Subsequent edits to the `.user-data` file will have no effect unless forcibly applied.
```

Finally, install that distribution as you would normally, and the cloud-init configuration should apply automatically.

> [Read more about how to use cloud-init with WSL](howto::cloud-init)

[//]: # "TODO for the Landscape release: Mention that if Landscape supplies a cloud-init file, it takes priority over any overlapping user-defined cloud-init file: cloudinit.readthedocs.io/en/latest/reference/datasources/wsl.html#user-data-configuration "

(ref::up4w)=

## Ubuntu Pro for WSL

Ubuntu Pro for WSL is a graphical application that automatically configures all compatible instances of Ubuntu on WSL to attach to your [Ubuntu Pro](https://ubuntu.com/pro) subscription.

> [Read more about using Pro for WSL](howto::up4w)

## Further reading

- [Microsoft WSL basic commands documentation](https://learn.microsoft.com/en-us/windows/wsl/basic-commands)
- [Microsoft WSL advanced configuration documentation](https://learn.microsoft.com/en-us/windows/wsl/wsl-config)
- [Cloud-init documentation](https://cloudinit.readthedocs.io/en/latest/)
