---
myst:
  html_meta:
    "description lang=en":
      "Explains time synchronization in WSL, and what to do if the default option doesn't suit your needs."
---

# Time synchronization for Ubuntu on WSL

Since at least early 2000s, major operating systems adopted the Network Time Protocol to synchronize
their system time with external time sources, network time servers, to ensure accurate timekeeping.

Virtualization environments, such as WSL, can have different time synchronization requirements. For
example, some users may want to have the WSL instance's system time independent from the Windows
host, while others may want to have it synchronized.

Historically, WSL had some issues with time synchronization, such as the WSL instance's system
time out of sync with the Windows host after resuming from suspension or hibernation. For that
reason, previous versions of Ubuntu enabled the `systemd-timesyncd.service` unit by default, which
is a simple NTP client that can synchronize the system time with external time servers.
Those issues were fixed a while ago, with the adoption of a kernel patch that treats messages sent
by Hyper-V containing time samples as a trigger to resynchronize the virtual machine time. That means
that, by default, WSL instances will have their system time synchronized with the Windows host. That
implies that a NTP client should no longer be needed and in fact it could conflict with the host,
causing clock skews if they synchronize with different time servers than the ones used by Windows.

It is expected that the majority of users benefit from this implicit time sync between WSL instances
and the Windows host. Users with special requirements about time synchronization can choose to
disable the default behavior and either use an internal NTP client or leave the system time
non synchronized.

## Disabling Hyper-V implicit time synchronization globally

We currently can't fully disable that feature. There is [an open issue](https://github.com/microsoft/WSL/issues/12765)
in the WSL repository to track that request. The problem is that the kernel command line option
`hv_utils.timesync_implicit=1` is always passed to the WSL kernel. It should be possible to disable
it by passing the kernel command line option `hv_utils.timesync_implicit=0` to the WSL kernel (by
adding the following lines to the WSL configuration file located at `%USERPROFILE%\.wslconfig`):

```{code-block} ini
[wsl2]
kernelCommandLine = hv_utils.timesync_implicit=0
```

Some users reported that command line not taking effect on WSL version 2.5.4 and later, but tests
with that command line on recent versions of Windows 11 failed even as far as WSL version 2.4.12.
The option is added to the kernel CLI, but it cannot override the first one, which results in the
implicit time synchronization being always enabled.

## Configuring NTP clients in Ubuntu on WSL

Starting with Ubuntu 25.10, we migrated from `systemd-timesyncd` to `chrony` for better security
features. By default the `chrony.service` is enabled, it can report clock drifts, but it cannot
touch the system clock when running inside WSL without further configuration. So, on Ubuntu 26.04
LTS and later you don't have to do anything special to opt into the implicit time synchronization
provided by Hyper-V. If you want to disable `chrony` completely, run the following command:

```{code-block} text
$ systemctl disable chrony.service
```

On the other hand, if you want `chrony` to fix your clock inside WSL instances, then you need to
change it's configuration file to enable synchronization in containers. To do that, edit the
following line to the `/etc/default/chrony` file:

```ini
SYNC_IN_CONTAINER="yes"
```

Then restart the systemd unit to apply the changes:

```{code-block} text
$ systemctl restart chrony.service
```

With that you tell `chrony` to not worry about being inside a container, and feel free to adjust
the system clock if it sees it is needed. Note that, if you have the Hyper-V implicit time
synchronization enabled, you may end up with clock skews if `chrony` synchronizes with different
time servers than the ones used by Windows.

The pool of servers `chrony` synchronizes with on Ubuntu is defined at
`/etc/chrony/sources.d/ubuntu-ntp-pools.sources`. To change that option, replace that file with your
own custom configuration. For example, to synchronize with the same NTP servers as Windows, you can
create a file replacing the one above with the following contents:

```ini
server time.windows.com iburst
server time.nist.gov iburst
```

To learn more about `chrony` and its configuration, check the official documentation listed in the
references below.

Ubuntu 24.04 LTS and previous releases have the `systemd-timesyncd.service` unit enabled by default.
If the default time synchronization suits your needs, you should disable that unit to avoid
conflicts with the Hyper-V implicit time synchronization. 

```{code-block} text
$ systemctl disable systemd-timesyncd.service
```

To change which NTP servers `systemd-timesyncd` synchronizes with, edit the file
`/etc/systemd/timesyncd.conf` and set the `NTP` variable to the configuration you want. For example,
to synchronize with the Windows time servers, you can set it to the following:

```ini
[Time]
NTP=time.windows.com time.nist.gov
```

To learn more about `systemd-timesyncd` and its configuration, check the official documentation
listed in the references below.

## References

- [Issue on disabling Hyper-V implicit time sync](https://github.com/microsoft/WSL/issues/12765)
- [Ubuntu Server documentation about chrony](https://ubuntu.com/server/docs/how-to/networking/chrony-client/)
- [chrony official documentation](https://chrony.tuxfamily.org/documentation.html)
- [systemd-timesyncd official documentation](https://www.freedesktop.org/software/systemd/man/latest/systemd-timesyncd.service.html)
- [WSL2 clock skew issues mega thread](https://github.com/microsoft/WSL/issues/10006)
