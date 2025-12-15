---
myst:
  html_meta:
    "description lang=en":
      "Explains why systemd-binfmt.service affects the WSL experience, our approach to the service on the Ubuntu distro and how users can configure the behaviour."
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

For reasons better explained in the rest of this page, we consider `systemd-binfmt.service` as a potential
issue for most WSL users, thus that service **is intentionally disabled for Ubuntu on WSL**. Most users should
not notice or care about that service, as, by default, it does not affect the user's ability to run Windows
binaries. But those relying on emulators or interpreters may find this behaviour particularly annoying. If
you are one of those users this page is for you.

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

## Current limitations of binfmt registration protection implemented by WSL

The scenarios above were reported by users in previous versions of WSL. The WSL developers have since
implemented numerous improvements. As of version 2.5.1, WSL is capable of restoring its binfmt registration at
startup and when that service is restarted. Yet, the current solution does not guarantee that WSL users won't
be affected by occasional breakage in Windows interoperability. For example, while `systemctl restart
systemd-binfmt.service` by itself won't cause any problems, if that command runs after `systemctl
daemon-reload` then the Windows executable format registration will be gone. This seems an edge case, but
there is a non-trivial number of combinations of packages that, when installed or uninstalled together, lead
to such behaviour. Consider for instance, installing both `qemu-user-static` and `binfmt-support` (used in
combination for example to allow ARM devices to execute x86_64 binaries). The latter needs to run `systemctl
daemon-reload` in its post installation script and the former restarts the binfmt service, which is exactly
the order that breaks Windows binary interoperability!

```{warning} If you really want to understand why that happens

**Doing the tests below can break binary interoperability until the WSL instance is restarted.**

WSL writes the override file `/run/systemd/generator.early/wsl-binfmt.service` that runs some commands to
restore the WSL interoperability registration when the systemd-binfmt.service (re)starts.

Try running the following command: `sudo systemctl daemon-reload`.
Then try finding that file again. It's gone.

When reloading daemons, systemd cleans the generator output directories (`/run/systemd/generator`,
`/run/systemd/generator.early` and `/run/systemd/generator.late`) before running them again. Refer to [systemd
documentation](https://www.freedesktop.org/software/systemd/man/latest/systemd.generator.html) to learn more
about that topic.

If `systemd-binfmt.service` was allowed to run at this point, Windows binary interoperability would break.
```


## The Ubuntu approach

While there are other ways to solve this problem, we assumed that most users wouldn't notice if the service was
disabled. Ubuntu WSL images are therefore published with a file that makes systemd disable that service on WSL,
which is:

`/usr/lib/systemd/system/systemd-binfmt.service.d/wsl.conf`

```ini
[Unit]
ConditionVirtualization=!wsl
```

Users that need emulators and binfmt support managed by systemd more than the Windows binary
interoperability or that can tolerate the need to restart WSL when the interoperability breaks are encouraged
to remove that file.

```{code-block} text
$ sudo rm /usr/lib/systemd/system/systemd-binfmt.service.d/wsl.conf
$ sudo systemctl daemon-reload
```

Then, manually restart that service:

```{code-block} text
$ sudo systemctl restart systemd-binfmt.service
```

After running these commands, foreign executable file format registration mediated by systemd will work.

## Closing out

More improvements are expected in the WSL 2.5.x release series, so we're positive that we will soon be able to remove the
override that disables the `systemd-binfmt.service` on Ubuntu on WSL. That transition should be
transparent to most users. In the meantime, users that need `systemd-binfmt.service` can follow the configuration steps outlined in
this page.

## Further reading

- [Kernel Support for miscellaneous Binary Formats](https://docs.kernel.org/admin-guide/binfmt-misc.html)
- [binfmt.d](https://www.freedesktop.org/software/systemd/man/latest/binfmt.d.html#)
