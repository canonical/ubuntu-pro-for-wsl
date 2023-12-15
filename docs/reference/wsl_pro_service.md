# WSL Pro service

A systemd unit running inside Ubuntu WSL instances allowing them to communicate
with the Windows host [background agent](windows_agent) and execute actions
requested by it, such as pro-attaching the instance or configuring the Landscape
client.
