---
myst:
  html_meta:
    "description lang=en":
      "How to build and test the code for Ubuntu Pro for WSL."
---

# Contributing to the code

We anticipate that most code contributions will be for the Pro for WSL
application. Information helpful for the development of this application is
included below.

## Prerequisite: a Windows machine

To test and debug Ubuntu on WSL and Pro for WSL you will need a Windows
machine with WSL installed.

It is possible to run Ubuntu on WSL in a VM with nested visualisation but we do
not recommend this as a testing environment.

## Additional dependencies for Ubuntu Pro for WSL

* Ubuntu 26.04 LTS installed in WSL
* Visual Studio Community 2022 or above
* Go
* Flutter
* An Ubuntu Pro token

## The test suite for Ubuntu Pro for WSL

The source code includes a comprehensive test suite made of unit and integration tests. All the tests must pass with and without the race detector.

Each module has its own package tests and you can also find the integration tests at the appropriate end-to-end (e2e) directory.

The test suite must pass before merging the PR to our main branch. Any new feature, change or fix must be covered by corresponding tests.

## Building and running the binaries for Ubuntu Pro for WSL

For building, you can use the following two scripts:

* [Build the WSL Pro Service](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/tools/build/build-deb.sh)
* [Build the Windows Agent](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/tools/build/build-appx.ps1)

:::{note}
You'll need to [create a self-signing certificate](https://learn.microsoft.com/en-us/windows/msix/package/create-certificate-package-signing) to build the Windows Agent.
:::

## Install individual components of Ubuntu Pro for WSL for development

This guide will show you how to install Pro for WSL for local development and testing.

**Requirements:**

- A Windows machine with access to the internet
- The Windows Subsystem for Linux optional feature enabled
- One or more instances of Ubuntu on WSL installed

### 1. Download the Windows Agent and the WSL Pro Service
<!-- TODO: Update when we change were artifacts are hosted -->
1. Go to the [repository actions page](https://github.com/canonical/ubuntu-pro-for-wsl/actions/workflows/qa-azure.yaml?query=branch%3Amain+).
2. Click the latest successful workflow run.
3. Scroll down past any warnings or errors, until you reach the Artifacts section.
4. Download:
    - Windows agent:    UbuntuProForWSL+...-production
    - wsl-pro-service:  Wsl-pro-service_...

Notice that, for the step above, there is also an alternative version of the MSIX bundle enabled for end-to-end testing. Most likely, that's not what you want to download.

(dev::install-agent)=
### 2. Install the Windows Agent

This is the Windows-side agent that manages the distros.

1. Uninstall Pro for WSL if you had installed previously:

    ```{code-block} powershell
    > Get-AppxPackage -Name CanonicalGroupLimited.UbuntuPro | Remove-AppxPackage
    ```

2. Follow the download steps to download UbuntuProForWSL
3. Unzip the artefact
4. Find the certificate inside. Install it into `Local Machine/Trusted people`.
5. Double click on the MSIX bundle and complete the installation.
6. The Firewall may ask for an exception. Allow it.
7. The GUI should show up. You’re done.

### 3. Install the WSL Pro Service

This is the Linux-side component that talks to the agent. Choose one or more distros Jammy or greater, and follow the instructions.

1. Uninstall the WSL-Pro-Service from your distro if you had it installed previously:

    ```{code-block} text
    $ sudo apt remove wsl-pro-service
    ```

2. Follow the download steps to download the WSL-Pro-Service.
3. Unzip the artifact.
4. Navigate to the unzipped directory containing the .deb file. Here is a possible path:

    ```{code-block} text
    $ cd /mnt/c/Users/<username>/Downloads/wsl-pro-service_*
    ```

5. Install the deb package.

    ```{code-block} text
    $ sudo apt install ./wsl-pro-service_*.deb
    ```

6. Ensure it works via systemd:

    ```{code-block} text
    $ systemctl status wsl-pro.service
    ```

## Restart Ubuntu Pro for WSL during development

Some configuration changes only apply when you restart Ubuntu Pro for WSL. Here is a guide on how to restart it. There are two options.

### Option 1: Restart your Ubuntu Pro for WSL host machine

This is the simple one. If you're not in a hurry to see the configuration updated, just wait until next time you boot your machine.

### Option 2: Restart only Ubuntu Pro for WSL

1. Stop the agent:

    ```powershell
    Get-Process -Name Ubuntu-Pro-Agent | Stop-Process
    ```

2. Stop the distro, or distros you installed WSL-Pro-Service in:

    ```powershell
    wsl --terminate DISTRO_NAME_1
    wsl --terminate DISTRO_NAME_2
    # etc.

    # Alternatively, stop all distros:
    wsl --shutdown
    ```

7. Start the agent again:
    1. Open the start Menu and search for "Ubuntu Pro for WSL".
    2. The GUI should start.
    3. Wait a minute.
    4. Click on "Click to restart it".
8. Start the distro, or distros you installed WSL-Pro-Service in.

## Access Ubuntu Pro for WSL logs for debugging

At some point you may want to read the Ubuntu Pro for WSL logs, most likely for debugging purposes. The agent and the service store their logs separately. This guide shows you where to find each of the logs.

### Access the logs for the WSL Pro service

To access the logs of a specific distribution's WSL-Pro-Service, you must first launch the distribution and then query the journal:

```bash
journalctl -u wsl-pro.service
```

For more information on using the journal, you can check out its man page with `man journalctl` or [online](https://man7.org/linux/man-pages/man1/journalctl.1.html).

These logs may be insufficient for proper debugging, so you may be interested in looking at the agent's logs as well.

### Access the logs for the Windows Agent

To access the logs for the Windows Agent:

1. Go to your home directory
   - Open the file explorer
   - Write `%USERPROFILE%` at the address
2. In the home directory, find the `.ubuntupro` directory and double-click on it.
2. In the `.ubuntupro` folder, find file `log` and open it with any text editor.
   - This file contains the logs sorted with the oldest entries at the top and the newest at the bottom.

## Enable opt-in features of Ubuntu Pro for WSL during development

Some features in Ubuntu Pro for WSL are opt-in or can be toggled on and off via the Windows Registry.
While the code is arranged such that CI always tests with those features enabled,
when running Pro for WSL on your machine, you may need to toggle them on and off
explicitly via the Windows registry. This guide shows you how to do that.

### Enable subscribing via the Microsoft Store

1. Open the Windows registry editor: press `Win + R`, type `regedit` and press `Enter`.
1. Navigate to the following key: `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro\`.
1. Create a new DWORD value named `AllowStorePurchase` and set it to `1`.

The next time you open the GUI you'll find the button to subscribe to Ubuntu
Pro via the Microsoft Store.

```{warning}
This can incur real charges if you proceed with the purchase.
```

### Disable Landscape configuration in the GUI

Landscape configuration page and related buttons are enabled by default, but can be disabled via registry.

1. Open the Windows registry editor: press `Win + R`, type `regedit` and press `Enter`.
1. Navigate to the following key: `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro\`.
1. Create a new DWORD value named `LandscapeConfigVisibility` and set it to `0`.

The next time you open the GUI, you'll find that the Landscape configuration
page can be shown via the set up wizard or by clicking on the 'Configure
Landscape' button.
If that value is not present or set to anything other than `0`, Landscape configuration page
and related buttons will be visible.

## Reset Ubuntu Pro for WSL to factory settings

You can reset Ubuntu Pro for WSL to factory settings following these steps:

1. Shut down WSL
   ```powershell
   wsl --shutdown
   ```
2. Uninstall the package and shut down WSL:

    ```powershell
    Get-AppxPackage -Name "CanonicalGroupLimited.UbuntuPro" | Remove-AppxPackage`
    ```
3. Remove the public directory:
    ```powershell
    Remove-Item -Recurse -Force "${env:UserProfile}\.ubuntupro\"
    ```
4. Remove the registry key:
   1. Press Win+R
   2. Type `regedit.exe` and click OK
   3. Write `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro` at the address bar
      - If this fails, you are done (the key does not exist).
   4. Find the `UbuntuPro` key on the left
   5. Right-click on it
   6. Click delete
5. Install the Windows Agent package again (see the section on [how to install](dev::install-agent)). You do not need to re-install the WSL-Pro-Service.
6. You're done. Next time you start the GUI it'll be like a fresh install.
