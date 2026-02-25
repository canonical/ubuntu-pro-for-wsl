---
myst:
  html_meta:
    "description lang=en":
      "Read about the opt-in, anonymous collection of system data present on Ubuntu on WSL."
---

# Data collection and Ubuntu on WSL

```{note}
This page refers to behaviour introduced in Ubuntu 26.04 LTS (Resolute Racoon).
```

Ubuntu includes opt-in, anonymous collection of system data to assist the Ubuntu development team in maintaining and improving Ubuntu.

The data collected cannot be used to track individual machines or users.

This page includes a general explanation of data collection on Ubuntu, as well as specifics unique to Ubuntu on WSL.

## When is data collected?

Ubuntu on WSL collects and uploads data following initial setup, as long as the one-time script that initialises the Ubuntu instance's environment is not bypassed.

When using [WSL 2 with systemd enabled](explanation::wsl-version), data is also collected approximately once a month, with a delay of one week between collection and upload.

Data is also collected following a [release upgrade of an existing instance](ref::upgrade-instructions), though this data is only uploaded on WSL 2 instances with systemd enabled.

## What kind of information is collected?

If you opt in, anonymous data related to your system is collected. This includes information, such as your hardware configuration, timezone, and language. We also may collect WSL-specific information, such as your WSL version, kernel version, and if you have interoperability enabled.

Personal information, including usernames and IP addresses, is not collected.

Should you opt out, no information is collected and sent beyond a simple opt-out notification.

## Can I see a preview of the data being collected?

During the consent prompt of the initial Ubuntu setup, you can view an example report to inform your choice about whether to opt in or opt out.

Running the following command from within your WSL instance will generate a new report without writing it to disk or uploading it:

```{code-block} text
$ ubuntu-insights collect --dry-run
```

You may also navigate to `~/.cache/ubuntu-insights` within your WSL instance to see what has already been collected and what has been uploaded.

## How do I manage my consent state?

### First time setup

During the first-ever installation of an Ubuntu for WSL instance, you will see an interactive prompt during the setup process asking if you would like to opt in or out.

This prompt will not be shown if data collection has been pre-configured through the Windows registry, cloud-init, or some other method.

Your answer is saved to Windows. This is then used as your default answer for all future instances, to avoid repeated prompting for consent.

### Managing the Windows user default

To avoid prompting every time a new Ubuntu instance is created on WSL, Ubuntu will save your answer to the DWORD value `UbuntuInsightsConsent` of the following [Windows registry key][link-ms-registry-documentation]:

`HKCU:\Software\Canonical\Ubuntu`

If this value is set, Ubuntu will refer to it instead of prompting for your consent again during the setup process.

To get the value:

```{code-block} powershell
> Get-ItemPropertyValue -Path "HKCU:\Software\Canonical\Ubuntu" -Name UbuntuInsightsConsent
```

Advanced users may manually change the DWORD value of this registry key to either `0` or `1` to opt in or out respectively. For example, to opt in by default:

```{code-block} powershell
> Set-ItemProperty -Path "HKCU:\Software\Canonical\Ubuntu" -Name UbuntuInsightsConsent -Value 1
```

```{note}
Ubuntu will only try to read from the Windows registry key during initial setup. Only instances created **after** the registry key has been modified will be affected.
```

### Setting consent with cloud-init

[Cloud-init](howto::cloud-init) may be used to automatically setup Ubuntu on WSL with pre-configured consent configuration files on a per-instance basis.

If Ubuntu is provisioned such that at least one non-root user with a home directory has a valid consent configuration file, the setup process will ignore any default state set by the Windows registry key, and it will also skip the interactive prompt asking for consent.

In this case, any additional users in a multi-user Ubuntu instance without a consent configuration will not collect or upload any data.

### Managing an individual Ubuntu instance's consent state

To see your current consent state for an individual Ubuntu instance, run the following command:

```{code-block} text
$ ubuntu-insights consent linux wsl_setup ubuntu_release_upgrader
```

To change your consent state for an individual Ubuntu instance after initial setup, run the following command, replacing the placeholder with your desired state:

```{code-block} text
$ ubuntu-insights consent linux wsl_setup ubuntu_release_upgrader --state=<true, false>
```

```{note}
Consent configuration is unique to each Ubuntu user. The setup process will apply your consent choice to all non-root users with a home directory at the time of setup. In most cases, this will just be the user you created as part of that setup process.
```

## Further reading

* [System information legal notice](https://canonical.com/legal/systems-information-notice)
* [Data privacy policy](https://canonical.com/legal/data-privacy)
* [Ubuntu Insights](https://github.com/ubuntu/ubuntu-insights)
* [Microsoft Windows registry documentation][link-ms-registry-documentation]

<!-- Link definitions -->
[link-ms-registry-documentation]: https://learn.microsoft.com/en-us/troubleshoot/windows-server/performance/windows-registry-advanced-users
