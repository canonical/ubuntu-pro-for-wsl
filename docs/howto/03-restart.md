---
myst:
  html_meta:
    "description lang=en":
      "For developers who are testing, debugging or developing the application."
---

# Restart Ubuntu Pro for WSL during development

```{include} ../includes/dev_docs_notice.txt
    :start-after: <!-- Include start dev -->
    :end-before: <!-- Include end dev -->
```

Some configuration changes only apply when you restart UP4W. Here is a guide on how to restart it. There are two options.

## Option 1: Restart your UP4W host machine

This is the simple one. If you're not in a hurry to see the configuration updated, just wait until next time you boot your machine.

## Option 2: Restart only UP4W

1. Stop the agent:

    ```powershell
    Get-Process -Name Ubuntu-Pro-Agent | Stop-Process
    ```

2. Stop the distro, or distros you installed WSL-Pro-Service in:

    ```powershell
    wsl --terminate DISTRO_NAME_1
    wsl --terminate DISTRO_NAME_2
    # etc.

    # Alternatively, stop all distros:
    wsl --shutdown
    ```

7. Start the agent again:
    1. Open the start Menu and search for "Ubuntu Pro for WSL".
    2. The GUI should start.
    3. Wait a minute.
    4. Click on "Click to restart it".
8. Start the distro, or distros you installed WSL-Pro-Service in.
