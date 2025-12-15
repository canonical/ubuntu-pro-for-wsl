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

### Uninstall the Pro for WSL app

Go to `Settings > Apps > Installed Apps`, locate the "Ubuntu Pro for WSL"
application, right-click on it and select **Uninstall**.

If you installed the app using WinGet, you can instead run the following command in PowerShell:

```{code-block} text
> winget uninstall Canonical.UbuntuProforWSL
```

### Remove Pro for WSL data

Remove `.ubuntupro` from your Windows user profile directory.

```{code-block} powershell
> Remove-Item -Recurse -Force C:\Users\<username>\.ubuntupro
```

### Remove registry key

If you or your organisation used the Windows Registry to store any configuration data, such as
a Pro token, remove the matching registry key:

```{code-block} powershell
> Remove-Item -Recurse -Force HKCU:\Software\Canonical\UbuntuPro
```

(howto::uninstall-ubuntu-wsl)=
## Ubuntu distros

### Stop WSL

In PowerShell, run the following command to stop WSL:

```{code-block} text
> wsl --shutdown
```

### Uninstall Ubuntu distros

The method to uninstall an Ubuntu distro depends on the installation format.

For installations that use the modern tar-based installation format, run:

```{code-block} text
> wsl --unregister <distro>
```

If a distribution was installed in the legacy format, go to `Settings > Apps >
Installed Apps`, locate the Ubuntu distro, right-click on it, and select
**Uninstall**.

````{tip}
While installing Ubuntu in the legacy format, a message appears in the terminal
recommending the modern tar-based format.

If you don't know the format of an installed distro, run:

```{code-block} powershell
> Get-ChildItem "HKCU:\Software\Microsoft\Windows\CurrentVersion\Lxss"
```

Distros installed using the tar-based format include the property `modern` with
value `1`.
````

(howto::uninstall-wsl)=
## WSL app

Only uninstall WSL if you no longer need WSL on your Windows machine:

```{code-block} text
> wsl --uninstall
```

```{note}
Running `wsl --install` for the first time enables the Virtual Machine
Platform, if it is disabled. Uninstalling WSL does not disable the Virtual
Machine Platform, and it needs to be disabled manually.
```
