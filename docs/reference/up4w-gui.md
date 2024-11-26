(ref::up4w-gui)=
# Application GUI

UP4W has a small GUI to help users with:
- Providing or acquiring an [Ubuntu Pro token](ref::ubuntu-pro-token).
- Providing the [Landscape configuration](ref::landscape-config).

![Image of the Ubuntu Pro for WSL GUI.](./assets/up4w-gui.png)

## Interaction between the GUI and the agent

When the GUI starts, it attempts to establish a connection to the [UP4W Windows Agent](ref::up4w-windows-agent). If this fails, the agent is restarted. For troubleshooting purposes, you can restart the Agent by stopping the Windows process `ubuntu-pro-agent-launcher.exe` and starting the GUI.
