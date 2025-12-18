# Security policy

## Supported versions

Ubuntu Pro for WSL is made of two main components: the application, which runs on the Windows host, and the `wsl-pro-service`,
which runs within instances of Ubuntu on WSL. When referring to supported versions, both components must be considered
separately. We currently provide security updates for:

* the Windows application on all versions of Windows 11
* the `wsl-pro-service` on Ubuntu 24.04 LTS.

We are planning to backport the service to Ubuntu 22.04 LTS.

Confirm that you are using a supported version to receive updates and
patches.

If you are unsure of the version, please run the following command in a
WSL terminal running your Ubuntu distro:

```
lsb_release -a
```

## What qualifies as a security issue

Pro for WSL operates within the security context of the authenticated Windows user. By design, the
application runs with standard user privileges and lack administrative elevation. While the local
user maintains full access to their own instance data, cross-user data access is strictly
prohibited.

A vulnerability is classified as a security issue if a flaw in Pro for WSL, or its underlying
dependencies, enables any of the following:
* Privilege Escalation: Gaining higher-level system permissions than those assigned.
* Unauthorized Access: Modification or exfiltration of Pro for WSL data by non-privileged or secondary users.
* Denial of Service (DoS): Compromising system availability or integrity.

Any behaviour meeting these criteria must be documented and reported through the established
security channels for immediate investigation.

## Reporting a vulnerability

If you discover a security vulnerability within this repository, we encourage
responsible disclosure. Please report any security issues to help us keep
`Ubuntu Pro for WSL` secure for everyone.

### Private vulnerability reporting

The most straightforward way to report a security vulnerability is through
[GitHub](https://github.com/canonical/ubuntu-pro-for-wsl/security/advisories/new). For detailed
instructions, please review the
[Privately reporting a security vulnerability](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
documentation. This method enables you to communicate vulnerabilities directly
and confidentially with the `Ubuntu Pro for WSL` maintainers.

The project's admins will be notified of the issue and will work with you to
determine whether the issue qualifies as a security issue and, if so, in which
component. We will then handle finding a fix, getting a CVE assigned and
coordinating the release of the fix to the various Linux distributions.

The [Ubuntu Security disclosure and embargo policy](https://ubuntu.com/security/disclosure-policy)
contains more information about what you can expect when you contact us, and what we expect from you.

#### Steps to report a vulnerability on GitHub

1. Go to the [Security Advisories Page](https://github.com/canonical/ubuntu-pro-for-wsl/security/advisories) of the `Ubuntu Pro for WSL` repository.
2. Click "Report a Vulnerability"
3. Provide detailed information about the vulnerability, including steps to reproduce, affected versions, and potential impact.

## Security resources

- [Canonical's Security Site](https://ubuntu.com/security)
- [Ubuntu Security disclosure and embargo policy](https://ubuntu.com/security/disclosure-policy)
- [Ubuntu Security Notices](https://ubuntu.com/security/notices)
- [Ubuntu on WSL documentation](https://documentation.ubuntu.com/wsl/en/latest/)

If you have any questions regarding security vulnerabilities, please reach out
to the maintainers through the aforementioned channels.

