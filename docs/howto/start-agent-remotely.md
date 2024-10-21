# How to start the agent remotely

Ubuntu Pro for WSL, being a Microsoft Store application, cannot ship user services as of the time of writing (late
2024), but can deploy startup tasks instead, programs that run with user permissions when the user logs into the
Windows device. The UP4W background agent runs as a startup task, which is only enabled by the
operating system when the user interacts with the application for the first time. While this behaviour is a feature for
end-users it presents a source of friction for deployments at scale, when system administrators expect zero-touch
deployment of UP4W to just work.

This guide shows how system administrators can leverage Windows remote management solutions to start the agent once
with user credentials, thus interacting with the application in user's behalf. Subsequent logons will have the UP4W
background agent automatically as a consequence of such interaction.

While this guide uses Intune, readers are expected to translate the steps into any remote management solution of their
choice, as long as the said solution can run scripts with current user's credentials.

## Pre-requisites

- At least one managed Windows device.
- A Windows remote management solution.

## Key takeaways

1. Running a script as user to start the UP4W background agent makes it immediately available.
2. Remote management solutions can be used to do that.
3. The operating system considers the startup task enabled going forward.
4. Subsequent logons will have the operating system starting the UP4W background agent automatically as expected.

## Using Intune to run the UP4W background agent

The contents of the script can be far more elaborate, but for the purposes of this guide the following is enough:

```powershell
Write-Output "Starting the UP4W background agent remotely from Intune"
ubuntu-pro-agent.exe
```
**Make sure to save that as UTF-8**, as required by Intune.

Follow [this section from Intune documentation](https://learn.microsoft.com/en-us/mem/intune/apps/intune-management-extension#create-a-script-policy-and-assign-it)
if you need more detailed step-by-step guide on how to create and assign script policies.

Access your organisation's [Intune Admin Center](https://intune.microsoft.com) and when logged in go to **Devices > Monitor > Manage Devices > Scripts and remediations**.
On that page, click on the **Platform scripts** tab.
![Platform scripts revealed under Devices > Scripts and remediations](./assets/intune-platform-scripts.png)

Click on the "Add" button to create a new script policy and select the platform **Windows 10 and later**.

Fill in the **Basics** form with Name and description for the script being created.

In the **Settings** tab browse your machine to the PowerShell script to be deployed, and select the following options:
- Run this script using the logged on credentials: Yes (the default). UP4W must run with user credentials.
- Enforce script signature check: No (unless required otherwise by your company's policies)
- Run script in 64-bit PowerShell host: No (the default).

Apply the "Scope tags" according to your company's practices and, in the "Assignments" make sure to select one or more
groups encompassing the users which must receive and run the script.

You can then monitor for this script execution in the Intune Admin Center.

When the selected users log on their devices Intune will eventually start the UP4W background agent, a terminal window
will be visible with its regular outputs, which might be a user experience issue. But remember that it will only happen
once.

## Remarks

Careful readers might have noticed that if the script is deployed in conjunction with a policy installing the UP4W, it
would be theoretically possible to have the script running before the application gets installed. A more elaborate
solution is required if that scenario is possible, like looping in the script or using a more proactive solution such as
[Intune Remediations](https://learn.microsoft.com/en-us/mem/intune/fundamentals/remediations).

[This guide](howto::enforce-with-intune) shows how to use that tool to enforce the UP4W startup task state.

