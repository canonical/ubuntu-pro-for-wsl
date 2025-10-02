---
myst:
  html_meta:
    "description lang=en":
      "Use Landscape to enforce startup of the Ubuntu Pro for WSL background agent to support zero-touch deployment at scale."
---

# Remotely enforce startup of the Ubuntu Pro for WSL background agent using the Windows Registry

```{include} ../pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

Ubuntu Pro for WSL, being a Microsoft Store application, cannot ship user services as of the time of writing (late
2024), but can deploy startup tasks instead, programs that run with user permissions when the user logs into the
Windows device. The UP4W background agent runs as a startup task, which is only enabled by the
operating system when the user interacts with the application for the first time. While this behaviour is a feature for
end-users it presents a source of friction for deployments at scale, when system administrators expect zero-touch
deployment of UP4W to just work.

This guide shows how sysadmins can use the Windows Registry to enforce the enablement of the UP4W background agent
startup task without depending on end-user interaction. While this guide uses
[Intune](https://learn.microsoft.com/en-us/mem/intune/fundamentals/what-is-intune), it should be reproducible with any
remote management solution capable of deploying MS Store (or MSIX-packaged) applications and registry keys.

## Pre-requisites

- At least one managed Windows device.
- A Windows remote management solution.
- If using Intune, an Enterprise E3 or E5 or Education A3 or A5 licenses.

## Overview

1. The registry path `"HKCU:\Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\SystemAppData\CanonicalGroupLimited.UbuntuPro_79rhkp1fndgsc"`
   holds configuration information specific about UP4W and is created or overwritten when the MSIX package is
   installed.
2. A sub-key named `UbuntuProAutoStart` governs the startup task state.
3. Setting the DWORD value named `State` to `4` makes the operating system interpret it as
   ["Enabled by Policy"](https://learn.microsoft.com/en-us/uwp/api/windows.applicationmodel.startuptaskstate).
4. When the user logs in, Windows executes the UP4W startup task, even if the user has not interacted with the application.
5. A Windows remote management solution can monitor that registry key value and proactively fix it, thus enforcing the
   UP4W startup task to be always enabled.

(howto::enforce-with-intune)=
## Using Intune Remediations

Remediations are script packages that can detect and fix common issues on a user's device before they realise
there's a problem. Each script package consists of a detection script, a remediation script, and metadata. Through
Intune, you can deploy these script packages and monitor reports of their effectiveness.
[Visit the Microsoft Intune documentation](https://learn.microsoft.com/en-us/mem/intune/fundamentals/remediations)
to learn more. Those scripts run on a predefined schedule and if the detection script reports a failure (by
`exit 1`) then the remediation script will run. That allows system administrators to watch for the specific Registry
key value that represents the enablement of the UP4W background agent startup task. The contents of both scripts are
presented below. **Make sure to save them encoded in UTF-8**, as required by Intune.

### Detection script

```powershell
$Path = "HKCU:\Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\SystemAppData\CanonicalGroupLimited.UbuntuPro_79rhkp1fndgsc\UbuntuProAutoStart"
$Name = "State"
$Value = 4

Try {
    $Registry = Get-ItemProperty -Path $Path -Name $Name -ErrorAction Stop | Select-Object -ExpandProperty $Name
    If ($Registry -eq $Value){
        Write-Output "Compliant"
        Exit 0
    }
    Write-Warning "Not Compliant"
    Exit 1
}
Catch {
    Write-Warning "Not Compliant"
    Exit 1
}
```

### Remediation script

```powershell
$Path = "HKCU:\Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\SystemAppData\CanonicalGroupLimited.UbuntuPro_79rhkp1fndgsc\UbuntuProAutoStart"
$Name = "State"
$KeyFormat = "DWORD"
$value = 4


try{
    if(!(Test-Path $Path)){New-Item -Path $Path -Force}
    if(!$Name){Set-Item -Path $Path -Value $Value}
    else{Set-ItemProperty -Path $Path -Name $Name -Value $Value -Type $KeyFormat}
    Write-Output "Key set: $Name = $Value"
}catch{
    Write-Error $_
}

```

### Running the scripts with Intune

Access your organisation's [Intune Admin Center](https://intune.microsoft.com) and when logged in go to **Devices > Monitor > Manage Devices > Scripts and remediations**.
![Scripts and remediations option revealed in the Intune portal](./assets/intune-remediations.png)

Click on the "Create" button to create a new script package. Fill in the **Basics** form with name, description and other details and proceed to **Settings**.
During that step, upload the scripts in the correct boxes and use the following options:

- Run this script using the logged-on credentials (important because the script refers to a registry path under `HKCU`
  a.k.a `HKEY_CURRENT_USER`)
- Enforce script signature check: No (unless otherwise required by your company's policies)
- Run script in 64-bit PowerShell: No

Finally, select "Next" and assign "Scope tags" (if relevant) and in the "Assignments" select
the device or user groups that are required.

Follow [this guide](https://learn.microsoft.com/en-us/mem/intune/fundamentals/remediations#deploy-the-script-packages)
if you need more detail on the steps outlined above.

When users covered by the assignment next log in, Intune executes the detection script and then runs the remediation
script if the device is found to be non-compliant.

## Remarks

Careful readers may notice that there is an inherent race condition between setting the registry value and installing
the MSIX (if remotely deployed): when the MSIX is installed the registry sub-key that is referenced is recreated, overwriting any
previous value that the remote management solution would have deployed if that happened before the package installation.

An advantage of Intune Remediation scripts in this scenario is that eventually Intune finds the non-compliant
state and fixes it automatically. A potential disadvantage is that the fix doesn't start the UP4W background
agent, The fix enables the startup task and the agent will start at next user login.

