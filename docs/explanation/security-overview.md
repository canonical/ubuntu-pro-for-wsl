---
myst:
  html_meta:
    "description lang=en":
      "Read about security considerations when using Ubuntu on WSL."
---

# Security overview for Ubuntu on WSL

This page includes explanations of security considerations when using Ubuntu on WSL.
It also includes example commands and configurations to help improve security.

```{note}
This page assumes a WSL version of 2.4.10 or later.
```

## Download and installation

Always use a [supported LTS version](reference::distros) of Ubuntu on WSL to
ensure that you receive regular updates and bug fixes.

> For our latest installation instructions, read the [install Ubuntu on
WSL](howto::install-ubuntu-wsl) guide.

### Verifying the download (automatic)

When installing an Ubuntu image directly from the terminal using `wsl --install
<ubuntu distro>`, the SHA-256 checksum is automatically verified to ensure that it is
secure.

### Verifying the download (manual)

If you download an Ubuntu image from an online archive before installation, we
recommend that you manually verify the checksum.

Before Ubuntu is installed on WSL, you can verify the checksum of the download
in PowerShell, like this:

```powershell
Get-FileHash C:\Users\<username>\downloads\ubuntu-<version number>-wsl-amd64.wsl -A SHA256
```

You can then cross-reference the output against the checksum on the
[releases.ubuntu.com](https://releases.ubuntu.com) page before installing the
verified download:

```powershell
wsl --install --from-file ubuntu-<version number>-wsl.amd64.wsl
```

* [Read more about verifying an Ubuntu download](https://ubuntu.com/tutorials/how-to-verify-ubuntu#1-overview)
* [Read Microsoft's about testing custom Linux distros for WSL ](https://learn.microsoft.com/en-us/windows/wsl/build-custom-distro#test-the-distribution-locally)

## Login

### Windows host

Any WSL instance is only as secure as its Windows host.

The Windows user should be protected by a strong password, which will — by
extension — help secure instances of Ubuntu on WSL on the host machine.

Store your passwords securely and only share them with administrators when/if
necessary.

### WSL instance

Once logged into a Windows host machine, the user can create WSL
instances without elevated privileges.

When first opening an Ubuntu on WSL terminal with `wsl.exe -d <ubuntu distro>`, the user is
prompted for a username and password to create the default user account on Ubuntu.

Even if a password is set, it can be changed by the root user; however, the
permissions of the Windows user supersede that of the Ubuntu user.

### Root access

Access to a WSL instance as the root user is possible:

```text
wsl -d <ubuntu distro> -u root
```

After accessing an instance as a regular (non-root) user, a password is still expected for
commands requiring `sudo` within the instance:
the standard Linux user account controls apply.

Interacting with an instance using root access has no effect on the permissions
of the Windows' user, which continues to take precedence.
Running Windows binaries as root inside WSL won't make them elevated on the
Windows side; they run with the Windows user permissions only.

## Package management

### Updates and upgrades

As with any distribution, packages should be routinely updated and upgraded:

```text
sudo apt update && sudo apt upgrade -y
```

It is generally recommended that you install packages from official repositories using
`apt`.

Ubuntu on WSL also supports the installation of `snaps`, which are a more secure alternative to third-party `apt` repositories.

> [Read more about third-party packages in the Ubuntu Server documentation](https://documentation.ubuntu.com/server/explanation/software/third-party-repository-usage/)

If an instance is running, security updates are installed automatically.
This is because `unattended-upgrades` are enabled by default.

### AppArmor

AppArmor is a Linux Security Module implementation that controls the
capabilities and permissions of applications.

By default, AppArmor is installed in Ubuntu on WSL but not yet enabled, as it
requires certain features and patches not currently available in the WSL
kernel.

As such, snaps cannot be full confined on WSL.

> [To learn more about how AppArmor contributes to Snap security, read the Snapcraft documentation](https://snapcraft.io/docs/security-policies)

## Interoperability

It is possible to interact with the Windows' filesystem from a WSL instance,
and a WSL filesystem from Windows.

Note, however, that the permissions and restrictions on the Windows' user still
apply when operating from within a WSL instance.

The instance is therefore as secure as any arbitrary program running on the user
account of the Windows' host.

If you remain concerned about the security implications of interoperability, it [can
be disabled](https://learn.microsoft.com/en-us/windows/wsl/wsl-config#interop-settings) in `/etc/wsl.conf`:

```ini
[interop]
enabled=false
```

```{warning}
Interoperability is necessary for certain processes, including provisioning
with cloud-init.

[One approach](exp::disable-interoperability) is to first provision an instance and
then subsequently disable the feature.
```

## Ubuntu Pro

Ubuntu Pro offers [additional security](https://ubuntu.com/pro) to Ubuntu
distributions. For Ubuntu on WSL, the Pro client is pre-installed.

### Manual Pro-attachment

To manually attach a Pro subscription to a new instance, run this
command from inside the instance:

```text
sudo pro attach
```

Once your instance is Pro-attached, you can run various commands to monitor and secure your instance, including `pro security-status` and `pro fix`:

* [For more detail on the Pro client read its official documentation](https://canonical-ubuntu-pro-client.readthedocs-hosted.com/en/latest/)
* [For guidance on air-gapped environments, refer to the Ubuntu Pro documentation](https://canonical-ubuntu-pro-client.readthedocs-hosted.com/en/latest/explanations/using_pro_offline/)

### Livepatch

The WSL kernel is maintained by Microsoft.

There is no livepatch support for WSL kernels.
Livepatch is therefore disabled for Ubuntu on WSL instances.

In a Pro-attached WSL instance, running `pro status --all` will show that you
are **entitled** to the service but the status is still **n/a**. This means
that while your Pro subscription entitles you to using Livepatch on — for
example — an Ubuntu Server, it does not apply to Ubuntu on WSL.

> [GitHub repo for the WSL kernel](https://github.com/microsoft/WSL2-Linux-Kernel)

## The Ubuntu Pro for WSL application

```{include} ../pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

### Automatic Pro-attachment

For Pro-attaching multiple instances automatically, use the Ubuntu Pro for WSL
application.
This is most relevant for deployment scenarios in which multiple Windows hosts
are being managed centrally, using software like Landscape or Intune.

> [Get started with Ubuntu Pro for WSL](howto::up4w)

### Firewall configuration

Firewall rules must be configured for Ubuntu Pro for WSL to enable interactions
with different services, including Landscape and the Microsoft Store.

Any exchanges of data are encrypted using TLS.

> [Read our reference on firewall configuration for Ubuntu Pro on WSL](ref::firewall)

(exp::wsl1-incompatibility)=
### Incompatibility with WSL 1

WSL 2 is the default WSL version on Windows 11.
The legacy version — WSL 1 — can also still be used.

> [Read more about WSL versions](https://learn.microsoft.com/en-us/windows/wsl/compare-versions)

Ubuntu Pro for WSL only supports WSL 2.
When relying on Ubuntu Pro for WSL to manage the security of WSL instances,
you should therefore consider enforcing WSL 2 on host Windows machines.

To set the default version to WSL 2:

```text
wsl --set-default-version 2
```

To convert a specific distribution from WSL 1 to WSL 2:

```text
wsl --set-version <distro> 2
```

You can also get and set the default WSL version using the Windows registry,
which may be necessary for certain remote management setups.

To get the version:

```powershell
Get-ItemPropertyValue -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Lxss" -Name DefaultVersion
```

To set it:

```powershell
Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Lxss" -Name DefaultVersion -Value 2
```

Intune also supports policies for WSL, which include toggling the availability of WSL 1 on client machines:

> [Intune configuration options for WSL](https://learn.microsoft.com/en-us/windows/wsl/intune?source=recommendations)

## Security tips

### Configuring WSL features

WSL features can be controlled if they present a security concern.

For example, root login can be disabled, WSL 1 availability toggled and network
access configured.

There are various options to configure WSL instances, including:

* The `.wslconfig` file can be edited to configure [global settings](https://learn.microsoft.com/en-us/windows/wsl/wsl-config#wslconfig) for instances
* [WSL policies for Intune](https://learn.microsoft.com/en-us/windows/wsl/intune?source=recommendations) enable remote management of WSL features
* Registry entries for features like [WSL 1 availability](exp::wsl1-incompatibility) can be changed in the registry editor or with PowerShell scripts 

(exp::automate-hardening)=
### Automate hardening

Provisioning of WSL instances can be automated with cloud-init.

> [Read about automatic setup of Ubuntu on WSL with cloud-init](howto::cloud-init)

Cloud-init can be used to initialise your instances in a more secure way
(depending on your needs) before first login.

Below are some snippets that can help you automate hardening.

#### Update and upgrade packages

Add this line to the start of the config file to update and upgrade packages on boot:

```ini
package_reboot_if_required: true
package_update: true
package_upgrade: true
```

#### Disable root login

Add the following to automatically run a `sed` command to modify the `/etc/passwd` file:

```ini
runcmd:
- sed -i 's/^root.*$/root:x:0:0:root:\/root:\/usr\/sbin\/nologin/' /etc/passwd
```

This replaces substitutes `root:x:0:0:root:/root:/usr/sbin/nologin` for the
line beginning with `root`.

#### Make SSH more secure

For the user `u`, grant user permissions, define the default shell and adds
an authorised public SSH key:

```ini
users:
- name: u
  groups: users,sudo
  sudo: ALL=(ALL) NOPASSWD:ALL
  shell: /bin/bash
  ssh-authorized-keys:
    - ssh-rsa ...
  lock_passwd: true
```

Make `u` the default user and grant them SSH access, then define paths for SSH
configuration and host key. In addition, prevent logins as root and with empty
passwords, and limit the number of unsuccessful attempts to 3.

```ini
write_files:
- path: /etc/wsl.conf
  append: true
  content: |
    [user]
    default=u
  - path: /etc/ssh/sshd_config
    content: |
      HostKey /etc/ssh/ssh_host_rsa_key
      MaxAuthTries 3
      PermitRootLogin no
      PermitEmptyPasswords no
      AllowUsers u
```
(exp::disable-interoperability)=
#### Disable interoperability

Run a command that adds a line to disable interoperability to `/etc/wsl.conf`:

```ini
runcmd:
  - echo "[interop]" | sudo tee -a /etc/wsl.conf
  - echo "enabled = false" | sudo tee -a /etc/wsl.conf
```

```{note}
It is expected that most users will SSH from WSL rather than SSH into WSL.
```

### Remote management tools

Ubuntu Pro for WSL increases the capacity of system administrators to manage
and secure Windows hosts containing instances of Ubuntu on WSL.

Learn more about remote management of Ubuntu on WSL in this documentation:

* [Tutorial on deploying instances with Landscape](tut::deploy)
* [Guides on remote management with Landscape and Intune](howto::index-remote-deployment)

### Reporting a vulnerability

Details on the security updates that we provide and the responsible disclosure
of security vulnerabilities for the Ubuntu distribution on WSL can be found
below:

* [Security policy for the Ubuntu Pro for WSL](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/SECURITY.md)
* [Security policy for the Ubuntu distro on WSL](https://github.com/ubuntu/WSL/blob/main/SECURITY.md)

## Resources

* [Ubuntu Pro client documentation](https://canonical-ubuntu-pro-client.readthedocs-hosted.com/en/latest/)
* [Microsoft guide on configuring WSL](https://learn.microsoft.com/en-us/windows/wsl/wsl-config)
* [Microsoft Defender for Endpoint plugin for WSL](https://learn.microsoft.com/en-us/defender-endpoint/mde-plugin-wsl)

