(reference::distros)=
# Distributions

Our flagship distribution (distro) is Ubuntu. It is the default option when you install WSL for the first time. Several releases of the Ubuntu distro are available for WSL.

Each release of Ubuntu for WSL is available as an application from the Microsoft Store. Once a release is [installed](https://documentation.ubuntu.com/wsl/en/latest/howto/install-ubuntu-wsl2/#method-1-microsoft-store-application), it will be available to use in your WSL environment.

- [Ubuntu](https://apps.microsoft.com/detail/9PDXGNCFSCZV?hl=en-us&gl=US) ships the latest stable LTS release of Ubuntu. When new LTS versions are released, Ubuntu can be upgraded once the first point release is available.
- [Ubuntu 18.04 LTS](https://apps.microsoft.com/detail/9PNKSF5ZN4SW?hl=en-us&gl=US), [Ubuntu 20.04 LTS](https://apps.microsoft.com/detail/9MTTCL66CPXJ?hl=en-us&gl=US), and [Ubuntu 22.04 LTS](https://apps.microsoft.com/detail/9PN20MSR04DW?hl=en-us&gl=US) are the LTS versions of Ubuntu and receive updates for five years. Upgrades to future LTS releases will not be proposed.
- [Ubuntu (Preview)](https://apps.microsoft.com/detail/9P7BDVKVNXZ6?hl=en-us&gl=US) is a daily build of the latest development version of Ubuntu previewing new features as they are developed. It does not receive the same level of QA as stable releases and should not be used for production workloads.

(naming)=
## Naming

Due to different limitations in different contexts, these applications will have different names in different contexts. Here is a table matching them.

1. App name is the name you'll see in the Microsoft Store and Winget.
2. AppxPackage is the name you'll see in `Get-AppxPackage`.
3. Distro name is the name you'll see when doing `wsl -l -v` or `wsl -l --online`.
4. Executable is the program you need to run to start the distro.

| App name             | AppxPackage name                       | Distro name      | Executable          |
| -------------------- | -------------------------------------- | ---------------- | ------------------- |
| `Ubuntu`             | `CanonicalGroupLimited.Ubuntu`         | `Ubuntu`         | `ubuntu.exe`        |
| `Ubuntu (Preview)`   | `CanonicalGroupLimited.UbuntuPreview`  | `Ubuntu-Preview` | `ubuntupreview.exe` |
| `Ubuntu XX.YY.Z LTS` | `CanonicalGroupLimited.UbuntuXX.YYLTS` | `Ubuntu-XX.YY`   | `ubuntuXXYY.exe`    |
