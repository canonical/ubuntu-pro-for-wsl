When the GUI starts, it attempts to establish a connection to the [UP4W Windows Agent](ref::up4w-windows-agent). If this fails, the agent is restarted. For troubleshooting purposes, you can restart the agent by first stopping the Windows process `ubuntu-pro-agent-launcher.exe` in Windows Task Manger or by issuing the following command in a PowerShell terminal:

```text
Stop-Process ubuntu-pro-agent-launcher.exe
```

You can then launch the GUI to complete the restart.
UP4W's Windows agent is a Windows application running in the background. It starts automatically when the user logs in to Windows. If it stops for any reason, it can be started by launching the UP4W GUI or running the executable from the terminal, optionally with `-vvv` for verbose logging:

```text
ubuntu-pro-agent.exe -vvv
```
