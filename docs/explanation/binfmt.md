---
myst:
  html_meta:
    "description lang=en":
      "Explains why systemd-binfmt.service used to affect the WSL experience, and what to do if that still happens."
---

# Support for miscellaneous binary formats (binfmt_misc) with Ubuntu on WSL

A key feature of  WSL is binary interoperability, which allows
Windows binaries to be run inside WSL and vice-versa. Linux can run Windows binaries thanks to `binfmt_misc`, a
capability offered by the Linux kernel. With `binfmt_misc`, arbitrary executable file formats can be recognised and
passed to certain user space applications, such as interpreters, emulators and virtual machines, which can then
execute that specific format. The executable formats are registered via a file interface, usually located at
`/proc/sys/fs/binfmt_misc/register` or using wrappers such as those offered by `binfmt-support` or
`systemd-binfmt.service`. WSL registers Windows binaries to be passed to the `/init` program, which knows how
to pass that along to Windows (check `/proc/sys/fs/binfmt_misc/WSLInterop*` in a WSL distro instance).

Historically, the systemd unit `systemd-binfmt.service` was known to break WSL binary
interoperability, so it was disabled for Ubuntu on WSL. This documentation explains these issues
with WSL binary interoperability, why they no longer apply in more recent versions of Ubuntu and
WSL, and what you can do if you still have problems.

## The systemd-binfmt.service and Windows binary interoperability

With systemd enabled, `systemd-binfmt.service` runs during boot, reads configuration files from specific
directories and registers additional executable formats with the kernel. All registrations are removed when
the service stops. If that service is not aware of the WSL registration mechanism, Windows binary
interoperability can break due to different factors, including:

- `binfmt_misc` mount point being shared by multiple distro instances cause interoperability to be broken for
multiple instances when one is shutdown. Since the WSL distro instances are effectively containers sharing the
same kernel, when a distro instance stops and `systemd-binfmt.service` quits, it can break the registration
for other instances.

- Startup ordering. If WSL didn't order its own registration after that service, the service would break WSL
interoperability at boot time.

- Service restart. Restarting `systemd-binfmt.service` implies unregistering and re-registering executable
file formats. That can easily happen without the user being aware, such as when installing emulators or other
packages that rely on `binfmt_misc`.

## WSL protection of binfmt registrations

The scenarios above were reported by users in previous versions of WSL. The developers have since
implemented numerous improvements. From version 2.5.7, WSL is capable of restoring its binfmt
registration at startup and when that service is restarted. This was achieved by implementing a systemd generator that
recreates the WSL binfmt registration whenever the `systemd-binfmt.service` unit runs, including
system startup and manual restarts. That protection is immune to `systemctl daemon-reload`, for
example. Because of that protection, Ubuntu 24.04 LTS and later no longer comes with that unit
disabled, as we no longer consider it a potential issue for Ubuntu users on WSL.

In the unlikely event of your WSL instances still facing binary interoperability issues caused by
`systemd-binfmt.service` or another systemd unit that your system may have installed that affects
binfmt registration, an easy solution is to override the unit to disable it under WSL by running the
command `systemctl edit <UNIT_NAME>` and entering the following contents:

```ini
[Unit]
ConditionVirtualization=!wsl
```

And then run the command:

```{code-block} text
$ sudo systemctl daemon-reload
```

That creates a file at `/etc/systemd/system/<UNIT_NAME>/override.conf` with the contents above,
effectively disabling that unit under WSL.

If later you need to re-enable the unit, remove that file and run `sudo systemctl daemon-reload`.

## Further reading

- [Kernel Support for miscellaneous Binary Formats](https://docs.kernel.org/admin-guide/binfmt-misc.html)
- [binfmt.d](https://www.freedesktop.org/software/systemd/man/latest/binfmt.d.html#)
