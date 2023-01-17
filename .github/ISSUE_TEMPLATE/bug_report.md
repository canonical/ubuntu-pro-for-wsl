---
name: Report an issue
about: Create a bug report to fix an existing issue.
title: ''
labels: ''
assignees: ''

---
>**Please do not report security vulnerabilities here**
>
> 1. If your bug is on the Windows side (Graphical interface or agent running as a background service on Windows), please use the advisories page of the repository and not a public bug report.
> 1. If your bug affects the "ubuntu-wsl" service part running on ubuntu, please use [launchpad private bugs](https://bugs.launchpad.net/ubuntu/+source/ubuntu-wsl-service/+filebug) which is monitored by our security team. On ubuntu machine, it’s best to use `ubuntu-bug ubuntu-wsl-service` to collect relevant information.

**Thank you in advance for helping us to improve Ubuntu Pro for Windows!**
Please read through the template below and answer all relevant questions. Your additional work here is greatly appreciated and will help us respond as quickly as possible. For general support or usage questions, use [Ubuntu Discourse](https://discourse.ubuntu.com/c/desktop/8). Finally, to avoid duplicates, please search existing Issues before submitting one here.

By submitting an Issue to this repository, you agree to the terms within the [Ubuntu Code of Conduct](https://ubuntu.com/community/code-of-conduct).

## Description

> Provide a clear and concise description of the issue, including what you expected to happen.

## Reproduction

> Detail the steps taken to reproduce this error, what was expected, and whether this issue can be reproduced consistently or if it is intermittent.
>
> Where applicable, please include:
>
> * Code or command sample to reproduce the issue
> * Log files (redact/remove sensitive information)
> * Application settings (redact/remove sensitive information)
> * Screenshots

### Environment

> Please provide the following:

#### For Ubuntu users, if the bug is on the ubuntu-wsl service running inside the WSL machine, please run and copy the following

1. `ubuntu-bug ubuntu-wsl-service --save=/tmp/report`
1. Copy paste below `/tmp/report` content:

```raw
COPY REPORT CONTENT HERE.
```

#### Installed versions

TODO: we should probably have a binary on the windows side collecting all this for you.

* Windows version: TODO, describe how to get it
* WSL version: TODO, describe how to get it
* WSL distribution: TODO, describe how to get it
* Windows application version:  TODO, describe how to get it
* WSL OS: (`/etc/os-release`) in the wsl instance
* ubuntu wsl service version: (`apt policy ubuntu-wsl-service` output)

#### Additional context

> Add any other context about the problem here.
