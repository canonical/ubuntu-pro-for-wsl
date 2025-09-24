---
myst:
  html_meta:
    "description lang=en": "Reference information on configuring WSL instances."
---

# WSL instance configuration

This page includes reference materials on configuring your WSL instances.

## Instance configuration

There are a number of different ways to configure WSL instances, each with a different scope and use-case. For the basic use-case of using Ubuntu with WSL, the default configuration should be sufficient. However, certain use-cases may find the flexibility provided by these advanced configuration options to be useful.

| Method                            | Scope                                              | Location                                                                            |
| --------------------------------- | -------------------------------------------------- | ----------------------------------------------------------------------------------- |
| [Ubuntu Pro for WSL](ref::up4w)   | Ubuntu Pro attachment management between instances | [Graphical application ](howto::up4w)                                               |
| [WSL Settings](ref::wsl-settings) | General settings that apply to all of WSL          | Graphical application named `WSL Settings`                                          |
| [`.wslconfig`](ref::.wslconfig)   | General settings that apply to all of WSL          | `%UserProfile%\.wslconfig`, outside of a WSL instance                               |
| [`wsl.config`](ref::wsl.config)   | Settings for specific WSL instance only            | `/etc/wsl.conf`, while inside a WSL instance                                        |
| [Cloud-init](ref::cloud-init)     | Ubuntu provisioning settings                       | `.userdata` files located at `\UserProfile%\.cloud-init\` outside of a WSL instance |

(ref::up4w)=

### Ubuntu Pro for WSL

Ubuntu Pro for WSL (UP4W) is a graphical application that allows you to automatically configure and attach Ubuntu WSL instances to your [Ubuntu Pro](https://ubuntu.com/pro) subscription.

> [Read more about using Ubuntu Pro for WSL](howto::up4w)

(ref::wsl-settings)=

### WSL Settings

WSL Settings is a graphical application that comes with WSL, allowing you to manage general settings that apply to all WSL 2 instances. It is analogous to the [`.wslconfig` file](ref::.wslconfig).

The WSL Settings application can be used to configure things like hardware resource limits, networking, and custom kernels.

For changes to apply, you may need to run `wsl --shutdown` from PowerShell to shut down the WSL 2 VM and then restart your WSL instance.

(ref::.wslconfig)=

### .wslconfig

Global configuration settings that apply to all WSL 2 instances can be managed using the `.wslconfig` file located at `%UserProfile%\.wslconfig`. By default, `.wslconfig` does not exist, and it may need to be created manually.

The `.wslconfig` file is analogous to the graphical application [WSL Settings](ref::wsl-settings). When possible, it is recommended to use WSL settings instead of directly modifying `.wslconfig`.

For changes to apply, you may need to run `wsl --shutdown` from PowerShell to shut down the WSL 2 VM and then restart your WSL instance.

> [Read more about `.wslconfig`](https://learn.microsoft.com/en-us/windows/wsl/wsl-config#wslconfig)

(ref::wsl.config)=

### wsl.config

Local, per instance configuration can be done with the `wsl.config` file located at `/etc/wsl.config` within a given WSL instance. Settings stored in this file are only applied to the instance that the `wsl.config` file belongs to.

To modify this file, enter the target WSL instance, and then open `/etc/wsl.config` in a text editor of your choice with admin permissions using `sudo`. For example, `sudo nano /etc/wsl.config`.

The `wsl.config` file can be used to configure instance-specific settings such as systemd support, automount settings, network settings, interoperability settings, and user settings.

```{warning}
Certain settings in `.wslconfig` are incompatible with certain Ubuntu features. For instance, we recommend keeping systemd and interoperability enabled.

For additional information on how features like systemd and interoperability as well as WSL versions may affect Ubuntu, please see the explanation page [comparing WSL versions](explanation::wsl-version)
```

To apply changes made to a `wsl.config` file, you need to restart your WSL instances by running `wsl --shutdown` or `wsl --terminate <Instance Name>` from PowerShell. Confirm that the instance is no longer running with `wsl --list --running`

> [Read more about `wsl.config`](https://learn.microsoft.com/en-us/windows/wsl/wsl-config#wslconf)

(ref::cloud-init)=

### Cloud-init

Cloud-init is a cross-platform tool for provisioning cloud instances and can be optionally used to automatically set up instances of Ubuntu on WSL.
With cloud-init, you can pre-configure things specific to a new Ubuntu instance, such as creating users, pre-installing software, or running arbitrary commands on initial startup.

```{important}
Cloud-init is only available with WSL 2 with systemd and interoperability enabled. For additional information, please reference the explanation page on how [WSL versions affect Ubuntu](explanation::wsl-version).
```

```{note}
Cloud-init is only applied once, during the first startup of a WSL instance. Subsequent edits to the `.user-data` file will have no effect unless forcibly applied.
```

To use cloud-init, place your cloud-init files in `%UserProfile%\.cloud-init\` of the user home directory (create this folder if it does not exist), and name your cloud-init file `<Distro Name>.user-data`, replacing `<Distro Name>` with the name of the distribution that you are making.

Afterwards, use the text editor of your choice to edit the `.user-data` file you just created, adding your desired cloud-init profile.

Finally, install Ubuntu as you would normally, cloud-init should apply automatically.

> [Read more about how to use cloud-init with WSL](howto::cloud-init)

## Further reading

- [Microsoft WSL basic commands documentation](https://learn.microsoft.com/en-us/windows/wsl/basic-commands)
- [Microsoft WSL advanced configuration documentation](https://learn.microsoft.com/en-us/windows/wsl/wsl-config)
- [Cloud-init documentation](https://cloudinit.readthedocs.io/en/latest/)
