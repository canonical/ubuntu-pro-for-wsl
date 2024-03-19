(ref::up4w-wsl-pro-service)=
# UP4W - WSL Pro service

A `systemd` unit running inside every Ubuntu WSL instance. The [Windows Agent](ref::up4w-windows-agent) running on the Windows host sends commands that the WSL Pro Service executes, such as [pro-attaching](ref::pro-attach) or configuring the [Landscape client](ref::landscape-client).


You can check the current status of the WSL Pro Service in any particular distro with:
```bash
systemctl status wsl-pro.service
```
