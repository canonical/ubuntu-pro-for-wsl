---
myst:
  html_meta:
    "description lang=en":
      "Multiple distributions of Ubuntu are available for WSL, including the latest Ubuntu LTS release and the latest development version which previews new features."
---

(reference::distros)=
# Distributions of Ubuntu on WSL


Our flagship distribution (distro) is Ubuntu. It is the default option when you install WSL for the first time. Several releases of the Ubuntu distro are available for WSL.

(reference::releases)=
## Releases of Ubuntu on WSL

```{admonition} Interim releases
:class: seealso
Interim releases of Ubuntu are currently not supported on WSL.
```

These are the releases of Ubuntu that we support for WSL and that are available on the Microsoft Store:

- [Ubuntu](https://apps.microsoft.com/detail/9PDXGNCFSCZV?hl=en-us&gl=US) ships the latest stable LTS (Long Term Support) release of Ubuntu. When new LTS versions are released, this release of Ubuntu can be upgraded once the first point release is available.
- Numbered releases --- for example, [Ubuntu 22.04 LTS](https://apps.microsoft.com/detail/9PN20MSR04DW?hl=en-us&gl=US) --- refer to specific Long Term Stability (LTS) releases that receive standard support for five years. For more information on LTS releases, support and timelines, visit the [Ubuntu releases page](https://wiki.ubuntu.com/Releases). Numbered releases of Ubuntu on WSL will not be upgraded unless configured to upgrade in `etc/update-manager/release-upgrades`.
- [Ubuntu (Preview)](https://apps.microsoft.com/detail/9P7BDVKVNXZ6?hl=en-us&gl=US) is a daily build of the latest development version of Ubuntu, which previews new features as they are developed. It does not receive the same level of QA as stable releases and should not be used for production workloads.

```{tip}
Ubuntu 24.04 LTS is available in the [new WSL distro
format](https://ubuntu.com/blog/ubuntu-wsl-new-format-available), which can be
installed directly from [ubuntu.com/wsl](https://ubuntu.com/desktop/wsl)
without the Microsoft Store.
```

(naming)=
## Naming

Depending on context, releases of Ubuntu are referred to by different names: 

1. **App name** is the name of the application for an Ubuntu release that appears in the Microsoft Store or as FRIENDLY NAME when you run the `wsl -l -v` command.
2. **AppxPackage name** is the name that can be passed to `Get-AppxPackage -Name` in PowerShell to get information about an installed package.
3. **Distro name** is the NAME logged when you invoke `wsl -l -v` to list installed releases of Ubuntu.
4. **Executable name** is the program you need to run to start the Ubuntu distro.

```{note}
WSL distros are transitioning from an appx-based architecture to a tar-based architecture.
The prior architecture involved building WSL distros as an AppxPackage; after installation,
they could be run with `<distro name>.exe`.

The more recent tar-based distros are available as images with the `.wsl` extension and must be run
with `wsl -d <distro name>`.
```

These naming conventions are summarised in the table below:

| App name             | AppxPackage name                       | Distro name      | Executable name     |
| -------------------- | -------------------------------------- | ---------------- | ------------------- |
| `Ubuntu`             | `CanonicalGroupLimited.Ubuntu`         | `Ubuntu`         | `ubuntu.exe`        |
| `Ubuntu (Preview)`   | `CanonicalGroupLimited.UbuntuPreview`  | `Ubuntu-Preview` | `ubuntupreview.exe` |
| `Ubuntu XX.YY.Z LTS` | `CanonicalGroupLimited.UbuntuXX.YYLTS` | `Ubuntu-XX.YY`   | `ubuntuXXYY.exe`    |

```{admonition} The WSL kernel
:class: important
The kernel used in WSL environments is maintained by Microsoft.
Bug reports and support requests for the WSL kernel should be directed to the [official repository for the WSL kernel](https://github.com/microsoft/WSL2-Linux-Kernel).
```
