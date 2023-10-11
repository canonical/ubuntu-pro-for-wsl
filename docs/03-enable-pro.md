# How to enable Ubuntu Pro with UP4W

## Manual pro enablement
1. Go to your [Ubuntu Pro dashboard](https://ubuntu.com/pro/dashboardand) to get your Ubuntu Pro token.
2. Go to the Windows menu, and search and click Ubuntu Pro For Windows. If it does not show up, your installation of the agent went wrong.
3. Click on "I already have a token".
4. Introduce the token you got from your Pro dashboard, and click "Apply".
5. That's it. All new and existing distros with the WSL-Pro-Service installed will be pro-attached. You can verify it by starting any WSL distro with WSL-Pro-Service installed, and running:
    ```bash
    pro status
    ```

