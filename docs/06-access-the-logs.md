# How to access the logs

At some point you may want to read the logs of Ubuntu Pro for Windows, most likely for debugging purposes. The agent and the service store their logs separately. This guide shows you were to find each of the logs.

## WSL Pro service
In order to access the logs for a particular distro's WSL-Pro-Service, you need to start that distro and query the journal:
```bash
journalctl -u wsl-pro.service
```
For more information on using the journal, you can check out its man page with `man journalctl` or [online](https://man7.org/linux/man-pages/man1/journalctl.1.html).

These logs may be insufficient for proper debugging, so you may be interested in looking at the agent's logs as well.

## Windows agent
Follow these steps in order to access the logs for the Windows agent.
1. On powershell, go to the package's directory:
   ```powershell
   Set-Location "$env:LocalAppData\Packages\CanonicalGroupLimited.UbuntuProForWindows_*"
   Set-Location "LocalCache\Local\Ubuntu Pro"
   ```
2. If any of these folders do not exist, the Appx probably did not install. Otherwise, continue.
3. In the current folder, there should be various files. Modifying any of them could result in data loss.
4. Open file `log` with any text editor to see the logs. They are sorted from older at the top to newer at the bottom.