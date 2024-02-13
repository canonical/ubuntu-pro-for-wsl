(ref::up4w-windows-agent)=
# UP4W - Windows Agent

The Windows agent is a Windows application running in the background. It is started when the user first logs in to Windows. The GUI will also start it if it stops for some reason.

The Windows agent communicates with the different components to coordinate them:


![Diagram displaying the Windows agent communicating with the GUI, the Landscape server and the WSL-Pro-Service. It also reads the registry.](./assets/up4w-c4-windows-agent.png)
