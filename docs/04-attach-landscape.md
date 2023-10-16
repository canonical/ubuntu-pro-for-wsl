# How to auto-register WSL distros to Landscape with UP4W
You can use a private Landscape instance (different from [landscape.canonical.com](https://landscape.canonical.com)). It must be over HTTP, as using certificates is not yet supported. To do so, follow these steps:
1. Press Windows+R.
2. Write `regedit.exe` and enter.
3. Go to `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro`.
4. There are two relevant fields:
    - `LandscapeAgentURL` should contain the URL where the Landscape Host-agent server is hosted.
    - `LandscapeClientConfig` should contain the contents of the YAML file with the settings, such as the [example from the Landscape repository](https://github.com/canonical/landscape-client/blob/master/example.conf).
5. To edit any of the fields, right-click and Edit.
6. If you need more than one line, delete the field and create a new one with the same name, and type `Multi-String Value`.
7. The changes will take effect next time you start the machine. If you want them to be applied now, follow the next steps. Otherwise, you're done. All new distros will automatically attach to Landscape.
8. Stop the agent:
    ```powershell
    Get-Process -Name Ubuntu-Pro-Agent | Stop-Process
    ```
9. Start the agent again:
    1. Open the start Menu and search for "Ubuntu Pro For Windows".
    2. The GUI should start.
    3. Wait a minute.
    4. Click on "Click to restart it".
10. Stop the distro you installed WSL-Pro-Service in:
    ```powershell
    wsl --terminate DISTRO_NAME
    ```
11. Start the distro you installed WSL-Pro-Service in.
12. You should see a new "pending computer authorisation" in your Landscape dashboard.