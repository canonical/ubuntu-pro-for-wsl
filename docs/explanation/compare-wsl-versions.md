---
myst:
  html_meta:
    "description lang=en": "Read about the differences between major versions of WSL, such as WSL 1 and WSL 2, and how it affects Ubuntu on WSL."
---

# Comparing WSL versions for Ubuntu on WSL

This page explains the main differences between different major versions of WSL and how they affect Ubuntu on WSL.

## Background on WSL Versions

WSL has had two major versions: WSL 1 and WSL 2. The default version on Windows is WSL 2, although it is possible to run individual Ubuntu instances using either version.

There are significant architectural differences between the two versions, which can impact the behaviour of Ubuntu on WSL.

## Overview of major differences between WSL 1 and WSL 2

The primary difference between WSL 1 and WSL 2 is that WSL 1 functions as a compatibility layer, translating Linux system calls for the Windows kernel and providing only partial support for Linux system calls. In contrast, WSL 2 runs a full Linux kernel inside a lightweight virtual machine, offering complete system call compatibility. This fundamental architectural difference leads to significant variations in capabilities and performance between the two versions.

As WSL 1 only has partial support for Linux system calls, some programs may not behave as expected when using WSL 1. Additionally, WSL 1 lacks support for graphical applications and systemd, a system and service management suite.

In most cases, applications that are entirely contained in WSL will be faster on WSL 2, particularly for file IO intensive applications. However, WSL 1 has faster access to files mounted from Windows, so for certain use cases, WSL 1 may be faster.

## Summary of feature support across WSL versions

The differences between WSL 1 and WSL 2 and how it affects Ubuntu can be summarised with the table below:

| Feature                                                               |         WSL 1          |       WSL 2        |
| --------------------------------------------------------------------- | :--------------------: | :----------------: |
| Integration between Windows and Linux                                 |   {bdg-success}`Yes`   | {bdg-success}`Yes` |
| Fast boot times                                                       |   {bdg-success}`Yes`   | {bdg-success}`Yes` |
| Small resource footprint compared to traditional Virtual Machines     |   {bdg-success}`Yes`   | {bdg-success}`Yes` |
| Managed VM                                                            |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| Full Linux Kernel                                                     |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| Full system call compatibility                                        |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| High performance across OS file systems                               |   {bdg-success}`Yes`   |  {bdg-danger}`No`  |
| systemd support                                                       |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| IPv6 support                                                          |   {bdg-success}`Yes`   | {bdg-success}`Yes` |
| Graphical application support                                         |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| [Snap](https://snapcraft.io/) support                                 |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| [cloud-init](https://cloud-init.io/) support                          |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| [Ubuntu Pro for WSL](../tutorials/getting-started-with-up4w/) support |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| [Landscape](ref::landscape-client) support                            |    {bdg-danger}`No`    | {bdg-success}`Yes` |
| [Ubuntu Pro Client](ref::ubuntu-pro-client) support                   | {bdg-warning}`Partial` | {bdg-success}`Yes` |

### How this affects Ubuntu for WSL

The differences between WSL 1 and WSL 2 have notable consequences for Ubuntu on WSL, largely due to the lack of systemd support on WSL 1. Systemd is a system and service management suite that Ubuntu and many applications for Ubuntu depend on. Most notably, [Snaps](https://snapcraft.io/) and [cloud-init](https://cloud-init.io/) do not work on WSL 1 due to the lack of support for systemd. There is also a lack of support for graphical applications on WSL 1.

While Ubuntu works on WSL 1, the experience may be degraded relative to WSL 2. Thus, we generally recommend using WSL 2 unless you have specific requirements. For example, if you make extensive use of file interoperability between WSL and Windows, you may benefit from the better performance of WSL 1 in that specific area.

### How this affects Ubuntu Pro for WSL

[Ubuntu Pro for WSL](../tutorials/getting-started-with-up4w/) relies on systemd for much of its functionality, including automatic Pro attachment, so it does not support WSL 1. For the same reason, the [Landscape](ref::landscape-client) tool for remote management of Ubuntu instances is also not supported on WSL 1. You can still manually attach your WSL 1 instance to [Ubuntu Pro](https://documentation.ubuntu.com/pro/) using the [Ubuntu Pro Client](ref::ubuntu-pro-client), although some of Ubuntu Pro's features may not work on WSL 1.

### Switching WSL versions

Microsoft's official WSL documentation includes guidance on:

* [Setting the WSL version for a specific distro](https://learn.microsoft.com/en-us/windows/wsl/basic-commands#set-wsl-version-to-1-or-2)
* [Setting the default WSL version for new distro installations](https://learn.microsoft.com/en-us/windows/wsl/basic-commands#set-default-wsl-version)

## Further reading

- Visit the [Microsoft WSL documentation](https://learn.microsoft.com/en-us/windows/wsl/compare-versions) for additional information on the differences between WSL 1 and WSL 2.
