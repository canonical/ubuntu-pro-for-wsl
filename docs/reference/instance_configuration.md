---
myst:
  html_meta:
    "description lang=en": "Reference information on configuring WSL instances."
---

# WSL instance configuration

This page includes reference materials on the basics of managing and configuring your WSL instances.

## Basic instance management

### List installed instances

To get a list of installed instances, their present state, and the WSL version the instance is using, run:

```
> wsl -l -v
```

### Install an instance

To install an instance using a `.wsl` [file](https://ubuntu.com/desktop/wsl), run the following command:

```
> wsl --install --from-file <File Name>
```

Additional options of note include:

- `--name <Instance Name>`: Specify the unique name of the instance
- `--no-launch`: Install the instance, but do not launch it automatically afterward
- `--version <Version Number>`: Specify the [WSL version](../explanation/compare-wsl-versions.md)

```{note}
For additional methods on WSL instances, please reference the [WSL install how-to guide](../howto/02-install.md).
```

### Set an instance as the default

WSL commands that are run without specifying the instance with `--distribution` or `-d` utilize the default instance. To set it, run:

```
> wsl --set-default <Instance Name>
```

### Terminate an instance

To terminate a specific instance, stopping it from running, run:

```
> wsl --terminate <Instance Name>
```

```{note}
Termination of an instance does not also stop backend virtual machine behind WSL. To do so, all running instances need to be stopped with `wsl --shutdown`.
```

### Unregister or remove an instance

To remove and unregister an instance, removing all data, software, and settings associated with that instance, run:

```
> wsl --unregister <Instance Name>
```

Afterward, running `wsl --list` will no longer list that instance, and everything associated with it would have been removed.

### Set WSL version

To set the [WSL version](../explanation/compare-wsl-versions.md) for an already registered instance, run:

```
> wsl --set-version <Instance Name> <Version Number>
```

```{warning}
Switching between WSL versions can not only be time-consuming, but cause unexpected issues due to the architectural differences. Consider backing up your files before attempting a switch.
```

## Advanced settings configuration

### UP4W

### Windows GUI Application

### wsl.config

### .wslconfig

### Cloud-init

Cloud-init is a cross-platform tool for provisioning cloud instances.
It is an industry standard and can now also be used to automatically setup instances of Ubuntu on WSL.

```{note}
Cloud-init is only available with WSL 2 with Systemd and interoperability enabled. For additional information, please reference the explanation page on how [WSL versions affect Ubuntu](../explanation/compare-wsl-versions.md).
```

## Further reading

- [Microsoft WSL basic commands documentation](https://learn.microsoft.com/en-us/windows/wsl/basic-commands)
- [Microsoft WSL advanced configuration documentation](https://learn.microsoft.com/en-us/windows/wsl/wsl-config)
- [Cloud-init documentation](https://cloudinit.readthedocs.io/en/latest/)
