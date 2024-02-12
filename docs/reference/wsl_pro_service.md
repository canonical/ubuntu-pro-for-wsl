# WSL Pro service

A `systemd` unit running inside each Ubuntu WSL instance. It communicates
with the Windows host [background agent](windows_agent) and executes actions
requested by the former, such as pro-attaching the instance or configuring the Landscape
client.
