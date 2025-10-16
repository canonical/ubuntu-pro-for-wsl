---
myst:
  html_meta:
    "description lang=en":
      "For developers who are testing, debugging or developing the application."
---

# How to reset Ubuntu Pro for WSL back to factory settings

```{include} ../includes/dev_docs_notice.txt
    :start-after: <!-- Include start dev -->
    :end-before: <!-- Include end dev -->
```

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
