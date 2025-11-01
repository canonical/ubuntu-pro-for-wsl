---
myst:
  html_meta:
    "description lang=en":
      "For developers who are testing, debugging or developing the application."
---

# Install individual components of Ubuntu Pro for WSL for development

```{include} ../includes/dev_docs_notice.txt
    :start-after: <!-- Include start dev -->
    :end-before: <!-- Include end dev -->
```

This guide will show you how to install Pro for WSL for local development and testing.

<br/>

**Requirements:**

- A Windows machine with access to the internet
- Appx from the Microsoft Store:
  - Windows Subsystem For Linux
  - Either Ubuntu, Ubuntu 22.04, or Ubuntu (Preview)
- The Windows Subsystem for Windows optional feature enabled

## 1. Download the Windows Agent and the WSL Pro Service
<!-- TODO: Update when we change were artifacts are hosted -->
1. Go to the [repository actions page](https://github.com/canonical/ubuntu-pro-for-wsl/actions/workflows/qa-azure.yaml?query=branch%3Amain+).
2. Click the latest successful workflow run.
3. Scroll down past any warnings or errors, until you reach the Artifacts section.
4. Download:
    - Windows agent:    UbuntuProForWSL+...-production
    - wsl-pro-service:  Wsl-pro-service_...

Notice that, for the step above, there is also an alternative version of the MSIX bundle enabled for end-to-end testing. Most likely, that's not what you want to download.

(dev::install-agent)=
## 2. Install the Windows Agent

This is the Windows-side agent that manages the distros.

1. Uninstall Pro for WSL if you had installed previously:

    ```powershell
    Get-AppxPackage -Name CanonicalGroupLimited.UbuntuPro | Remove-AppxPackage
    ```

2. Follow the download steps to download UbuntuProForWSL
3. Unzip the artefact
4. Find the certificate inside. Install it into `Local Machine/Trusted people`.
5. Double click on the MSIX bundle and complete the installation.
6. The Firewall may ask for an exception. Allow it.
7. The GUI should show up. You’re done.

## 3. Install the WSL Pro Service

This is the Linux-side component that talks to the agent. Choose one or more distros Jammy or greater, and follow the instructions.

1. Uninstall the WSL-Pro-Service from your distro if you had it installed previously:

    ```bash
    sudo apt remove wsl-pro-service
    ```

2. Follow the download steps to download the WSL-Pro-Service.
3. Unzip the artifact.
4. Navigate to the unzipped directory containing the .deb file. Here is a possible path:

    ```bash
    cd /mnt/c/Users/<username>/Downloads/wsl-pro-service_*
    ```

5. Install the deb package.

    ```bash
    sudo apt install ./wsl-pro-service_*.deb
    ```

6. Ensure it works via systemd:

    ```bash
    systemctl status wsl-pro.service
    ```
