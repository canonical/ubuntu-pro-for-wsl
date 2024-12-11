When the GUI starts, it attempts to establish a connection to the [UP4W Windows Agent](ref::up4w-windows-agent). If this fails, the agent is restarted. For troubleshooting purposes, you can restart the agent by first stopping the Windows process `ubuntu-pro-agent-launcher.exe` in Windows Task Manger or by issuing the following command in a PowerShell terminal:

```text
Stop-Process ubuntu-pro-agent-launcher.exe
```

You can then launch the GUI to complete the restart.
