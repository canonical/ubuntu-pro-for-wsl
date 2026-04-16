---
myst:
  html_meta:
    "description lang=en":
      "Explains time synchronization in WSL, and what to do if the default option doesn't suit your needs."
---

# Time synchronization for Ubuntu on WSL

Since at least the early 2000s, major operating systems adopted the Network Time Protocol (NTP) to
synchronize their system time with external time sources, network time servers, and to ensure
accurate timekeeping.

Virtualization environments, such as WSL, can have different time synchronization requirements. For
example, some users may want to have the WSL instance's system time independent from the Windows
host, while others may want to have them synchronized.

Historically, WSL had some issues with time synchronization. One problem was that the WSL instance's system
time would get out-of-sync with the Windows host after resuming from suspension or hibernation. For this
reason, previous versions of Ubuntu enabled the `systemd-timesyncd.service` unit by default, which
is a simple NTP client that can synchronize the system time with external time servers.
Those issues have been fixed, with the adoption of a kernel patch which ensures that Hyper-V
time sample messages trigger immediate time synchronization implicitly, i.e. the guest (the WSL
virtual machine may treat sample messages as forceful requests). That means that, by default, WSL
instances have their system time always synchronized with the Windows host. That implies that a NTP
client should no longer be needed and, in fact, it could conflict with the host, causing clock skews
if they synchronize with different time servers than the ones used by Windows.

It is expected that the majority of users benefit from this implicit time sync between WSL instances
and the Windows host. Users with special requirements about time synchronization can choose to
disable the default behavior and either use an internal NTP client or a non-synchronized system time.

## Disabling Hyper-V implicit time synchronization globally

Currently, Hyper-V implicit time synchronization can't be fully disabled. There is [an open issue](https://github.com/microsoft/WSL/issues/12765)
in the WSL repository to track that request. The problem is that the kernel command line option
`hv_utils.timesync_implicit=1` is always passed to the WSL kernel. It should be possible to disable
it by passing the kernel command line option `hv_utils.timesync_implicit=0` to the WSL kernel. This can be done by
adding the following lines to the WSL configuration file located at `%USERPROFILE%\.wslconfig`:

```{code-block} ini
[wsl2]
kernelCommandLine = hv_utils.timesync_implicit=0
```

Some users reported that this doesn't work on WSL version 2.5.4 and later, but tests
on recent versions of Windows 11 failed even as far back as WSL version 2.4.12.
The option is appended to the kernel CLI passed by the WSL platform, which already includes
`hv_utils.timesync_implicit=1`. The Linux kernel parses its command line options left to right, so
the last option specified for a particular setting should take precedence. But for this particular
option that's not the observed behaviour, resulting in the implicit time synchronization being
always enabled.

## Interacting with NTP clients in Ubuntu on WSL

Starting with Ubuntu 25.10, we migrated from `systemd-timesyncd` to `chrony` for better security
features, such as support for Network Time Security (NTS), which provides cryptographic
authentication for time data—and offers better resistance against malicious time manipulation. NTS
is for NTP what HTTPS is for HTTP, roughly speaking. By default, the `chrony.service` is enabled and
can report clock drifts, but it cannot modify the system clock when running inside WSL without
further configuration. So, on Ubuntu 26.04 LTS and later you don't have to do anything special to
opt into the implicit time synchronization provided by Hyper-V. If you want to disable `chrony`
completely, run the following command:

```{code-block} text
$ systemctl disable chrony.service
```

On the other hand, if you want `chrony` to fix your clock inside WSL instances, then you need to
change its configuration file to enable synchronization in containers. To do that, edit the
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

The pool of servers `chrony` synchronizes with on Ubuntu is defined in
`/etc/chrony/sources.d/ubuntu-ntp-pools.sources`. You can change that file to synchronize with your
preferred time servers or apply custom configuration. For example, to synchronize with the same NTP
servers as Windows, you can create a file replacing the one above with the following contents:

```ini
server time.windows.com iburst
server time.nist.gov iburst
```

There are lots of other options supported by `chrony`, it's outside of the scope of this document
to cover them all. You can learn more about `chrony` and its configuration, in [chrony's official documentation](https://chrony.tuxfamily.org/documentation.html)

Ubuntu 24.04 LTS and previous releases have the `systemd-timesyncd.service` unit enabled by default.
If the default time synchronization suits your needs, you should disable that unit to avoid
conflicts with the Hyper-V implicit time synchronization. 

```{code-block} text
$ systemctl disable systemd-timesyncd.service
```

To change which NTP servers `systemd-timesyncd` synchronizes with, edit the file
`/etc/systemd/timesyncd.conf` and set the `NTP` variable to the configuration you want. For example,
to synchronize with the Windows time servers, set it to the following:

```ini
[Time]
NTP=time.windows.com time.nist.gov
```

You can learn more about `systemd-timesyncd` and its configuration in [systemd-timesyncd's official documentation](https://www.freedesktop.org/software/systemd/man/latest/systemd-timesyncd.service.html).

## References

- [Issue on disabling Hyper-V implicit time sync](https://github.com/microsoft/WSL/issues/12765)
- [Ubuntu Server documentation about chrony](https://ubuntu.com/server/docs/how-to/networking/chrony-client/)
- [chrony official documentation](https://chrony.tuxfamily.org/documentation.html)
- [systemd-timesyncd official documentation](https://www.freedesktop.org/software/systemd/man/latest/systemd-timesyncd.service.html)
- [WSL2 clock skew issues mega thread](https://github.com/microsoft/WSL/issues/10006)
