# How to enable Ubuntu Pro with UP4W

## Manual pro enablement
1. Go to your [Ubuntu Pro dashboard](https://ubuntu.com/pro) to get your Ubuntu Pro token.
2. Go to the Windows menu, and search and click Ubuntu Pro For Windows. If it does not show up, your installation of the agent went wrong.
3. Click on "I already have a token".
4. Introduce the token you got from your Pro dashboard, and click "Apply".
5. That's it. All new and existing distros with the WSL-Pro-Service installed will be pro-attached. You can verify it by starting any WSL distro with WSL-Pro-Service installed, and running:
    ```bash
    pro status
    ```

## Organisational pro enablement
1. Find registry key `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro`, field `ProTokenOrg`.
2. Write your Ubuntu Pro Token into the registry key
3. The changes will take effect next time you start Ubuntu Pro For Windows.  All new distros will automatically become pro-enabled. If you want them to be applied now, follow the steps on how to restart Ubuntu Pro For Windows. Otherwise, you're done.