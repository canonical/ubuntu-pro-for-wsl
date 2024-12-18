# Ubuntu Pro for WSL (UP4W)
```{note}
This documentation describes a future release of UP4W. UP4W is not yet generally available in the Microsoft Store.
```

Ubuntu Pro for WSL (UP4W) is a powerful automation tool for managing [WSL](https://ubuntu.com/desktop/wsl) on Windows. If you are responsible for a fleet of Windows devices, UP4W will enable you to monitor, customise and secure Ubuntu WSL instances at scale.

UP4W is designed to achieve close integration with applications for customising images, enforcing standards and validating security compliance. WSL instances can be created, removed and monitored with [Landscape](https://ubuntu.com/landscape). Microsoft Defender is WSL-aware, making it easy to confirm if instances are compliant. [Cloud-init](https://cloudinit.readthedocs.io/en/latest/) support is built-in, allowing efficient customisation of standard images.

Once you have an [Ubuntu Pro](https://ubuntu.com/pro) subscription, adding your Pro token to UP4W on a Windows host will add that token to all connected WSL instances with the Ubuntu Pro client installed. When the Landscape client is installed on the host, any connected WSL instances will be auto-enrolled in Landscape. WSL instances can then be remotely created, provisioned and managed from the Windows host.

WSL is preferred by many organisations as a solution to run a fully-functioning Linux environment on a Windows machine. UP4W empowers system administrators and corporate security teams to manage large numbers of WSL instances effectively.

Read our [getting started tutorial](tutorial/getting-started) to begin.

## In this documentation

````{grid} 1 1 2 2

```{grid-item-card} [Tutorials](tutorial/index)
:link: tutorial/index
:link-type: doc

**Start here** with hands-on tutorials for new users, guiding you through your first-steps
```

```{grid-item-card} [How-to guides](howto/index)
:link: howto/index
:link-type: doc

**Follow step-by-step** instructions for key operations and common tasks
```

````

````{grid} 1 1 2 2

```{grid-item-card} [Reference](reference/index)
:link: reference/index
:link-type: doc

**Read technical descriptions** of important factual information relating to UP4W
```

```{grid-item-card} [Explanation](explanation/index)
:link: explanation/index
:link-type: doc

**Read an explanation** of UP4W's system architecture
```

````

## Project and community

UP4W is a member of the Ubuntu family. It’s an open-source project that warmly welcomes community contributions, suggestions, fixes and constructive feedback. Check out our [contribution page](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/CONTRIBUTING.md) on GitHub in order to bring ideas, report bugs, participate in discussions and much more!

Thinking about using UP4W for your next project? Get in touch!

```{toctree}
:hidden:
:titlesonly:

UP4W <self>
Tutorial </tutorial/index>
How-to guides </howto/index>
Reference </reference/index>
Explanation </explanation/index>
```
