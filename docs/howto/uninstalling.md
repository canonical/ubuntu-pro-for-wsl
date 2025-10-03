---
myst:
  html_meta:
    "description lang=en":
      "Follow the steps to uninstall the Ubuntu Pro for WSL Windows application, Ubuntu on WSL instances and WSL itself."
---

# Uninstall Ubuntu Pro for WSL, Ubuntu on WSL and WSL

This page briefly outlines the steps required to uninstall Ubuntu Pro for WSL,
Ubuntu distros, and WSL itself.

(howto::uninstall-up4w)=
## Ubuntu Pro for WSL

Go to `Settings > Apps > Installed Apps`, locate the "Ubuntu Pro for WSL"
application, right-click on it and select **Uninstall**.

You should also remove `.ubuntupro` from your Windows user profile directory.

```text
> Remove-Item -Recurse -Force C:\Users\<username>\.ubuntupro
```

(howto::uninstall-ubuntu-wsl)=
## Ubuntu distros

In PowerShell, run the following command to stop WSL:

```text
> wsl --shutdown
```

The method to uninstall an Ubuntu distro depends on the installation format.

For installations that use the modern tar-based installation format, run:

```text
PS C:\Users\me> wsl --unregister <distro>
```

If a distribution was installed in the legacy format, go to `Settings > Apps >
Installed Apps`, locate the Ubuntu distro, right-click on it, and select
**Uninstall**.

````{tip}
While installing Ubuntu in the legacy format, a message appears in the terminal
recommending the modern tar-based format.

If you don't know the format of an installed distro, run:

```text
PS C:\Users\me> Get-ChildItem "HKCU:\Software\Microsoft\Windows\CurrentVersion\Lxss"
```

Distros installed using the tar-based format include the property `modern` with
value `1`.

````

(howto::uninstall-wsl)=
## WSL app

Only do this if you no longer need WSL on your Windows machine:

```text
PS C:\Users\me> wsl --uninstall
```

```{note}
Running `wsl --install` for the first time enables the Virtual Machine
Platform, if it is disabled. Uninstalling WSL does not disable the Virtual
Machine Platform, and it needs to be disabled manually.
```
