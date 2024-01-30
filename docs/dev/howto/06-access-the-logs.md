# How to access the logs

At some point you may want to read the logs of Ubuntu Pro for WSL, most likely for debugging purposes. The agent and the service store their logs separately. This guide shows you where to find each of the logs.

## WSL Pro service

To access the logs of a specific distribution's WSL-Pro-Service, you must first launch the distribution and then query the journal:

```bash
journalctl -u wsl-pro.service
```

For more information on using the journal, you can check out its man page with `man journalctl` or [online](https://man7.org/linux/man-pages/man1/journalctl.1.html).

These logs may be insufficient for proper debugging, so you may be interested in looking at the agent's logs as well.

## Windows agent

Follow these steps in order to access the logs for the Windows agent.

1. On powershell, go to the package's directory:

   ```powershell
   Set-Location "$env:LocalAppData\Packages\CanonicalGroupLimited.UbuntuProForWSL_*"
   Set-Location "LocalCache\Local\Ubuntu Pro"
   ```

2. If any of these folders are missing, the Appx probably did not install. Otherwise, proceed with the next steps.
3. In the current folder, there should be various files. Be aware that modifying any of them could result in data loss.
4. Open file `log` with any text editor to see the logs. They are sorted with the oldest entries at the top and the newest at the bottom.
