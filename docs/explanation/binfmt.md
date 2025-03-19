---
myst:
  html_meta:
    "description lang=en":
      "Explains how systemd-binfmt.service affects the Ubuntu WSL experience and what can be done about that."
---

# Ubuntu on WSL and `binfmt_misc`

One key feature of the WSL experience is the binary interoperability, that is, the ability to seamlessly run
Windows binaries from inside WSL and vice-versa. Linux can run Windows binaries thanks to `binfmt_misc`, a
capability offered by the Linux kernel via which arbitrary executable file formats can be recognised and
passed to certain user space applications, such as interpreters, emulators and virtual machines, that know how
to execute such format. The executable formats are registered via a file interface, usually located at
`/proc/sys/fs/binfmt_misc/register` or using wrappers such as those offered by `binfmt-support` or
`systemd-binfmt.service`. WSL registers Windows binaries to be passed to the `/init` program, which knows how
to pass that along to Windows (check `/proc/sys/fs/binfmt_misc/WSLInterop*` in a WSL distro instance).

For reasons better explained in the rest of this page, we consider `systemd-binfmt.service` as a potential
issue for most WSL users, thus that service **is intentionally disabled for Ubuntu on WSL**. Most users should
not notice or care about that service, but those relying on emulators or interpreters may find this behaviour
particularly annoying. If you are one of those users this page is for you.

## systemd-binfmt.service

With systemd enabled, `systemd-binfmt.service` runs during boot, reads configuration files from specific
directories and registers additional executable formats with the kernel. All registrations are removed when it
stops. If that service is not aware of the WSL registration mechanism it can break Windows binary
interoperability in different ways, such as:

- multiple running distro instances affected when one stops. Since the WSL distro instances are effectively
containers sharing the same kernel and the `binfmt_misc` file system interface is a shared mount point, when a
distro instance stops and `systemd-binfmt.service` quits it can broken the registration for other instances.

- Startup ordering. If WSL didn't order its own registration after that service, the service would break WSL
interoperability.

- Service restart. Restarting `systemd-binfmt.service` implies unregistering and re-registering executable
file formats. That can easily happen without the user awareness, such as when installing emulators or other
packages that rely on `binfmt_misc`.

The scenarios above were reported in previous versions of WSL. Upstream implemented numerous improvements
about that topic. As of version 2.5.1, WSL is capable of restoring its binfmt registration at startup and when
that service is restarted. Yet, the solution is not complete enough to ensure most WSL users won't be affected
by Windows interoperability breaks from time to time. For example, while `systemctl restart
systemd-binfmt.service` by itself won't cause any issues, if that command runs after `systemctl daemon-reload`
then the Windows executable format registration will be gone. It seems an edge case, but there is a
non-trivial amount of combinations of packages that, when installed or uninstalled together, lead to such
behaviour. Consider for instance, installing both `qemu-user-static` and `binfmt-support` (used in combination
for example to allow ARM devices to execute x86_64 binaries). The latest needs to run `systemctl daemon-reload`
in its post installation script and the first restarts the binfmt service. That happens exactly in the order
that breaks Windows binary interoperability!

```{warning} If you really want to understand why that happens

**Doing the tests below can make binary interoperability broken until the WSL instance is restarted!**

WSL writes the override file `/run/systemd/generator.early/wsl-binfmt.service` that runs some commands to
restore the WSL interoperability registration when the systemd-binfmt.service (re)starts.

Try running the following command: `sudo systemctl daemon-reload`
Then try finding that file again. It's gone.

When reloading daemons, systemd cleans the generator output directories (`/run/systemd/generator`,
`/run/systemd/generator.early` and `/run/systemd/generator.late`) before running them again. Refer to [systemd
documentation](https://www.freedesktop.org/software/systemd/man/latest/systemd.generator.html) to learn more
about that topic.

If `systemd-binfmt.service` was allowed to run at this point, Windows binary interoperability would break.
```


## The Ubuntu approach

While there are other ways to solve this problem, we assumed most users wouldn't notice that service being
disabled, thus Ubuntu WSL images are published with a file that makes systemd disable that service on WSL,
which is:

`/usr/lib/systemd/system/systemd-binfmt.service.d/wsl.conf`

```ini
[Unit]
ConditionVirtualization=!wsl
```

Advanced users that need emulators and binfmt support managed by systemd more than the Windows binary
interoperability or that can tolerate the need to restart WSL when the interoperability breaks are encouraged
to remove that file.

```bash
sudo rm /usr/lib/systemd/system/systemd-binfmt.service.d/wsl.conf
sudo systemctl daemon-reload
```

Then, manually restart that service.

```bash
sudo systemctl restart systemd-binfmt.service
```

From now on, foreign executable file format registration mediated by systemd will just work as you'd expect.

## Closing out

More improvements are expected in the WSL 2.5.x release series, so we're positive about removing the
override that disables the `systemd-binfmt.service` on Ubuntu on WSL soon. That transition should be
transparent to most users. In the meanwhile, power users that need `systemd-binfmt.service` should refer to
this page.

## Further reading

- [Kernel Support for miscellaneous Binary Formats](https://docs.kernel.org/admin-guide/binfmt-misc.html)
- [binfmt.d](https://www.freedesktop.org/software/systemd/man/latest/binfmt.d.html#)
